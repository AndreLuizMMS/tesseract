package screen

import (
	"strings"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
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
func DrawList(state protocol.State, focus Focus, mode keyboard.Mode, width, height int, err string) string {
	if len(state.Projects) == 0 {
		return emptyScreen(mode, width, height, err, state.Notice)
	}
	focus = Adjust(state, focus)
	body := height - barHeight - 3

	index := indexLines(state, focus, body)
	preview := PreviewInner(width, height)

	var item protocol.Cell
	if cells := state.Projects[focus.Project].Cells; len(cells) > 0 {
		item = cells[focus.Cell]
	}

	lines := titleBar(mode, width, countCalls(state), state.Quota)
	lines = append(lines, borderColor.Render("┌"+strings.Repeat("─", indexWidth)+"┬"+strings.Repeat("─", preview.Cols)+"┐"))
	header := titleColor.Render(" " + item.Type + " · " + item.Name)
	for l := range body {
		left := ""
		if l < len(index) {
			left = index[l]
		}
		right := ""
		switch {
		case l == 0:
			right = header
		case l >= 2 && l-2 < len(item.Lines):
			right = item.Lines[l-2]
		}
		lines = append(lines,
			borderColor.Render("│")+pad(left, indexWidth)+
				borderColor.Render("│")+pad(right, preview.Cols)+
				borderColor.Render("│"))
	}
	lines = append(lines, borderColor.Render("└"+strings.Repeat("─", indexWidth)+"┴"+strings.Repeat("─", preview.Cols)+"┘"))
	lines = append(lines, footer(mode, width, err))
	return strings.Join(lines, "\n")
}

// indexLines builds the left column: project, cells and Docker.
func indexLines(state protocol.State, focus Focus, height int) []string {
	var lines []string
	var focusLine int
	for i, project := range state.Projects {
		if i > 0 {
			lines = append(lines, "")
		}
		path := shortenPath(project.Path, indexWidth-len(project.Name)-3)
		lines = append(lines, projectColor(project.Color).Render(" "+strings.ToUpper(project.Name))+" "+faintColor.Render(path))
		for j, item := range project.Cells {
			mark := "  "
			if i == focus.Project && j == focus.Cell {
				mark = focusedBorderColor.Render("▸ ")
				focusLine = len(lines)
			}
			marker := markerFor(item.State)
			lines = append(lines, mark+marker.color().Render(marker.symbol)+" "+
				barColor.Render(pad(item.Type, 8))+faintColor.Render(truncate(item.Name, indexWidth-14)))
		}
		if project.HasCompose {
			dockerState := project.Docker
			if dockerState == "" {
				dockerState = "stopped"
			}
			lines = append(lines, "  "+faintColor.Render("● "+pad("docker", 8)+dockerState))
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
