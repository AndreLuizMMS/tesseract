package screen

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// linesWithColor is what the cell actually sends: text already styled by
// the program inside it. Copying can't drag the color codes along.
var linesWithColor = []string{
	"\x1b[32mMoved the\x1b[0m check to guard",
	"for the check.",
	"Which do you prefer?",
}

// TestSelectionCopiesOnlyTheLetters — what goes to the clipboard is the
// text, with no color and no trailing spaces at the end of the line.
func TestSelectionCopiesOnlyTheLetters(t *testing.T) {
	s := Selection{Cell: "c1", AnchorX: 6, AnchorY: 0, CurrentX: 7, CurrentY: 2}

	text := s.Text(linesWithColor, 40)
	expected := "the check to guard\nfor the check.\nWhich do"
	if text != expected {
		t.Fatalf("the copied stretch came out %q, expected %q", text, expected)
	}
	if strings.Contains(text, "\x1b") {
		t.Errorf("the copied stretch carried a color code along: %q", text)
	}
}

// TestSelectionWorksBothWays — dragging bottom-to-top marks the same stretch
// as dragging top-to-bottom.
func TestSelectionWorksBothWays(t *testing.T) {
	down := Selection{AnchorX: 6, AnchorY: 0, CurrentX: 7, CurrentY: 2}
	up := Selection{AnchorX: 7, AnchorY: 2, CurrentX: 6, CurrentY: 0}
	if down.Text(linesWithColor, 40) != up.Text(linesWithColor, 40) {
		t.Fatalf("the drag direction changed the stretch:\n%q\n%q",
			down.Text(linesWithColor, 40), up.Text(linesWithColor, 40))
	}
}

// TestSelectionOnASingleLine — a single-line mark runs from the anchor to
// the point, including the letter under the cursor.
func TestSelectionOnASingleLine(t *testing.T) {
	s := Selection{AnchorX: 0, AnchorY: 1, CurrentX: 2, CurrentY: 1}
	if text := s.Text(linesWithColor, 40); text != "for" {
		t.Fatalf("the single-line stretch came out %q, expected %q", text, "for")
	}
}

// TestPaintFitsInsideTheContent — the mark lights up to the margin, like on
// a terminal, but never past the cell's width, or the box would warp.
func TestPaintFitsInsideTheContent(t *testing.T) {
	s := Selection{AnchorX: 6, AnchorY: 0, CurrentX: 7, CurrentY: 2}
	painted := s.Paint(linesWithColor, 40)

	for i, line := range painted {
		if width := lipgloss.Width(line); width > 40 {
			t.Errorf("line %d ended up with %d columns, and the content is 40", i, width)
		}
	}
	if !strings.Contains(painted[1], markOpening()+"for the check.") {
		t.Errorf("the middle line should be highlighted end to end: %q", painted[1])
	}
	if start := lipgloss.Width(ansiBeforeMark(painted[0])); start != 6 {
		t.Errorf("the first line's mark should start at column 6, started at %d", start)
	}
	if strings.Contains(painted[2], markOpening()+"Which do you prefer?") {
		t.Error("the last line should stop where the mouse let go")
	}
}

// markOpening is the code that turns on the highlight, without the text or
// what turns it off after.
func markOpening() string {
	return strings.SplitN(selectionColor.Render("marca"), "marca", 2)[0]
}

// ansiBeforeMark is the piece of the line before the highlight starts.
func ansiBeforeMark(line string) string {
	if cut := strings.Index(line, markOpening()); cut >= 0 {
		return line[:cut]
	}
	return line
}

// TestDraggingInTheCellCopies is the whole path: the button goes down on a
// cell that wasn't even focused, the mouse drags, and on release the
// stretch goes to the terminal's clipboard.
func TestDraggingInTheCellCopies(t *testing.T) {
	m := testModel(testGrid())
	area, has := m.visibleAreas()["c2"]
	if !has {
		t.Fatal("cell c2 should be visible on the grid")
	}

	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: area[0] + 2, Y: area[1]})
	m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: area[0] + 8, Y: area[1]})
	_, cmd := m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: area[0] + 8, Y: area[1]})

	if item := m.focusedCell(); item == nil || item.ID != "c2" {
		t.Fatalf("clicking a cell should also select it, focus at %#v", m.focus)
	}
	if cmd == nil {
		t.Fatal("releasing the button should send the stretch to the clipboard")
	}
	if copied := fmt.Sprint(cmd()); copied != "go test" {
		t.Fatalf("the copied stretch came out %q, expected %q", copied, "go test")
	}
}

// TestEscClearsTheMark — the mark stays lit after copying, as a receipt, and
// leaves with esc like everything else.
func TestEscClearsTheMark(t *testing.T) {
	m := testModel(testGrid())
	area := m.visibleAreas()["c1"]

	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: area[0], Y: area[1]})
	m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: area[0] + 4, Y: area[1]})
	m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: area[0] + 4, Y: area[1]})

	if m.selection == nil {
		t.Fatal("the mark should stay lit after copying")
	}
	m.handleNavigate("esc")
	if m.selection != nil {
		t.Fatal("esc should clear the mark")
	}
}

// TestClickOnlySelectsTheCell — going down and up on the same spot is a
// click. It changes the focus and doesn't touch the clipboard, or selecting
// a cell would throw away what the user had copied before.
func TestClickOnlySelectsTheCell(t *testing.T) {
	m := testModel(testGrid())
	area := m.visibleAreas()["c3"]

	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: area[0] + 3, Y: area[1]})
	_, cmd := m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: area[0] + 3, Y: area[1]})

	if item := m.focusedCell(); item == nil || item.ID != "c3" {
		t.Fatalf("the click should select cell c3, focus at %#v", m.focus)
	}
	if cmd != nil {
		t.Fatal("a click with no drag copies nothing")
	}
	if m.selection != nil {
		t.Fatal("a click with no drag leaves no mark lit")
	}
}

// TestDraggingInTypeDoesNotSwapTheKeyboard — in TYPE the keyboard's owner
// doesn't change because of a click; the mark only works inside the cell
// that already holds the keyboard.
func TestDraggingInTypeDoesNotSwapTheKeyboard(t *testing.T) {
	m := testModel(testGrid())
	m.typing = true
	area := m.visibleAreas()["c2"]

	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: area[0] + 2, Y: area[1]})

	if item := m.focusedCell(); item == nil || item.ID != "c1" {
		t.Fatalf("the keyboard should stay with c1, focus at %#v", m.focus)
	}
	if m.selection != nil {
		t.Fatal("in TYPE the mark doesn't start on a cell that doesn't hold the keyboard")
	}
}
