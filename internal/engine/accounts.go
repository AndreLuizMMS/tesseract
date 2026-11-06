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

// claudeTab is the tab a session's Claude runs in.
const claudeTab = "claude"

// moveIdleAfter is how still a cell has to be before it moves to another
// account on its own. A key or a byte of output in the last minute may be
// someone halfway through a prompt in the box, and the move would throw it
// away.
const moveIdleAfter = time.Minute

// accounts are the Claude accounts on this machine, primary first, and until
// when each one is out for having hit its usage limit. The limits are saved
// with the engine's state, so a restarted engine doesn't send a cell to an
// account it already knows is out.
type accounts struct {
	mu      sync.Mutex
	folders []string
	out     map[string]time.Time
	// warned is the comeback the "every account is out" notice last named:
	// two cells hitting the last limit at once say it once.
	warned time.Time
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

// accountName is how an account shows on screen: Claude Code's own folder is
// the primary, any other is its folder's name without the ".claude-" in
// front — ~/.claude-norven is norven.
func accountName(folder string) string {
	if folder == cell.NativeAccount() {
		return "primary"
	}
	base := filepath.Base(folder)
	if name := strings.TrimPrefix(strings.TrimPrefix(base, "."), "claude-"); name != "" && name != "claude" {
		return name
	}
	return base
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

// outUntil says whether the account is at its limit now, and until when.
func (a *accounts) outUntil(folder string) (time.Time, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	until := a.out[folder]
	return until, until.After(time.Now())
}

// limits are the accounts still out, for the saved state.
func (a *accounts) limits() map[string]time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	kept := map[string]time.Time{}
	for folder, until := range a.out {
		if until.After(now) {
			kept[folder] = until
		}
	}
	return kept
}

// restore takes back the limits a previous engine saved. One that already
// reset is dropped: the account is free again.
func (a *accounts) restore(saved map[string]time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for folder, until := range saved {
		if until.After(now) {
			a.out[folder] = until
		}
	}
}

// warnOnce says whether the "every account is out" notice still has to name
// this comeback.
func (a *accounts) warnOnce(until time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.warned.Equal(until) {
		return false
	}
	a.warned = until
	return true
}

// usage is every account the way the status line and the picker show it, in
// the config's order: its usage and, while it's out, 100% and when it's back.
func (a *accounts) usage() []protocol.Account {
	now := time.Now()
	var all []protocol.Account
	for _, folder := range a.list() {
		entry := protocol.Account{Name: accountName(folder), Quota: ReadQuota(folder)}
		if until, out := a.outUntil(folder); out {
			quota := protocol.Quota{Percent: 100, Rollover: timeRemaining(until.Sub(now))}
			if entry.Quota != nil {
				quota.Weekly = entry.Quota.Weekly
			}
			entry.Quota, entry.Until = &quota, clockTime(until, now)
		}
		all = append(all, entry)
	}
	return all
}

// clockTime writes when something happens as the clock on the wall says it,
// with the weekday when it isn't today.
func clockTime(at, now time.Time) string {
	if at.YearDay() != now.YearDay() || at.Year() != now.Year() {
		return at.Format("Mon 15:04")
	}
	return at.Format("15:04")
}

// claudeOf is the cell's Claude and the tab it runs in: the cell itself, or
// its claude tab while that tab is up. Nil when there is none.
func claudeOf(proc cell.Cell) (cell.Cell, string) {
	if withTab, ok := proc.(cell.WithTab); ok {
		return withTab.Tab(claudeTab), claudeTab
	}
	return proc, ""
}

// accountOf is the account the agent runs on while its process is up; empty
// for an agent that has none to choose from, and for one that's down.
func accountOf(agent cell.Cell) string {
	withAccount, ok := agent.(cell.WithAccount)
	if !ok {
		return ""
	}
	switch agent.State() {
	case cell.Crashed, cell.Stopped, cell.Orphaned:
		return ""
	}
	return withAccount.Account("")
}

// atRest says a cell can move to another account without anyone losing
// anything: at its prompt with nothing unread, and still for a minute. An
// answer nobody read stays where it is, glyph and summary on show, until
// someone reads it.
func atRest(agent cell.Cell) bool {
	if state := agent.State(); state != cell.Idle && state != cell.Interrupted {
		return false
	}
	idle, ok := agent.(cell.WithIdle)
	return ok && idle.Idle() >= moveIdleAfter
}

// reportLimit takes the cell's account out and moves its conversation to the
// account that's still free, with the prompt that tells it to pick up where
// it stopped. Then every other cell at rest on that account moves too. With
// every account out the conversation stays where it is: Claude Code is
// already saying so on the cell's screen.
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
	e.mu.Lock()
	e.save()
	e.mu.Unlock()

	next, free := e.accounts.pick()
	if !free || next == account {
		if back, _ := e.accounts.outUntil(next); e.accounts.warnOnce(back) {
			go e.announce("every Claude account is at its usage limit; " + accountName(next) + " is back at " + clockTime(back, time.Now()))
		}
		return nil
	}
	// A cell already on its way to another account isn't sent twice.
	e.mu.Lock()
	already := c.moving
	c.moving = true
	e.mu.Unlock()
	// Off the report: the process that's reporting is the one going down.
	if !already {
		go e.moveTo(c.id, report.Tab, next, failoverPrompt, accountName(account)+" at its limit, ")
	}
	go e.rebalance()
	return nil
}

// rebalance holds the accounts to one rule: no Claude cell at rest stays on
// an account at its limit while another account is free. Those cells move
// to the first free one with nothing sent to them; a cell working, waiting
// on you or with an answer unread moves once it comes to rest, or when its
// own turn hits the limit. A cell in the background, stopped or asleep picks
// its account when it comes back.
//
// ponytail: it runs on a limit and on the watcher's 60s sweep, so an account
// that resets is used again up to a minute late. A timer at the soonest reset
// is the upgrade if that minute matters.
func (e *Engine) rebalance() {
	next, free := e.accounts.pick()
	if len(e.accounts.list()) < 2 || !free {
		return
	}
	type move struct{ id, tab, from string }
	var moves []move
	e.mu.Lock()
	for _, p := range e.projects {
		for _, c := range p.cells {
			if c.moving || c.proc == nil {
				continue
			}
			agent, tab := claudeOf(c.proc)
			if agent == nil {
				continue
			}
			from := accountOf(agent)
			if _, out := e.accounts.outUntil(from); from == "" || !out || !atRest(agent) {
				continue
			}
			c.moving = true
			moves = append(moves, move{c.id, tab, from})
		}
	}
	e.mu.Unlock()
	for _, m := range moves {
		e.moveTo(m.id, m.tab, next, "", accountName(m.from)+" at its limit, ")
	}
}

// MoveAccount moves the cell's Claude conversation to the account the user
// picked by name, with nothing sent to it: it comes back up at its prompt,
// waiting. A turn in progress is cut — the user asked for it. The choice
// holds until the cell next comes up, which picks the first free account
// again.
func (e *Engine) MoveAccount(cellID, name string) error {
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", cellID)
	}
	folders := e.accounts.list()
	if len(folders) < 2 {
		return fmt.Errorf("Only one Claude account is set up. ts account add sets up another.")
	}
	folder := ""
	for _, candidate := range folders {
		if accountName(candidate) == name {
			folder = candidate
		}
	}
	if folder == "" {
		return fmt.Errorf("No Claude account named %s. a lists them.", name)
	}
	agent, tab := claudeOf(c.proc)
	from := ""
	if agent != nil {
		from = accountOf(agent)
	}
	switch {
	case from == "":
		return fmt.Errorf("%s has no Claude running. r brings it back up.", c.name)
	case from == folder:
		return nil
	case agent.State() == cell.Background:
		return fmt.Errorf("%s's conversation runs in the background. ↵ enters the cell, and Esc there brings it back.", c.name)
	}
	if until, out := e.accounts.outUntil(folder); out {
		return fmt.Errorf("%s is at its limit until %s. a picks another account.", name, clockTime(until, time.Now()))
	}
	e.mu.Lock()
	moving := c.moving
	c.moving = true
	e.mu.Unlock()
	if moving {
		return fmt.Errorf("%s is already moving to another account.", c.name)
	}
	go e.moveTo(c.id, tab, folder, "", "")
	return nil
}

// moveTo brings the cell's Claude back up on the account in folder, on the
// same conversation, with prompt as the first thing it hears. A cell with
// tabs restarts that tab alone; the shell and the other agents next to it
// don't notice. why opens the notice that says where it went.
func (e *Engine) moveTo(cellID, tab, folder, prompt, why string) {
	e.mu.Lock()
	var name string
	var err error
	for _, p := range e.projects {
		for _, c := range p.cells {
			if c.id != cellID {
				continue
			}
			c.moving = false
			// An engine shut down while the move waited brings nothing up.
			if c.proc == nil || e.down {
				continue
			}
			name = c.name
			if respawn, ok := c.proc.(cell.WithRespawn); ok {
				err = respawn.RespawnTab(tab, prompt, folder)
			} else {
				_ = c.proc.Kill()
				if c.record != nil {
					_ = c.record.Close()
				}
				err = e.wake(p, c, prompt, folder)
			}
		}
	}
	e.markDirty()
	e.mu.Unlock()

	switch {
	case name == "":
	case err != nil:
		e.announce(name + ": could not move to " + accountName(folder) + ": " + err.Error())
	default:
		e.announce(name + ": " + why + "moved to " + accountName(folder))
	}
}
