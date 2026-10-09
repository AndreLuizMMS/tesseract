package screen

import (
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/theme"
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
		if !strings.Contains(list, project.Name) || !strings.Contains(grid, project.Name) {
			t.Errorf("the list doesn't show project %q", project.Name)
		}
		for _, item := range project.Cells {
			if !strings.Contains(list, item.Name) {
				t.Errorf("the list doesn't show cell %q", item.Name)
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

// TestListSaysTheStackOnlyWhenItFell — a stack that's fine is nothing to
// read; one with a fallen service is said beside its project, here as on the
// grid.
func TestListSaysTheStackOnlyWhenItFell(t *testing.T) {
	state := testGrid()
	if list := noStyle(DrawList(state, Focus{}, keyboard.Browse, 140, 30, "")); strings.Contains(list, "docker") {
		t.Errorf("a stack with nothing wrong isn't on the list:\n%s", list)
	}
	state.Projects[0].DockerDown = true
	for name, drawing := range map[string]string{
		"list": DrawList(state, Focus{}, keyboard.Browse, 140, 30, ""),
		"grid": Draw(state, Focus{}, keyboard.Browse, 140, 30, ""),
	} {
		if !strings.Contains(noStyle(drawing), "doxar-api  ✖ docker") {
			t.Errorf("the %s should say a service fell:\n%s", name, noStyle(drawing))
		}
	}
}

// TestListPreviewIsTheCell — the preview is drawn by the hand that draws the
// cell on the grid: same top line, same word once it holds the keyboard, and
// the size reported to the engine is the one drawn.
func TestListPreviewIsTheCell(t *testing.T) {
	state := testGrid()
	const width, height = 100, 16
	lines := strings.Split(noStyle(DrawList(state, Focus{Project: 0, Cell: 1}, keyboard.Type, width, height, "")), "\n")
	if !strings.Contains(lines[0], "╭ claude  cursor  bash  testes") || !strings.HasSuffix(lines[0], " ▸ ╮") {
		t.Errorf("the preview opens with the cell's own top line: %q", lines[0])
	}
	if !strings.HasSuffix(lines[height-2], " TYPE ─╯") {
		t.Errorf("the preview wears the word once it holds the keyboard: %q", lines[height-2])
	}
	inner := PreviewInner(width, height)
	content := []rune(lines[1])
	if got := len(content) - (indexWidth + 2) - 1; got != inner.Cols {
		t.Errorf("the preview draws %d columns and reports %d", got, inner.Cols)
	}
	if got := height - 3; got != inner.Rows {
		t.Errorf("the preview draws %d rows and reports %d", got, inner.Rows)
	}
	if got := string(content[indexWidth+2:]); !strings.HasPrefix(got, "$ go test ./...") {
		t.Errorf("the cursor's origin sits on the first content line: %q", got)
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

// TestListKeepsOneSolidBar — approve is the only solid area, and one per
// blocked cell: in the list it's the top line of the cell in the preview.
// The index row says the same with a glyph in the state's color and no fill;
// and the raised row of the selection never repaints a glyph as plain text.
func TestListKeepsOneSolidBar(t *testing.T) {
	inFullColor(t)
	state := testGrid()
	bar := codeOf(theme.StateBlock, true)
	amber := codeOf(theme.StateBlock, false)
	// cortz-web's "fix nav" is the blocked one: once in focus, once not.
	for _, focus := range []Focus{{Project: 1, Cell: 0}, {Project: 0, Cell: 0}} {
		for _, index := range indexLines(state, focus, 29, keyboard.Browse) {
			if strings.Contains(index, bar) {
				t.Fatalf("the index drew a solid bar: %q", index)
			}
			if strings.Contains(noStyle(index), "fix nav") && !strings.Contains(index, amber) {
				t.Errorf("the blocked row lost its state's color: %q", index)
			}
		}
	}
	focused := DrawList(state, Focus{Project: 1, Cell: 0}, keyboard.Browse, 140, 30, "")
	if top := strings.Split(focused, "\n")[0]; !strings.Contains(top, bar) || !strings.Contains(noStyle(top), "APPROVE") {
		t.Errorf("the preview keeps the cell's own bar: %q", noStyle(top))
	}

	// The selected row of a cell with an unread answer: the glyph stays blue.
	answered := strings.Split(DrawList(state, Focus{Project: 0, Cell: 0}, keyboard.Browse, 140, 30, ""), "\n")[1]
	if !strings.Contains(noStyle(answered), "⬤ refatora auth") || !strings.Contains(answered, codeOf(theme.Do(theme.Answered).Color, false)) {
		t.Errorf("the selected row repainted the state's glyph: %q", answered)
	}
}
