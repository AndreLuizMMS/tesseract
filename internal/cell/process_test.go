package cell

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"

	"github.com/andreluiz/tesseract/internal/engine/history"
)

// waitFor tries the condition until it succeeds or the deadline runs out.
func waitFor(t *testing.T, deadline time.Duration, condition func() bool) bool {
	t.Helper()
	limit := time.Now().Add(deadline)
	for time.Now().Before(limit) {
		if condition() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return condition()
}

func screenOf(c Cell) string {
	return strings.Join(c.Draw().Lines, "\n")
}

// pasteAndEnter does what the user does: pastes the command and then sends
// enter. The paste goes in marked as a paste, so the line break doesn't come
// along with the pasted text.
func pasteAndEnter(t *testing.T, c Cell, command string) {
	t.Helper()
	if err := c.Key(Keystroke{Paste: command}); err != nil {
		t.Fatalf("paste: %v", err)
	}
	if err := c.Key(Keystroke{Code: vt.KeyEnter}); err != nil {
		t.Fatalf("enter: %v", err)
	}
}

// TestBashShowsWhatWasTyped is the vertical slice: brings up a real shell,
// writes to it, and the engine's internal screen ends up containing the
// output.
func TestBashShowsWhatWasTyped(t *testing.T) {
	dir := t.TempDir()
	hist, err := history.Open(filepath.Join(dir, "hist.log"), history.DefaultCap)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	defer hist.Close()

	c, err := New("bash")
	if err != nil {
		t.Fatalf("manufacture cell: %v", err)
	}
	if err := c.Spawn(Config{
		ID: "c1", Directory: dir, Name: "tests",
		History: hist, Columns: 60, Lines: 12,
	}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer c.Kill()

	if c.State() != Working {
		t.Fatalf("a live cell should be working, it's %q", c.State())
	}

	pasteAndEnter(t, c, "echo tesseract")
	if !waitFor(t, 2*time.Second, func() bool {
		return strings.Contains(screenOf(c), "tesseract")
	}) {
		t.Fatalf("the output didn't show up on the internal screen in 2s:\n%s", screenOf(c))
	}

	// What the cell produced also went to the history on disk.
	if !waitFor(t, 2*time.Second, func() bool {
		found, _ := hist.Search("tesseract")
		return len(found) > 0
	}) {
		t.Fatal("the output wasn't recorded in the history")
	}
}

// TestKilledCellBecomesStopped guarantees killing isn't the same as crashing.
func TestKilledCellBecomesStopped(t *testing.T) {
	dir := t.TempDir()
	c, err := New("bash")
	if err != nil {
		t.Fatalf("manufacture cell: %v", err)
	}
	if err := c.Spawn(Config{ID: "c1", Directory: dir, Columns: 40, Lines: 10}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if err := c.Kill(); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if !waitFor(t, 2*time.Second, func() bool { return c.State() == Stopped }) {
		t.Fatalf("state after killing: %q, expected stopped", c.State())
	}
}

// TestProcessThatDiesOnItsOwnCrashes is the other end: nobody asked, so it
// crashed.
func TestProcessThatDiesOnItsOwnCrashes(t *testing.T) {
	dir := t.TempDir()
	c, err := New("bash")
	if err != nil {
		t.Fatalf("manufacture cell: %v", err)
	}
	if err := c.Spawn(Config{ID: "c1", Directory: dir, Columns: 40, Lines: 10}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer c.Kill()

	pasteAndEnter(t, c, "exit")
	if !waitFor(t, 3*time.Second, func() bool { return c.State() == Crashed }) {
		t.Fatalf("state after the shell exited: %q, expected crashed", c.State())
	}
}

// TestSpawnInNonexistentDirectoryFails protects the engine from an invalid
// state.
func TestSpawnInNonexistentDirectoryFails(t *testing.T) {
	c, err := New("bash")
	if err != nil {
		t.Fatalf("manufacture cell: %v", err)
	}
	err = c.Spawn(Config{ID: "c1", Directory: filepath.Join(t.TempDir(), "does-not-exist"), Columns: 40, Lines: 10})
	if err == nil {
		t.Fatal("spawning in a nonexistent directory should fail")
	}
	if !strings.Contains(err.Error(), "doesn't exist") {
		t.Fatalf("unclear error message: %v", err)
	}
}

// TestUnknownKind guarantees the registry is the only entry point.
func TestUnknownKind(t *testing.T) {
	if _, err := New("planilha"); err == nil {
		t.Fatal("a kind outside the registry should fail")
	}
}

// TestKeystrokeByKeystrokeReachesTheShell covers the TYPE mode path: a key
// that prints, a key with shift, and a special key.
func TestKeystrokeByKeystrokeReachesTheShell(t *testing.T) {
	dir := t.TempDir()
	c, err := New("bash")
	if err != nil {
		t.Fatalf("manufacture cell: %v", err)
	}
	if err := c.Spawn(Config{ID: "c1", Directory: dir, Columns: 60, Lines: 12}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer c.Kill()
	time.Sleep(300 * time.Millisecond) // let the shell draw the prompt

	for _, letter := range "echo Tesseract" {
		tap := Keystroke{Code: letter, Text: string(letter)}
		if letter >= 'A' && letter <= 'Z' {
			tap.Code = letter + 32
			tap.Mod = int(vt.ModShift)
		}
		if err := c.Key(tap); err != nil {
			t.Fatalf("key %q: %v", letter, err)
		}
	}
	if err := c.Key(Keystroke{Code: vt.KeyEnter}); err != nil {
		t.Fatalf("enter: %v", err)
	}

	if !waitFor(t, 3*time.Second, func() bool {
		return strings.Count(screenOf(c), "Tesseract") >= 2 // the command echo and the output
	}) {
		t.Fatalf("the uppercase letter or the enter got lost along the way:\n%s", screenOf(c))
	}
}

// TestScrollShowsThePast covers the mouse wheel: goes up through history and
// back to live.
func TestScrollShowsThePast(t *testing.T) {
	dir := t.TempDir()
	c, err := New("bash")
	if err != nil {
		t.Fatalf("manufacture cell: %v", err)
	}
	if err := c.Spawn(Config{ID: "c1", Directory: dir, Columns: 40, Lines: 8}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer c.Kill()

	pasteAndEnter(t, c, "for i in $(seq 1 40); do echo linha-$i; done")
	if !waitFor(t, 3*time.Second, func() bool {
		return strings.Contains(screenOf(c), "linha-40")
	}) {
		t.Fatalf("the output didn't finish coming out:\n%s", screenOf(c))
	}
	if strings.Contains(screenOf(c), "linha-1\n") {
		t.Fatal("with 8 lines on screen, the start should already have scrolled off")
	}

	c.Scroll(30, false)
	frame := c.Draw()
	if frame.Live {
		t.Fatal("after scrolling, the reading isn't live anymore")
	}
	if frame.Scroll != 30 {
		t.Fatalf("scroll came back %d, expected 30", frame.Scroll)
	}
	if !strings.Contains(strings.Join(frame.Lines, "\n"), "linha-1") {
		t.Fatalf("the past didn't show up when scrolling:\n%s", strings.Join(frame.Lines, "\n"))
	}

	c.Scroll(0, true)
	frame = c.Draw()
	if !frame.Live || frame.Scroll != 0 {
		t.Fatalf("esc should go back to live: %#v", frame)
	}
	if !strings.Contains(strings.Join(frame.Lines, "\n"), "linha-40") {
		t.Fatal("back to live, the end had to be on screen")
	}
}

// TestScrollPreservesStyle is the reading rule: scrolling shows the past the
// way it appeared, with color and bold, not as plain text.
func TestScrollPreservesStyle(t *testing.T) {
	dir := t.TempDir()
	c, err := New("bash")
	if err != nil {
		t.Fatalf("manufacture cell: %v", err)
	}
	if err := c.Spawn(Config{ID: "c1", Directory: dir, Columns: 40, Lines: 6}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer c.Kill()

	// Each line comes out green and bold, and then scrolls into history.
	pasteAndEnter(t, c, "for i in $(seq 1 30); do printf '\\033[1;32mverde-%s\\033[0m\\n' $i; done")
	if !waitFor(t, 3*time.Second, func() bool {
		return strings.Contains(screenOf(c), "verde-30")
	}) {
		t.Fatalf("the output didn't finish coming out:\n%s", screenOf(c))
	}

	c.Scroll(25, false)
	scrolled := strings.Join(c.Draw().Lines, "\n")
	if !strings.Contains(scrolled, "verde-2") {
		t.Fatalf("the past didn't show up when scrolling:\n%q", scrolled)
	}
	if !strings.Contains(scrolled, "\x1b[") {
		t.Fatalf("scrolling delivered text with no style at all:\n%q", scrolled)
	}
	if !strings.Contains(scrolled, "32") {
		t.Fatalf("the green color got lost while scrolling:\n%q", scrolled)
	}
}

// TestSpawnWithHistoryThatAsksTheTerminal is the regression for a real hang:
// replaying the history brings along questions the old program asked the
// terminal, and the internal screen answers them. Without something
// listening for those answers, the cell used to spawn stuck and the engine
// never opened the socket.
func TestSpawnWithHistoryThatAsksTheTerminal(t *testing.T) {
	dir := t.TempDir()
	hist, err := history.Open(filepath.Join(dir, "hist.log"), history.DefaultCap)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	defer hist.Close()

	// Plenty of questions: device attributes, cursor position and modes.
	questions := strings.Repeat("\x1b[c\x1b[6n\x1b[>0c\x1b[?1049$p", 200)
	if _, err := hist.Write([]byte("old output\n" + questions)); err != nil {
		t.Fatalf("write history: %v", err)
	}

	spawned := make(chan error, 1)
	go func() {
		c, err := New("bash")
		if err != nil {
			spawned <- err
			return
		}
		err = c.Spawn(Config{ID: "c1", Directory: dir, History: hist, Columns: 60, Lines: 12})
		if err == nil {
			t.Cleanup(func() { c.Kill() })
		}
		spawned <- err
	}()

	select {
	case err := <-spawned:
		if err != nil {
			t.Fatalf("spawn: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the cell got stuck replaying a history with questions to the terminal")
	}
}

// TestResizingToTheSameSizeDoesNotClearTheScreen is the regression for a tab
// that used to show up blank: the internal screen was cleared by a resize
// that changed nothing, and the program in there had no reason to redraw.
func TestResizingToTheSameSizeDoesNotClearTheScreen(t *testing.T) {
	dir := t.TempDir()
	c, err := New("bash")
	if err != nil {
		t.Fatalf("manufacture cell: %v", err)
	}
	if err := c.Spawn(Config{ID: "c1", Directory: dir, Columns: 60, Lines: 12}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer c.Kill()

	pasteAndEnter(t, c, "echo mark-on-screen")
	if !waitFor(t, 3*time.Second, func() bool {
		return strings.Contains(screenOf(c), "mark-on-screen")
	}) {
		t.Fatalf("the output didn't show up:\n%s", screenOf(c))
	}

	if err := c.Resize(60, 12); err != nil {
		t.Fatalf("resize: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if !strings.Contains(screenOf(c), "mark-on-screen") {
		t.Fatalf("the same size shouldn't have cleared the screen:\n%s", screenOf(c))
	}
}

// TestStillScreenReturnsTheSameFrame pins down the stored portrait: drawing
// a cell that received nothing returns the exact same frame, and what
// arrived from the process afterward shows up all the same.
func TestStillScreenReturnsTheSameFrame(t *testing.T) {
	dir := t.TempDir()
	hist, err := history.Open(filepath.Join(dir, "hist.log"), history.DefaultCap)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	defer hist.Close()

	c, err := New("bash")
	if err != nil {
		t.Fatalf("manufacture cell: %v", err)
	}
	if err := c.Spawn(Config{
		ID: "c1", Directory: dir, Name: "tests",
		History: hist, Columns: 60, Lines: 12,
	}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer c.Kill()

	pasteAndEnter(t, c, "echo primeiro")
	if !waitFor(t, 5*time.Second, func() bool { return strings.Contains(screenOf(c), "primeiro") }) {
		t.Fatalf("the shell didn't write to the screen: %s", screenOf(c))
	}

	// Nothing came in between one draw and the other: the two readings are
	// the same slice of lines, not two equal copies.
	before, after := c.Draw(), c.Draw()
	if len(before.Lines) == 0 || len(before.Lines) != len(after.Lines) {
		t.Fatalf("frames of different sizes: %d and %d", len(before.Lines), len(after.Lines))
	}
	if &before.Lines[0] != &after.Lines[0] {
		t.Fatal("a still screen should return the stored frame, not redraw")
	}

	// What arrives afterward has to show up: storing can't freeze the cell.
	pasteAndEnter(t, c, "echo segundo")
	if !waitFor(t, 5*time.Second, func() bool { return strings.Contains(screenOf(c), "segundo") }) {
		t.Fatalf("the stored frame froze the cell: %s", screenOf(c))
	}
}
