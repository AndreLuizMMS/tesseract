package screen

import (
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/keyboard"
)

// TestListAndGridTellTheSameStory is the mechanical guarantee of the rule:
// the screen decides nothing, so the list and the grid fed the same state
// show the same projects, the same cells and the same markers. It's
// structurally impossible for the two to disagree.
func TestListAndGridTellTheSameStory(t *testing.T) {
	state := testGrid()
	focus := Focus{Project: 0, Cell: 0}

	grid := noStyle(Draw(state, focus, keyboard.Browse, 140, 30, ""))
	list := noStyle(DrawList(state, focus, keyboard.Browse, 140, 30, ""))

	for _, project := range state.Projects {
		if !strings.Contains(strings.ToUpper(list), strings.ToUpper(project.Name)) {
			t.Errorf("the list doesn't show project %q", project.Name)
		}
		for _, item := range project.Cells {
			if !strings.Contains(list, item.Name) {
				t.Errorf("the list doesn't show cell %q", item.Name)
			}
			if !strings.Contains(list, item.Type) {
				t.Errorf("the list doesn't show the type of cell %q", item.Name)
			}
			marker := markerFor(item.State).symbol
			if !strings.Contains(list, marker) {
				t.Errorf("the list doesn't show marker %q of cell %q", marker, item.Name)
			}
			if !strings.Contains(grid, marker) && project.ID == state.Projects[focus.Project].ID {
				t.Errorf("the grid doesn't show marker %q of cell %q", marker, item.Name)
			}
		}
	}

	// The focused cell shows up in both, and with the same content in both.
	focused := state.Projects[0].Cells[0]
	for _, line := range focused.Lines {
		if !strings.Contains(grid, line) {
			t.Errorf("the grid doesn't show line %q of the focused cell", line)
		}
		if !strings.Contains(list, line) {
			t.Errorf("the list's preview doesn't show line %q of the focused cell", line)
		}
	}
}

// TestListShowsTheProjectsDocker — compose belongs to the project, and the
// list says so.
func TestListShowsTheProjectsDocker(t *testing.T) {
	state := testGrid()
	list := noStyle(DrawList(state, Focus{}, keyboard.Browse, 140, 30, ""))
	if !strings.Contains(list, "docker") || !strings.Contains(list, "4/5") {
		t.Errorf("the list should show the stack's state:\n%s", list)
	}
}

// TestListFitsOnScreen — nothing renders crooked on a tight terminal.
func TestListFitsOnScreen(t *testing.T) {
	state := testGrid()
	for _, size := range [][2]int{{140, 30}, {100, 14}, {80, 10}} {
		width, height := size[0], size[1]
		lines := strings.Split(DrawList(state, Focus{Project: 1}, keyboard.Browse, width, height, ""), "\n")
		if len(lines) != height {
			t.Errorf("%dx%d: the list has %d lines", width, height, len(lines))
		}
		for _, line := range lines {
			if visible := len([]rune(noStyle(line))); visible > width {
				t.Errorf("%dx%d: line with %d columns", width, height, visible)
			}
		}
	}
}

// TestListGolden pins the drawing of the index with preview.
func TestListGolden(t *testing.T) {
	checkGolden(t, "list.txt", DrawList(testGrid(), Focus{Project: 0, Cell: 1}, keyboard.Browse, 120, 24, ""))
}
