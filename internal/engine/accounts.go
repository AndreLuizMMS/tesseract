package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// limitFallback is how long an account sits out when its limit came without
// a reset time: the length of the 5-hour window.
const limitFallback = 5 * time.Hour

// failoverPrompt is the first thing a conversation hears on the account it
// moved to. The turn that hit the limit never finished, and the agent comes
// back up at its prompt waiting for someone to say so.
const failoverPrompt = "The previous Claude account hit its usage limit and this conversation moved to another account. Pick up where you left off."

// accounts are the Claude accounts on this machine, primary first, and until
// when each one is out for having hit its usage limit.
//
// ponytail: the limits live in memory. An engine restarted while the primary
// is out sends the next cell to it once more, and that cell's limit moves it
// on — one failed turn, not a lost conversation.
type accounts struct {
	mu      sync.Mutex
	folders []string
	out     map[string]time.Time
}

func newAccounts(folders []string) *accounts {
	return &accounts{folders: ExpandAccounts(folders), out: map[string]time.Time{}}
}

// ExpandAccounts turns the folders the config lists into absolute paths:
// "~/" is the home, blanks are dropped.
func ExpandAccounts(folders []string) []string {
	home, _ := os.UserHomeDir()
	expanded := make([]string, 0, len(folders))
	for _, folder := range folders {
		folder = strings.TrimSpace(folder)
		if rest, ok := strings.CutPrefix(folder, "~/"); ok && home != "" {
			folder = filepath.Join(home, rest)
		}
		if folder != "" {
			expanded = append(expanded, filepath.Clean(folder))
		}
	}
	return expanded
}

// list is the accounts, primary first.
func (a *accounts) list() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.folders...)
}

// set swaps the accounts for the ones the config lists now. An account that
// stays keeps its limit.
func (a *accounts) set(folders []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.folders = ExpandAccounts(folders)
}

// ReloadAccounts reads the Claude accounts in the config again. Cells born
// from now on choose among them; the ones running stay where they are.
func (e *Engine) ReloadAccounts() (string, error) {
	config, err := LoadConfig(ConfigPath())
	if err != nil {
		return "", err
	}
	e.accounts.set(config.ClaudeAccounts)
	folders := e.accounts.list()
	if len(folders) < 2 {
		return "one Claude account: no failover", nil
	}
	return fmt.Sprintf("%d Claude accounts, primary %s", len(folders), filepath.Base(folders[0])), nil
}

// current is the account a cell born now runs on (see pick).
func (a *accounts) current() string {
	folder, _ := a.pick()
	return folder
}

// pick is the first account not at its limit, which is the primary whenever
// it can be, and free says one was found. With every account out, it's the
// one that comes back first. Empty when there are no accounts.
func (a *accounts) pick() (folder string, free bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	soonestAt := time.Time{}
	for _, candidate := range a.folders {
		until := a.out[candidate]
		if !until.After(now) {
			return candidate, true
		}
		if folder == "" || until.Before(soonestAt) {
			folder, soonestAt = candidate, until
		}
	}
	return folder, false
}

// exhaust takes an account out until its limit resets.
func (a *accounts) exhaust(folder string, until time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.out[folder] = until
}

// reportLimit takes the cell's account out and moves its conversation to the
// account that's still free. With every account out the conversation stays
// where it is: Claude Code is already saying so on the cell's screen.
func (e *Engine) reportLimit(c *liveCell, report protocol.Report) error {
	if len(e.accounts.list()) < 2 {
		return nil
	}
	withAccount, ok := c.proc.(cell.WithAccount)
	if !ok {
		return fmt.Errorf("cell %s has no account", c.name)
	}
	account := withAccount.Account(report.Tab)
	if account == "" {
		return nil
	}
	until := time.Now().Add(limitFallback)
	// A reset time that doesn't parse costs the account a longer wait, never
	// the move: dropping the report would leave the conversation stuck.
	if parsed, err := time.Parse(time.RFC3339, report.ResetsAt); err == nil && parsed.After(time.Now()) {
		until = parsed
	}
	e.accounts.exhaust(account, until)

	next, free := e.accounts.pick()
	if !free || next == account {
		go e.announce(c.name + ": every Claude account is at its usage limit")
		return nil
	}
	// Off the report: the process that's reporting is the one going down.
	go e.failover(c.id, report.Tab, filepath.Base(next))
	return nil
}

// failover brings the cell's Claude back up on the account that's free now,
// on the same conversation. A cell with tabs restarts that tab alone; the
// shell and the other agents next to it don't notice.
func (e *Engine) failover(cellID, tab, account string) {
	e.mu.Lock()
	var name string
	var err error
	for _, p := range e.projects {
		for _, c := range p.cells {
			if c.id != cellID || c.proc == nil {
				continue
			}
			name = c.name
			if respawn, ok := c.proc.(cell.WithRespawn); ok {
				err = respawn.RespawnTab(tab, failoverPrompt)
			} else {
				_ = c.proc.Kill()
				if c.record != nil {
					_ = c.record.Close()
				}
				err = e.wake(p, c, failoverPrompt)
			}
		}
	}
	e.markDirty()
	e.mu.Unlock()

	switch {
	case name == "":
	case err != nil:
		e.announce(name + ": could not move to " + account + ": " + err.Error())
	default:
		e.announce(name + ": usage limit hit, conversation moved to " + account)
	}
}
