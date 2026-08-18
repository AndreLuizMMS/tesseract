package screen

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// selectionColor is the marked stretch in reverse video. The color the agent
// wrote fades out while the stretch is marked — what matters there is where
// the mark starts and ends.
var selectionColor = theme.Paint(theme.BgVoid, theme.FgDefault)

// Selection is the stretch marked with the mouse inside a cell, from the
// anchor (where the button went down) to the current point. The coordinates
// are the cell's inner content, not the screen's: the box can move and the
// stretch stays the same.
//
// ponytail: the mark only covers the window currently in view. Scrolling the
// history before dragging already covers the common case; marking more than
// fits on screen would need saving a position in the history, and that's the
// engine's job, not the screen's.
type Selection struct {
	Cell     string
	AnchorX  int
	AnchorY  int
	CurrentX int
	CurrentY int
	Dragging bool
}

// Empty is true when the mouse went down and up in the same spot: that's a
// click, which only selects the cell, not a mark.
func (s Selection) Empty() bool {
	return s.AnchorX == s.CurrentX && s.AnchorY == s.CurrentY
}

// ordered puts the anchor and the current point in reading order, so
// dragging bottom-to-top counts the same as top-to-bottom.
func (s Selection) ordered() (x1, y1, x2, y2 int) {
	if s.AnchorY < s.CurrentY || (s.AnchorY == s.CurrentY && s.AnchorX <= s.CurrentX) {
		return s.AnchorX, s.AnchorY, s.CurrentX, s.CurrentY
	}
	return s.CurrentX, s.CurrentY, s.AnchorX, s.AnchorY
}

// span says which columns of the line are marked. Like on a terminal, the
// first line runs from the anchor to the end and the last one starts at the
// margin — that's how a whole paragraph comes out.
func (s Selection) span(line, columns int) (from, to int, has bool) {
	x1, y1, x2, y2 := s.ordered()
	if line < y1 || line > y2 || columns <= 0 {
		return 0, 0, false
	}
	from, to = 0, columns
	if line == y1 {
		from = max(x1, 0)
	}
	if line == y2 {
		to = min(x2+1, columns)
	}
	if to <= from {
		return 0, 0, false
	}
	return from, to, true
}

// Text is what goes to the clipboard: just the letters, no color and no
// trailing spaces at the end of each line.
func (s Selection) Text(lines []string, columns int) string {
	_, y1, _, y2 := s.ordered()
	var pieces []string
	for line := y1; line <= y2; line++ {
		from, to, has := s.span(line, columns)
		if !has {
			continue
		}
		text := ""
		if line >= 0 && line < len(lines) {
			text = ansi.Cut(ansi.Strip(lines[line]), from, to)
		}
		pieces = append(pieces, strings.TrimRight(text, " "))
	}
	// Dragging past the cell's last line doesn't count as a blank line at
	// the end.
	return strings.TrimRight(strings.Join(pieces, "\n"), "\n")
}

// Paint returns the lines with the marked stretch highlighted. Blank lines
// inside the mark light up too: dragging over an empty area still has to
// show that it was marked.
func (s Selection) Paint(lines []string, columns int) []string {
	_, _, _, y2 := s.ordered()
	painted := make([]string, max(len(lines), y2+1))
	copy(painted, lines)

	for i := range painted {
		from, to, has := s.span(i, columns)
		if !has {
			continue
		}
		marked := ansi.Strip(ansi.Cut(painted[i], from, to))
		marked += strings.Repeat(" ", max(to-from-lipgloss.Width(marked), 0))
		painted[i] = ansi.Cut(painted[i], 0, from) + selectionColor.Render(marked) + ansi.Cut(painted[i], to, columns)
	}
	return painted
}

// withSelection returns the snapshot with the marked stretch painted into
// its cell. The mark belongs to the screen and dies here: the engine never
// finds out about it.
func withSelection(state protocol.State, s Selection, columns int) protocol.State {
	for i, project := range state.Projects {
		for j, item := range project.Cells {
			if item.ID != s.Cell {
				continue
			}
			item.Lines = s.Paint(item.Lines, columns)
			cells := append([]protocol.Cell(nil), project.Cells...)
			cells[j] = item
			projects := append([]protocol.Project(nil), state.Projects...)
			projects[i].Cells = cells
			state.Projects = projects
			return state
		}
	}
	return state
}
