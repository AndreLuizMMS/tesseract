package screen

import (
	"strconv"
	"strings"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// indexWidth is the left column of the list: the projects and their cells.
const indexWidth = 30

// PreviewInner is the space left for the live preview of the selected cell,
// to the right of the index.
func PreviewInner(width, height int) Dims {
	return Dims{
		Cols: max(width-indexWidth-3, 1),
		Rows: max(height-barHeight-3, 1),
	}
}

// DrawList is the index: the same content as the grid, without video, with a
// live preview of the selected cell on the right. It's for scanning many
// projects and for a short terminal.
//
// The two modes read here the same way they do on the grid: a single line
// while you command the application, and the preview — the cell that took
// the keyboard — closed in a double phosphor line while the index sinks.
func DrawList(state protocol.State, focus Focus, mode keyboard.Mode, width, height int, err string) string {
	if len(state.Projects) == 0 {
		return emptyScreen(state, mode, width, height, err)
	}
	focus = Adjust(state, focus)
	body := height - barHeight - 3
	typing := mode == keyboard.Type

	index := indexLines(state, focus, body, typing)
	preview := PreviewInner(width, height)

	var item protocol.Cell
	if cells := state.Projects[focus.Project].Cells; len(cells) > 0 {
		item = cells[focus.Cell]
	}

	// The index's frame and the preview's, stroke by stroke: the index never
	// changes its line, the preview takes the mode's.
	single, frame := theme.Browse.Frame(), theme.Browse.Frame()
	paintCorner, paintEdge := borderColor, lineColor
	paintOwnCorner, paintOwnEdge := borderColor, lineColor
	joinTop, joinBottom := "┬", "┴"
	if typing {
		frame = theme.Type.Frame()
		paintCorner, paintEdge = dimLineColor, dimLineColor
		paintOwnCorner, paintOwnEdge = typingColor, typingColor
		joinTop, joinBottom = frame[0], frame[2]
	}
	indexRule := strings.Repeat(single[4], indexWidth)
	previewRule := strings.Repeat(frame[4], preview.Cols)
	left, divider, right := paintEdge.Render(single[5]), paintOwnEdge.Render(frame[5]), paintOwnEdge.Render(frame[5])

	lines := titleBar(mode, width, countCalls(state), state.Quota)
	lines = append(lines, paintCorner.Render(single[0])+paintEdge.Render(indexRule)+
		paintOwnCorner.Render(joinTop)+paintOwnEdge.Render(previewRule)+paintOwnCorner.Render(frame[1]))
	for l := range body {
		entry := ""
		if l < len(index) {
			entry = index[l]
		}
		content := ""
		switch {
		case l == 0:
			// The preview's header is a raised band, like a cell's.
			content = boxTitleColor.Render(pad(" "+item.Type+" · "+item.Name, preview.Cols))
		case l >= 2 && l-2 < len(item.Lines):
			content = item.Lines[l-2]
		}
		lines = append(lines, left+pad(entry, indexWidth)+
			divider+surfaceInk.Under(pad(content, preview.Cols))+right)
	}
	lines = append(lines, paintCorner.Render(single[2])+paintEdge.Render(indexRule)+
		paintOwnCorner.Render(joinBottom)+paintOwnEdge.Render(previewRule)+paintOwnCorner.Render(frame[3]))
	lines = append(lines, footer(mode, width, err))
	return strings.Join(lines, "\n")
}

// indexLines builds the left column: project, cells and Docker. With silent
// set a cell holds the keyboard, and the index dims — except for whoever is
// blocked, which stays a solid bar: that one can't wait for you to come back.
func indexLines(state protocol.State, focus Focus, height int, silent bool) []string {
	paintNumber, paintProject, paintType, paintName := keyColor, titleColor, barColor, mutedColor
	if silent {
		paintNumber, paintProject, paintType, paintName = faintColor, faintColor, faintColor, faintColor
	}
	var lines []string
	var focusLine int
	for i, project := range state.Projects {
		if i > 0 {
			lines = append(lines, "")
		}
		number := ""
		if i < 9 {
			number = strconv.Itoa(i+1) + " "
		}
		path := shortenPath(project.Path, indexWidth-len(number)-len(project.Name)-3)
		lines = append(lines, " "+paintNumber.Render(number)+paintProject.Render(strings.ToUpper(project.Name))+" "+faintColor.Render(path))
		for j, item := range project.Cells {
			marker := markerFor(item.State)
			focused := i == focus.Project && j == focus.Cell
			if focused {
				focusLine = len(lines)
			}
			row := marker.symbol + " " + pad(item.Type, 8) + truncate(item.Name, indexWidth-14)
			switch {
			case marker.blocks && item.Live:
				// Urgency is filled area here too: the blocked cell takes the
				// whole line of the index.
				mark := " "
				if focused {
					mark = selectedBar
				}
				lines = append(lines, mark+marker.color().Render(pad(" "+row, indexWidth-1)))
			case focused:
				lines = append(lines, selectedBar+selectedRowColor.Render(pad(" "+row, indexWidth-1)))
			default:
				paintGlyph := marker.tone()
				if silent {
					paintGlyph = faintColor
				}
				lines = append(lines, "  "+paintGlyph.Render(marker.symbol)+" "+
					paintType.Render(pad(item.Type, 8))+paintName.Render(truncate(item.Name, indexWidth-14)))
			}
		}
		if project.HasCompose {
			dockerState := project.Docker
			if dockerState == "" {
				dockerState = "stopped"
			}
			// Docker isn't a cell and has no glyph in the alphabet: the
			// column where the marker would be stays empty.
			lines = append(lines, "    "+paintName.Render(pad("docker", 8)+dockerState))
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

// shortenPath swaps the home directory for ~ and cuts from the left when it
// doesn't fit.
func shortenPath(path string, width int) string {
	if width < 4 {
		return ""
	}
	short := path
	if h := home(); h != "" && strings.HasPrefix(short, h) {
		short = "~" + strings.TrimPrefix(short, h)
	}
	if len(short) <= width {
		return short
	}
	// Cut on a slash, so the leftover piece keeps looking like a path.
	leftover := short[len(short)-width+1:]
	if slash := strings.Index(leftover, "/"); slash >= 0 {
		leftover = leftover[slash:]
	}
	return "…" + leftover
}
