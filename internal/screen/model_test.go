package screen

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andreluiz/tesseract/internal/protocol"
)

// testModel builds the screen over a state, with no engine on the other
// side. Only the keys that touch focus are exercised here — the ones that
// talk to the engine are tested on its side.
func testModel(state protocol.State) *Model {
	return &Model{state: state, view: viewGrid, sizes: map[string]Dims{}, width: 120, height: 30}
}

func TestArrowsMoveThroughTheGrid(t *testing.T) {
	m := testModel(testGrid())

	m.handleNavigate("right")
	if m.focus.Cell != 1 {
		t.Fatalf("→ should go to the next cell, focus %#v", m.focus)
	}
	// On the grid every cell is in view, so moving crosses project.
	m.handleNavigate("right")
	if m.focus.Project != 1 || m.focus.Cell != 0 {
		t.Fatalf("→ should cross into the next project: %#v", m.focus)
	}
	m.handleNavigate("left")
	if m.focus.Project != 0 || m.focus.Cell != 1 {
		t.Fatalf("← should go back to the previous cell: %#v", m.focus)
	}
	m.focus = Focus{}
	m.handleNavigate("left")
	if m.focus.Project != 2 {
		t.Fatalf("← on the first cell wraps around: %#v", m.focus)
	}

	m.focus = Focus{}
	m.handleNavigate("down")
	if m.focus.Project != 1 || m.focus.Cell != 0 {
		t.Fatalf("↓ should go to the next project: %#v", m.focus)
	}
	m.handleNavigate("up")
	if m.focus.Project != 0 {
		t.Fatalf("↑ should go back: %#v", m.focus)
	}
}

func TestNumberGoesStraightToTheProject(t *testing.T) {
	m := testModel(testGrid())
	m.handleNavigate("3")
	if m.focus.Project != 2 {
		t.Fatalf("3 should lead to the third project: %#v", m.focus)
	}
	m.handleNavigate("9")
	if m.focus.Project != 2 {
		t.Fatalf("a number beyond what exists stays on the last one: %#v", m.focus)
	}
}

// TestSpaceJumpsToTheCaller — crosses project to find whoever's asking for
// attention.
func TestSpaceJumpsToTheCaller(t *testing.T) {
	m := testModel(testGrid())
	// From the first cell (which already answered), the jump goes to the
	// next one calling: the second project's, which asks for approval.
	m.handleNavigate("space")
	if m.focus.Project != 1 || m.focus.Cell != 0 {
		t.Fatalf("space should cross into the project that's calling: %#v", m.focus)
	}
	// From there, it wraps around and comes back to the first.
	m.handleNavigate("space")
	if m.focus.Project != 0 || m.focus.Cell != 0 {
		t.Fatalf("space should wrap around: %#v", m.focus)
	}
}

// TestSpaceWithNoOneCallingDoesNothing.
func TestSpaceWithNoOneCallingDoesNothing(t *testing.T) {
	state := testGrid()
	for i := range state.Projects {
		for j := range state.Projects[i].Cells {
			state.Projects[i].Cells[j].State = "working"
		}
	}
	m := testModel(state)
	m.focus = Focus{Project: 1}
	m.handleNavigate("space")
	if m.focus.Project != 1 {
		t.Fatalf("with no one calling, the focus stays put: %#v", m.focus)
	}
}

// TestFullScreenTogglesOnAndOff.
func TestFullScreenTogglesOnAndOff(t *testing.T) {
	m := testModel(testGrid())
	m.handleNavigate("o")
	if !m.focus.Full {
		t.Fatal("o should turn on full screen")
	}
	m.handleNavigate("o")
	if m.focus.Full {
		t.Fatal("o again should leave full screen")
	}
	m.handleNavigate("o")
	m.handleNavigate("esc")
	if m.focus.Full {
		t.Fatal("esc also leaves full screen")
	}
}

// TestHelpOpensAndCloses.
func TestHelpOpensAndCloses(t *testing.T) {
	m := testModel(testGrid())
	m.handleNavigate("?")
	if !m.help {
		t.Fatal("? should open help")
	}
	if m.Mode().String() != "BROWSE" {
		t.Fatal("help doesn't change who owns the keyboard")
	}
}

// TestModeShowsWhoHoldsTheKeyboard — there are never two owners at once.
func TestModeShowsWhoHoldsTheKeyboard(t *testing.T) {
	m := testModel(testGrid())
	if m.Mode().String() != "BROWSE" {
		t.Fatalf("the default is BROWSE, got %s", m.Mode())
	}
	m.typing = true
	if m.Mode().String() != "TYPE" {
		t.Fatalf("typing is TYPE, got %s", m.Mode())
	}
	m.form = NewForm(m.state.Types, "")
	if m.Mode().String() != "FORM" {
		t.Fatalf("with a form open the keyboard is its, got %s", m.Mode())
	}
	m.panel = NewDockerPanel("p1", "cortz")
	if m.Mode().String() != "DOCKER" {
		t.Fatalf("with the panel open the keyboard is its, got %s", m.Mode())
	}
}

// TestFocusNeverLeavesWhatExists — a state that shrinks doesn't leave the
// screen pointing at nothing.
func TestFocusNeverLeavesWhatExists(t *testing.T) {
	m := testModel(testGrid())
	m.focus = Focus{Project: 2, Cell: 5}
	m.state = protocol.State{Projects: testGrid().Projects[:1]}
	m.focus = Adjust(m.state, m.focus)
	if m.focus.Project != 0 || m.focus.Cell != 1 {
		t.Fatalf("the focus should have been pulled inside what exists: %#v", m.focus)
	}

	m.state = protocol.State{}
	m.focus = Adjust(m.state, m.focus)
	if m.focus.Project != 0 || m.focus.Cell != 0 {
		t.Fatalf("with no project at all, the focus resets: %#v", m.focus)
	}
	if m.focusedCell() != nil {
		t.Fatal("with no cell, there's no focused cell")
	}
}

// TestPasteGoesWholeToTheCell — paste doesn't arrive as a key: the terminal
// sends the whole content at once, in its own event. If the screen ignored
// that event, ctrl-v would do nothing inside the cell.
func TestPasteGoesWholeToTheCell(t *testing.T) {
	m := testModel(testGrid())
	here, there := net.Pipe()
	defer here.Close()
	defer there.Close()
	m.client = &Client{conn: here, encoder: json.NewEncoder(here), decoder: json.NewDecoder(here)}
	m.typing = true

	received := make(chan protocol.Key, 1)
	go func() {
		var envelope protocol.Message
		if err := json.NewDecoder(there).Decode(&envelope); err != nil {
			return
		}
		key, _ := protocol.Unpack[protocol.Key](envelope)
		received <- key
	}()

	pasted := "primeira linha\nsegunda linha"
	m.Update(tea.PasteMsg{Content: pasted})

	select {
	case key := <-received:
		if key.Cell != "c1" {
			t.Fatalf("the pasted text went to cell %q, expected the focused c1", key.Cell)
		}
		if key.Paste != pasted {
			t.Fatalf("the pasted text should arrive whole, got %q", key.Paste)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pasting in TYPE sent nothing to the cell")
	}
}

// TestPasteInTheFormFitsOneLine — a single-line field doesn't accept the
// line break that came along with the copied path.
func TestPasteInTheFormFitsOneLine(t *testing.T) {
	m := testModel(testGrid())
	m.form = NewForm(nil, "")
	m.form.fields[0].value = ""

	m.Update(tea.PasteMsg{Content: "/home/dev/doxar-api\n"})

	if value := m.form.fields[0].value; value != "/home/dev/doxar-api" {
		t.Fatalf("the pasted path should come in without the line break, got %q", value)
	}
}

// TestArrowsOnA2x2GridMoveLiterally — a project's four cells become 2x2, and
// the arrow follows the drawing: ↓ drops a row instead of moving sideways.
func TestArrowsOnA2x2GridMoveLiterally(t *testing.T) {
	state := protocol.State{Projects: []protocol.Project{{ID: "p1", Name: "regula-mais", Path: "/dev/rm"}}}
	for i := range 4 {
		state.Projects[0].Cells = append(state.Projects[0].Cells, protocol.Cell{
			ID: "c" + string(rune('0'+i)), Type: "session", Name: "session", State: "working", Live: true,
		})
	}
	m := &Model{state: state, view: viewGrid, sizes: map[string]Dims{}, width: 200, height: 40}

	// 0 1
	// 2 3
	for _, c := range []struct {
		from     int
		key      string
		expected int
	}{
		{0, "right", 1}, {1, "left", 0},
		{0, "down", 2}, {2, "up", 0},
		{1, "down", 3}, {3, "up", 1},
		{2, "right", 3}, {3, "left", 2},
	} {
		m.focus = Focus{Cell: c.from}
		m.handleNavigate(c.key)
		if m.focus.Project != 0 || m.focus.Cell != c.expected {
			t.Errorf("%s from cell %d should go to %d, went to %#v", c.key, c.from, c.expected, m.focus)
		}
	}
}
