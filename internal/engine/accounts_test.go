package engine

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// TestPrimaryAccountComesFirstWhenFree is the rule the user asked for: every
// cell runs on the primary unless it's at its limit, goes back to it the
// moment the limit resets, and with every account out waits on the one that
// comes back first.
func TestPrimaryAccountComesFirstWhenFree(t *testing.T) {
	t.Setenv("HOME", "/home/someone")
	a := newAccounts([]string{"~/.claude", "/srv/claude-2/", " "})
	primary, second := "/home/someone/.claude", "/srv/claude-2"
	if len(a.folders) != 2 || a.current() != primary {
		t.Fatalf("folders %v, current %q: want the primary first, ~ expanded", a.folders, a.current())
	}

	a.exhaust(primary, time.Now().Add(time.Hour))
	if folder, free := a.pick(); folder != second || !free {
		t.Fatalf("primary out: got %q free=%v, want the second account", folder, free)
	}

	a.exhaust(second, time.Now().Add(30*time.Minute))
	if folder, free := a.pick(); folder != second || free {
		t.Fatalf("both out: got %q free=%v, want the one back first, not free", folder, free)
	}

	a.exhaust(primary, time.Now().Add(-time.Second))
	if a.current() != primary {
		t.Fatal("a primary whose limit reset is the one cells run on again")
	}
	if newAccounts(nil).current() != "" {
		t.Fatal("no accounts configured means no choice: the cell keeps Claude Code's own folder")
	}
}

// TestAccountNamesAreShort — the border has room for a word, not a path.
func TestAccountNamesAreShort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	for folder, want := range map[string]string{
		filepath.Join(home, ".claude"):        "primary",
		filepath.Join(home, ".claude-norven"): "norven",
		filepath.Join(home, ".claude-2"):      "2",
		"/srv/claude-work":                    "work",
		"/srv/elsewhere":                      "elsewhere",
	} {
		if got := accountName(folder); got != want {
			t.Errorf("%s shows as %q, want %q", folder, got, want)
		}
	}
}

// TestLimitsSurviveARestart — a restarted engine already knows which account
// is out, so the first cell it wakes goes to the free one; a limit that reset
// while it was down is forgotten.
func TestLimitsSurviveARestart(t *testing.T) {
	_, primary, second := twoAccounts(t)
	config := testConfig()
	config.ClaudeAccounts = []string{"~/.claude", "~/.claude-norven"}
	stateDir := t.TempDir()

	first := New(stateDir, config)
	if err := first.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	first.accounts.exhaust(primary, time.Now().Add(time.Hour))
	first.accounts.exhaust(second, time.Now().Add(-time.Minute))
	first.mu.Lock()
	first.save()
	first.mu.Unlock()
	first.Shutdown()

	saved, err := Load(StatePath(stateDir))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, kept := saved.Limits[second]; kept || saved.Limits[primary].IsZero() {
		t.Fatalf("only the limit still running is saved: %v", saved.Limits)
	}

	again := New(stateDir, config)
	if err := again.Start(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	t.Cleanup(again.Shutdown)
	if folder, free := again.accounts.pick(); folder != second || !free {
		t.Fatalf("after the restart a cell should go to %s, got %q free=%v", second, folder, free)
	}

	gone := newAccounts([]string{primary})
	gone.restore(map[string]time.Time{primary: time.Now().Add(-time.Second)})
	if len(gone.limits()) != 0 || gone.current() != primary {
		t.Fatal("a limit that reset while the engine was down is forgotten")
	}
}

// TestLimitMovesEveryRestingCell is the invariant: once an account is out, no
// cell at rest stays on it while another account is free. The cell whose turn
// was cut moves with the prompt to pick up; the ones at rest move with
// nothing sent; the rest wait for their turn to end.
func TestLimitMovesEveryRestingCell(t *testing.T) {
	cells := map[string]*fakeClaude{
		"cut":         {state: cell.Working},
		"resting":     {state: cell.Idle, idle: 2 * time.Minute},
		"stopped":     {state: cell.Interrupted, idle: 2 * time.Minute},
		"typing":      {state: cell.Idle, idle: 10 * time.Second},
		"unread":      {state: cell.Replied, idle: time.Hour},
		"busy":        {state: cell.Working, idle: time.Hour},
		"asking":      {state: cell.Approve, idle: time.Hour},
		"background":  {state: cell.Background, idle: time.Hour},
		"on the free": {state: cell.Idle, idle: time.Hour},
	}
	e, primary, second := accountsEngine(t, cells)
	cells["on the free"].account = second

	resets := time.Now().Add(3 * time.Hour).Truncate(time.Second)
	if err := e.Report(protocol.Report{Cell: "cut", Kind: "limit", ResetsAt: resets.Format(time.RFC3339)}); err != nil {
		t.Fatalf("limit: %v", err)
	}
	if until, out := e.accounts.outUntil(primary); !out || !until.Equal(resets) {
		t.Fatalf("the primary should be out until %v, got %v out=%v", resets, until, out)
	}
	if !waitFor(t, 2*time.Second, func() bool {
		return cells["cut"].moved() != nil && cells["resting"].moved() != nil && cells["stopped"].moved() != nil
	}) {
		t.Fatal("the cut cell and the ones at rest should have moved")
	}
	if got := *cells["cut"].moved(); got != (move{failoverPrompt, second}) {
		t.Fatalf("the cut cell picks up where it stopped, on %s: %+v", second, got)
	}
	for _, name := range []string{"resting", "stopped"} {
		if got := *cells[name].moved(); got != (move{"", second}) {
			t.Errorf("%s moves with nothing sent to it: %+v", name, got)
		}
	}
	for _, name := range []string{"typing", "unread", "busy", "asking", "background", "on the free"} {
		if got := cells[name].moved(); got != nil {
			t.Errorf("%s should have stayed where it is, moved %+v", name, *got)
		}
	}

	// The cell that was being typed in moves once it's been still for a
	// minute — unless every account is out by then.
	e.accounts.exhaust(second, time.Now().Add(time.Hour))
	cells["typing"].set(cell.Idle, 2*time.Minute)
	e.rebalance()
	if got := cells["typing"].moved(); got != nil {
		t.Fatalf("with every account out nobody moves, moved %+v", *got)
	}
	e.accounts.exhaust(second, time.Now().Add(-time.Second))
	e.rebalance()
	if got := cells["typing"].moved(); got == nil || *got != (move{"", second}) {
		t.Fatalf("the cell still for a minute moves to the free account, got %+v", got)
	}
}

// TestMoveByHand — the picker moves a conversation to the account named,
// with nothing sent; an account at its limit is refused, and so is a cell
// with no Claude up.
func TestMoveByHand(t *testing.T) {
	cells := map[string]*fakeClaude{
		"auth": {state: cell.Working},
		"dead": {state: cell.Crashed},
	}
	e, primary, second := accountsEngine(t, cells)

	e.accounts.exhaust(second, time.Now().Add(time.Hour))
	if err := e.MoveAccount("auth", "norven"); err == nil || !strings.Contains(err.Error(), "norven is at its limit until") {
		t.Fatalf("an account at its limit is refused with when it's back, got %v", err)
	}
	if err := e.MoveAccount("auth", "primary"); err != nil || cells["auth"].moved() != nil {
		t.Fatalf("the account it's already on is nothing to do, got %v", err)
	}
	if err := e.MoveAccount("auth", "nowhere"); err == nil {
		t.Fatal("an account that isn't configured is refused")
	}
	if err := e.MoveAccount("dead", "primary"); err == nil || !strings.Contains(err.Error(), "no Claude running") {
		t.Fatalf("a cell with no Claude up has nothing to move, got %v", err)
	}

	e.accounts.exhaust(second, time.Now().Add(-time.Second))
	if err := e.MoveAccount("auth", "norven"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if !waitFor(t, 2*time.Second, func() bool { return cells["auth"].moved() != nil }) {
		t.Fatal("the conversation should have moved")
	}
	if got := *cells["auth"].moved(); got != (move{"", second}) {
		t.Fatalf("a move by hand sends nothing and lands where asked: %+v", got)
	}
	if e.accounts.current() != primary {
		t.Fatal("a move by hand pins nothing: the next cell born still picks the primary")
	}

	one := New(t.TempDir(), testConfig())
	one.projects = []*project{{id: "p", path: t.TempDir(), cells: []*liveCell{{id: "auth", name: "auth", proc: &fakeClaude{state: cell.Idle}}}}}
	if err := one.MoveAccount("auth", "primary"); err == nil || !strings.Contains(err.Error(), "Only one Claude account") {
		t.Fatalf("with a single account there's nowhere to move, got %v", err)
	}
}

// TestSnapshotNamesEachCellsAccount — every Claude cell says which account it
// runs on, the status line gets each account's own usage, and a report
// counts for the account of the cell that sent it.
func TestSnapshotNamesEachCellsAccount(t *testing.T) {
	forgetReportedQuota(t)
	forgetQuota(t)
	cells := map[string]*fakeClaude{"auth": {state: cell.Idle}, "web": {state: cell.Idle}}
	e, primary, second := accountsEngine(t, cells)
	cells["web"].account = second

	if err := e.Report(protocol.Report{Cell: "web", Kind: "quota", Percent: 34}); err != nil {
		t.Fatalf("quota: %v", err)
	}
	e.accounts.exhaust(primary, time.Now().Add(time.Hour))

	snapshot := e.Snapshot()
	if snapshot.Quota != nil || len(snapshot.Accounts) != 2 {
		t.Fatalf("two accounts take the single badge's place: quota %+v, accounts %+v", snapshot.Quota, snapshot.Accounts)
	}
	first, other := snapshot.Accounts[0], snapshot.Accounts[1]
	if first.Name != "primary" || first.Quota == nil || first.Quota.Percent != 100 || first.Until == "" {
		t.Errorf("the account out reads 100%% and says when it's back: %+v", first)
	}
	if other.Name != "norven" || other.Quota == nil || other.Quota.Percent != 34 || other.Until != "" {
		t.Errorf("the free account carries its own usage: %+v", other)
	}
	byID := map[string]protocol.Cell{}
	for _, c := range snapshot.Projects[0].Cells {
		byID[c.ID] = c
	}
	if got := byID["auth"]; got.Account != "primary" || !got.AccountOut {
		t.Errorf("the cell on the account out says so: %+v", got)
	}
	if got := byID["web"]; got.Account != "norven" || got.AccountOut {
		t.Errorf("the cell on the free account names it: %+v", got)
	}
}

// twoAccounts gives the test a home of its own with Claude Code's folder and
// a second account next to it.
func twoAccounts(t *testing.T) (home, primary, second string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	return home, filepath.Join(home, ".claude"), filepath.Join(home, ".claude-norven")
}

// accountsEngine is an engine with two accounts and the fake cells in one
// project, every one of them on the primary.
func accountsEngine(t *testing.T, cells map[string]*fakeClaude) (e *Engine, primary, second string) {
	t.Helper()
	_, primary, second = twoAccounts(t)
	config := testConfig()
	config.ClaudeAccounts = []string{"~/.claude", "~/.claude-norven"}
	e = New(t.TempDir(), config)
	p := &project{id: "p", path: t.TempDir()}
	for name, fake := range cells {
		fake.account = primary
		p.cells = append(p.cells, &liveCell{id: name, name: name, kind: "claude", proc: fake})
	}
	e.projects = []*project{p}
	return e, primary, second
}

// move is what a fake cell was asked when its Claude came back up.
type move struct{ prompt, account string }

// fakeClaude is a Claude cell with no process: its state, how long it's been
// still and its account are whatever the test says, and it remembers the
// move it was asked for.
type fakeClaude struct {
	mu      sync.Mutex
	state   cell.State
	idle    time.Duration
	account string
	last    *move
}

func (f *fakeClaude) set(state cell.State, idle time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state, f.idle = state, idle
}

func (f *fakeClaude) moved() *move {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.last
}

func (f *fakeClaude) RespawnTab(_, prompt, account string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.last, f.account = &move{prompt, account}, account
	return nil
}

func (f *fakeClaude) Account(string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.account
}

func (f *fakeClaude) Idle() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.idle
}

func (f *fakeClaude) State() cell.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

func (f *fakeClaude) Spawn(cell.Config) error  { return nil }
func (f *fakeClaude) Draw() cell.Frame         { return cell.Frame{Live: true} }
func (f *fakeClaude) Key(cell.Keystroke) error { return nil }
func (f *fakeClaude) States() []cell.State     { return nil }
func (f *fakeClaude) Resize(int, int) error    { return nil }
func (f *fakeClaude) Scroll(int, bool)         {}
func (f *fakeClaude) Kill() error              { return nil }
