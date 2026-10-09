package screen

import (
	"strings"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// indexWidth is the left column of the list: the projects and their cells.
const indexWidth = 30

// PreviewInner is the space left for the live preview of the selected cell,
// to the right of the index: the cell's own box, minus its line.
func PreviewInner(width, height int) Dims {
	return Dims{
		Cols: max(width-indexWidth-3, 1),
		Rows: max(height-3, 1),
	}
}

// DrawList is the index: the same content as the grid, without video, with a
// live preview of the selected cell on the right. It's for scanning many
// projects and for a short terminal.
//
// The index is a column of text with no frame of its own, and the preview is
// the cell itself, drawn by the same hand that draws it on the grid — so the
// two modes read here exactly as they do there.
func DrawList(state protocol.State, focus Focus, mode keyboard.Mode, width, height int, err string) string {
	if len(state.Projects) == 0 {
		return emptyScreen(state, mode, width, height, err)
	}
	focus = Adjust(state, focus)
	body := height - 1
	project := state.Projects[focus.Project]

	var item protocol.Cell
	if len(project.Cells) > 0 {
		item = project.Cells[focus.Cell]
	}
	index := indexLines(state, focus, body, mode)
	preview := cellBox(item, projectHue(project), mode, true, max(width-indexWidth-1, 3), body)

	lines := make([]string, 0, height)
	for l := range body {
		entry, box := "", ""
		if l < len(index) {
			entry = index[l]
		}
		if l < len(preview) {
			box = preview[l]
		}
		lines = append(lines, pad(pad(entry, indexWidth)+" "+box, width))
	}
	lines = append(lines, footerFor(state, &item, mode, width, err, 0))
	return strings.Join(lines, "\n")
}

// indexLines builds the left column: each project's name in its hue, and
// its cells under it — a state's glyph and a name. While a cell
// holds the keyboard the index dims — except for the glyph of whoever is
// blocked: that one can't wait for you to come back.
func indexLines(state protocol.State, focus Focus, height int, mode keyboard.Mode) []string {
	silent := mode == keyboard.Type
	var lines []string
	var focusLine int
	for i, project := range state.Projects {
		if i > 0 {
			lines = append(lines, "")
		}
		hue := projectHue(project)
		lines = append(lines, projectDivider(project, indexWidth, mode, i == focus.Project))
		for j, item := range project.Cells {
			marker := markerFor(item.State)
			if marker.symbol == "" {
				// No marker still takes the marker's column.
				marker.symbol = " "
			}
			focused := i == focus.Project && j == focus.Cell
			if focused {
				focusLine = len(lines)
			}
			name := truncate(item.Name, indexWidth-6)
			// The bar on the left is the focus, in the project's hue: it's
			// what's left of the selection with no color.
			mark := " "
			if focused {
				mark = hue.Line(true).Render("▌")
			}
			if focused {
				// The raised row keeps the glyph in its state's color: the
				// selection is where you are, not what the cell is doing.
				glyph := marker.tone().Inherit(selectedRowColor).Render(marker.symbol)
				row := selectedRowColor.Render(" ") + glyph + selectedRowColor.Render(pad(" "+name, indexWidth-4))
				lines = append(lines, " "+mark+row)
				continue
			}
			// A blocked cell is a glyph and a color here, never a bar: the
			// solid area is the cell's own top line, in the preview, and a
			// second one in the index would be approve said twice.
			paintGlyph, paintName := marker.tone(), mutedColor
			if silent {
				paintName = faintColor
				if !(marker.blocks && item.Live) {
					paintGlyph = faintColor
				}
			}
			lines = append(lines, "   "+paintGlyph.Render(marker.symbol)+" "+paintName.Render(name))
		}
	}
	return windowLines(lines, focusLine, height)
}

// windowLines cuts the list to fit the height, keeping the focus visible.
func windowLines(lines []string, focus, height int) []string {
	if len(lines) <= height {
		return lines
	}
	first := min(max(focus-height/2, 0), len(lines)-height)
	return lines[first : first+height]
}
