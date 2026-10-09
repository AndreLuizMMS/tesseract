package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// TestStillGridProducesNoFrame pins down the engine's silence: with the
// watcher looking and no cell producing anything, the version doesn't climb
// — and with no new version, no screen gets a snapshot to redraw.
//
// The watcher is stepped by hand, tick by tick, instead of running on its
// own clock: its slow heartbeat is a frame on purpose, and whether it fell
// inside the window being measured used to depend on how long the shell took
// to come up.
func TestStillGridProducesNoFrame(t *testing.T) {
	m := testEngine(t)
	if _, err := m.Create(protocol.Create{Path: t.TempDir(), Type: "bash"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// First the shell has to speak. A shell that reads a long startup file
	// stays silent for a good while before its first prompt, and silence
	// before the first word isn't a still grid: it's a grid about to move.
	born := m.Version()
	for deadline := time.Now().Add(20 * time.Second); m.Version() == born; {
		if time.Now().After(deadline) {
			t.Fatal("the shell never wrote its prompt")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Then it has to finish: while it's talking, climbing the version is
	// correct. Three quiet windows in a row is enough to know it stopped, not
	// just paused mid-output.
	quiet := 0
	for range 100 {
		before := m.Version()
		time.Sleep(200 * time.Millisecond)
		if before == m.Version() {
			quiet++
		} else {
			quiet = 0
		}
		if quiet == 3 {
			break
		}
	}
	if quiet < 3 {
		t.Fatal("the shell never settled down")
	}

	// Every tick short of the heartbeat, over a grid where nothing moves.
	before := m.Version()
	for tick := 1; tick < ticksBetweenRefresh; tick++ {
		m.watchTick(tick)
	}
	time.Sleep(time.Second)
	if after := m.Version(); after != before {
		t.Fatalf("still grid generated %d frame(s) for nothing", after-before)
	}

	// And the heartbeat is the one frame a still grid does get.
	m.watchTick(ticksBetweenRefresh)
	if m.Version() != before+1 {
		t.Fatal("the heartbeat should redraw a still grid exactly once")
	}
}

// TestVanishedDirectoryOrphansTheCells — a project whose directory left the
// disk shows every cell as orphaned, and keeps showing it: the watcher's
// next look at the processes, which are still alive in there, used to write
// "working" back over the mark. The directory coming back clears it.
func TestVanishedDirectoryOrphansTheCells(t *testing.T) {
	m := testEngine(t)
	dir := filepath.Join(t.TempDir(), "api")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(protocol.Create{Path: dir, Type: "bash"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	state := func() string { return m.Snapshot().Projects[0].Cells[0].State }

	m.checkDirectories()
	if got := state(); got == string(cell.Orphaned) {
		t.Fatalf("a project that's on disk isn't orphaned: %q", got)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if !m.checkDirectories() {
		t.Fatal("losing the directory is a change worth a frame")
	}
	// The ticks between two disk checks only look at the processes.
	for range 3 {
		m.checkStates()
		if got := state(); got != string(cell.Orphaned) {
			t.Fatalf("the cell should stay orphaned while the directory is gone, got %q", got)
		}
	}
	// `ts status` is read with no legend at hand: the glyph and its word,
	// and the kind only when the name doesn't already say it.
	if !strings.Contains(m.Summary(), "  ⚠ orphaned     bash 1\n") {
		t.Fatalf("the status summary should say orphaned too, glyph and word: %q", m.Summary())
	}

	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if !m.checkDirectories() {
		t.Fatal("the directory coming back is a change worth a frame")
	}
	if got := state(); got == string(cell.Orphaned) {
		t.Fatal("the directory is back and the cell is still orphaned")
	}
}

// tabbedCell is a cell with two tabs whose state is the tab in view's: the
// agent's tab holds an answer nobody read, the other one is at rest.
type tabbedCell struct {
	cell.Cell
	tab    string
	states map[string]cell.State
}

func (c *tabbedCell) State() cell.State   { return c.states[c.tab] }
func (c *tabbedCell) Tabs() []string      { return []string{"claude", "md"} }
func (c *tabbedCell) ActiveTab() string   { return c.tab }
func (c *tabbedCell) SwitchTab(int) error { return nil }

// TestTurnEndIsAnnouncedOnce — the answer stays unread on the agent's tab
// while the user reads a markdown on another. Coming back to it shows the
// same answer again, and that isn't news: one turn's end, one notice.
func TestTurnEndIsAnnouncedOnce(t *testing.T) {
	log := filepath.Join(t.TempDir(), "notices.txt")
	notifier := filepath.Join(t.TempDir(), "notify")
	script := "#!/bin/sh\nshift\necho \"$1\" >> " + log + "\n"
	if err := os.WriteFile(notifier, []byte(script), 0o755); err != nil {
		t.Fatalf("prepare notifier: %v", err)
	}
	e := New(t.TempDir(), Config{Notify: true, NotifyCommand: notifier})
	agent := &tabbedCell{tab: "claude", states: map[string]cell.State{"claude": cell.Working, "md": cell.Idle}}
	watched := &liveCell{id: "c1", name: "session 1", proc: agent}
	e.projects = []*project{{path: "/work/beta", cells: []*liveCell{watched}}}
	notices := func() string {
		time.Sleep(300 * time.Millisecond) // the notifier is released, not waited for
		content, _ := os.ReadFile(log)
		return string(content)
	}

	e.checkStates()
	agent.states["claude"] = cell.Replied
	watched.detail, watched.detailTab = "done", "claude"
	e.checkStates()
	if got := notices(); got != "session 1 · beta responded: done\n" {
		t.Fatalf("the end of the turn should be announced with what was said, got %q", got)
	}

	agent.tab = "md"
	e.checkStates()
	agent.tab = "claude"
	if !e.checkStates() {
		t.Fatal("the tab coming back still has to redraw the grid")
	}
	if got := notices(); got != "session 1 · beta responded: done\n" {
		t.Fatalf("coming back to the tab announced the same answer again: %q", got)
	}

	// A new turn on the tab in view is news again.
	agent.states["claude"] = cell.Working
	e.checkStates()
	agent.states["claude"] = cell.Replied
	e.checkStates()
	if got := notices(); strings.Count(got, "responded") != 2 {
		t.Fatalf("the next turn's end should be announced, got %q", got)
	}
}
