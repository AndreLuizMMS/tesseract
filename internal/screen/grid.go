package screen

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
)

const (
	// minCellWidth is the least a cell needs for its content to still say
	// something. A row fills up to this before dropping to the next line.
	minCellWidth = 34
	// tightCellWidth is how far a cell shrinks when the alternative is the
	// row spilling off the screen. Tight, but still readable.
	tightCellWidth = 24
	// minCellHeight is a top border, three lines of content and a bottom
	// border.
	minCellHeight = 5
	// minColumnWidth is the least a column of projects needs to still fit two
	// cells in it.
	minColumnWidth = 2 * tightCellWidth
	// minProjectHeight is the divider plus the shortest cell that still says
	// something. When projects don't fit in this, the screen opens another
	// column instead of leaving a cell out of frame.
	minProjectHeight = minCellHeight + 1
)

// Focus is where the user is right now.
type Focus struct {
	Project int
	Cell    int
	Full    bool
}

// Dims is the usable area the screen reserved for a cell's inner
// content. The engine uses this to give the process inside it a terminal of
// the right size.
type Dims struct {
	Cols int
	Rows int
}

// row is one line of cells from the same project, side by side. A project
// with many cells occupies several rows; a project with one cell takes up
// the whole width. This is what makes every cell fit on screen without any
// of them turning into a strip.
type row struct {
	project int
	cells   []int
	// opens marks the project's first row, the one that carries its name.
	opens bool
	// height is how much room it got in the screen's split.
	height int
}

// column is a vertical slice of the screen with the projects that fit in
// it. The screen is a single column as long as the projects fit in the
// height; when they don't, the grid opens columns instead of squeezing a row
// or pushing a cell off screen.
type column struct {
	rows  []row
	x     int
	width int
	// hidden counts this column's cells that didn't fit in the height.
	hidden int
}

// Layout is the result of the layout calculation. Drawing and reporting the
// size to the engine both come from here, so they never disagree.
type Layout struct {
	columns []column
	inners  map[string]Dims
	origins map[string][2]int
	// hidden counts the cells that didn't fit on screen.
	hidden int
}

// allRows joins the rows of every column, in the order the projects appear.
func (d Layout) allRows() []row {
	var rows []row
	for _, c := range d.columns {
		rows = append(rows, c.rows...)
	}
	return rows
}

// Inners is what the screen reports to the engine: the usable size of each
// visible cell.
func (d Layout) Inners() map[string]Dims { return d.inners }

// Arrange calculates the grid layout: every cell of every project at once,
// using as much space as possible, split by project.
func Arrange(state protocol.State, focus Focus, width, height int) Layout {
	d := Layout{inners: map[string]Dims{}, origins: map[string][2]int{}}
	if len(state.Projects) == 0 || width < 12 || height < 6 {
		return d
	}
	focus = Adjust(state, focus)
	focusedProject := state.Projects[focus.Project]
	if len(focusedProject.Cells) == 0 {
		return d
	}

	if focus.Full {
		item := focusedProject.Cells[focus.Cell]
		inner := FullCellInner(width, height)
		d.inners[item.ID] = inner
		d.origins[item.ID] = [2]int{1, 2}
		return d
	}

	body := height - barHeight - 1 // header and footer
	d.columns = splitColumns(state, focus, width, body)

	for _, c := range d.columns {
		d.hidden += c.hidden
		y := barHeight // header
		for _, r := range c.rows {
			if r.opens {
				y++
			}
			project := state.Projects[r.project]
			x := c.x
			for i, cellWidth := range splitWidth(len(r.cells), c.width) {
				item := project.Cells[r.cells[i]]
				d.inners[item.ID] = Dims{
					Cols: max(cellWidth-2, 1),
					Rows: max(r.height-2, 1),
				}
				d.origins[item.ID] = [2]int{x + 1, y + 1}
				x += cellWidth
			}
			y += r.height
		}
	}
	return d
}

// splitColumns splits the projects into columns and plans each one's rows.
// It's what makes the grid grow sideways before it grows downward.
func splitColumns(state protocol.State, focus Focus, width, body int) []column {
	how := columnCount(len(state.Projects), width, body)
	perColumn := (len(state.Projects) + how - 1) / how
	how = (len(state.Projects) + perColumn - 1) / perColumn
	// one column of breathing room between one column of projects and the
	// next, or one divider would look like it continues into the other.
	widths := splitWidth(how, width-(how-1))

	columns := make([]column, 0, how)
	x := 0
	for start := 0; start < len(state.Projects); start += perColumn {
		c := column{x: x, width: widths[len(columns)]}
		all := planRows(state, start, min(start+perColumn, len(state.Projects)), c.width, body)
		visible := rowWindow(all, rowOfFocus(all, focus), body)
		c.hidden = hiddenCells(all, visible)
		c.rows = splitHeight(visible, body)
		columns = append(columns, c)
		x += c.width + 1
	}
	return columns
}

// columnCount decides how many columns the screen splits into: just one
// while the projects fit the height with room to spare, more when the
// alternative would be a squeezed row or a cell falling off screen.
func columnCount(projects, width, body int) int {
	if projects <= 1 {
		return 1
	}
	fitInHeight := max(body/minProjectHeight, 1)
	needed := (projects + fitInHeight - 1) / fitInHeight
	return min(needed, max(width/minColumnWidth, 1), projects)
}

// planRows breaks each project into rows of cells that fit the width,
// balanced so no row is left with just one cell. When the rows don't fit
// the height the project has, cells squeeze instead of falling off screen.
func planRows(state protocol.State, start, end, width, body int) []row {
	projectHeight := max(body/max(end-start, 1)-1, minCellHeight)
	rowsThatFit := max(projectHeight/minCellHeight, 1)
	var rows []row
	for iProject := start; iProject < end; iProject++ {
		project := state.Projects[iProject]
		total := len(project.Cells)
		if total == 0 {
			continue
		}
		rowCount := min(squareRows(total), rowsThatFit)
		fitInWidth := min(max(width/minCellWidth, 1), (total+rowCount-1)/rowCount)
		perRow := balance(total, fitInWidth)
		if (total+perRow-1)/perRow > rowsThatFit {
			tight := min((total+rowsThatFit-1)/rowsThatFit, max(width/tightCellWidth, 1))
			perRow = balance(total, max(tight, perRow))
		}
		for start := 0; start < total; start += perRow {
			end := min(start+perRow, total)
			indices := make([]int, 0, end-start)
			for i := start; i < end; i++ {
				indices = append(indices, i)
			}
			rows = append(rows, row{project: iProject, cells: indices, opens: start == 0})
		}
	}
	return rows
}

// squareRows says how many rows a project's cells split into for the
// squarest grid: four cells become 2x2 instead of a strip of four, two stay
// side by side. It's the rounded square root of the total.
func squareRows(total int) int {
	rows := 1
	for rows*(rows+1) < total {
		rows++
	}
	return rows
}

// balance says how many cells to put per row so no row ends up much emptier
// than another, respecting the cap on how many fit.
func balance(total, fit int) int {
	rows := (total + fit - 1) / fit
	return (total + rows - 1) / rows
}

// rowOfFocus finds which row the focused cell is in.
func rowOfFocus(rows []row, focus Focus) int {
	for i, r := range rows {
		if r.project != focus.Project {
			continue
		}
		for _, cell := range r.cells {
			if cell == focus.Cell {
				return i
			}
		}
	}
	return 0
}

// rowWindow chooses which rows show up when not all of them fit, keeping the
// focused one always visible.
func rowWindow(rows []row, focused, body int) []row {
	fit := max(body/(minCellHeight+1), 1)
	if fit >= len(rows) {
		return rows
	}
	first := min(max(focused-fit/2, 0), len(rows)-fit)
	window := append([]row{}, rows[first:first+fit]...)
	// The window's first row always says which project it belongs to, even
	// if the project's name got cut off above.
	window[0].opens = true
	return window
}

func hiddenCells(all, visible []row) int {
	count := func(rows []row) int {
		total := 0
		for _, r := range rows {
			total += len(r.cells)
		}
		return total
	}
	return count(all) - count(visible)
}

// splitHeight splits the height across the rows, subtracting the project
// name lines. The remainder of the division goes to the first ones.
func splitHeight(rows []row, body int) []row {
	if len(rows) == 0 {
		return nil
	}
	headers := 0
	for _, r := range rows {
		if r.opens {
			headers++
		}
	}
	available := max(body-headers, len(rows))
	base := available / len(rows)
	remainder := available % len(rows)

	split := append([]row{}, rows...)
	for i := range split {
		split[i].height = base
		if i < remainder {
			split[i].height++
		}
	}
	return split
}

// splitWidth splits the width across a row's cells.
func splitWidth(how, width int) []int {
	if how == 0 {
		return nil
	}
	base := width / how
	remainder := width % how
	widths := make([]int, how)
	for i := range widths {
		widths[i] = base
		if i < remainder {
			widths[i]++
		}
	}
	return widths
}

// Draw builds the grid: every open cell at once, grouped by project, each
// one taking up as much space as the screen allows.
func Draw(state protocol.State, focus Focus, mode keyboard.Mode, width, height int, err string) string {
	if len(state.Projects) == 0 {
		return emptyScreen(mode, width, height, err, state.Notice)
	}
	focus = Adjust(state, focus)
	project := state.Projects[focus.Project]
	if focus.Full && len(project.Cells) > 0 {
		return DrawFull(project, project.Cells[focus.Cell], mode, width, height, err)
	}

	d := Arrange(state, focus, width, height)
	if len(d.columns) == 0 {
		return emptyScreen(mode, width, height, err, state.Notice)
	}

	body := height - barHeight - 1
	blocks := make([][]string, len(d.columns))
	for i, c := range d.columns {
		blocks[i] = columnBlock(state, c, focus, mode)
	}

	lines := titleBar(mode, width, countCalls(state), state.Quota)
	for l := range body {
		var line strings.Builder
		for i, block := range blocks {
			if i > 0 {
				line.WriteString(" ")
			}
			if l < len(block) {
				line.WriteString(block[l])
				continue
			}
			line.WriteString(strings.Repeat(" ", d.columns[i].width))
		}
		lines = append(lines, line.String())
	}
	if d.hidden > 0 && len(lines) > 1 {
		lines[len(lines)-1] = markHidden(lines[len(lines)-1], d.hidden, width)
	}
	for len(lines) < height-1 {
		lines = append(lines, strings.Repeat(" ", width))
	}
	lines = lines[:height-1]
	return strings.Join(append(lines, footer(mode, width, err)), "\n")
}

// columnBlock draws a whole column: each project with its divider and its
// rows of cells, all at the column's width.
func columnBlock(state protocol.State, c column, focus Focus, mode keyboard.Mode) []string {
	var lines []string
	for _, r := range c.rows {
		project := state.Projects[r.project]
		if r.opens {
			lines = append(lines, projectDivider(project, c.width, mode, r.project == focus.Project))
		}
		lines = append(lines, cellRow(project, r, focus, mode, c.width)...)
	}
	return lines
}

// projectDivider is the line that separates one project from the next: the
// name, the path and the stack's state.
func projectDivider(project protocol.Project, width int, mode keyboard.Mode, focused bool) string {
	paintName := projectColor(project.Color).Bold(focused)
	paintDash := borderColor
	if focused {
		paintDash = activeGridColor
	}
	if !focused {
		paintName = faintColor
	}
	if mode == keyboard.Type {
		paintName, paintDash = faintColor, faintColor
	}

	mark := "──"
	if focused {
		mark = "━━"
	}
	left := mark + " " + strings.ToUpper(project.Name) + " "

	var signals []string
	calls := countProjectCalls(project)
	for _, state := range []string{"answered", "approve"} {
		if calls[state] > 0 {
			signals = append(signals, markerFor(state).symbol+strconv.Itoa(calls[state]))
		}
	}
	if project.HasCompose {
		docker := project.Docker
		if docker == "" {
			docker = "stopped"
		}
		signals = append(signals, "● "+docker)
	}
	right := ""
	if len(signals) > 0 {
		right = " " + strings.Join(signals, "  ") + " "
	}

	path := " " + shortenPath(project.Path, max(width/3, 10)) + " "
	if lipgloss.Width(left)+lipgloss.Width(path)+lipgloss.Width(right)+4 > width {
		path = ""
	}

	filler := width - lipgloss.Width(left) - lipgloss.Width(path) - lipgloss.Width(right)
	if filler < 0 {
		left = truncate(left, max(width-lipgloss.Width(right), 1))
		path, filler = "", max(width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	}

	return paintDash.Render(mark) + paintName.Render(strings.TrimPrefix(left, mark)) +
		faintColor.Render(path) +
		paintDash.Render(strings.Repeat("─", filler)) +
		paintDash.Render(right)
}

// cellRow draws one row: the cells side by side, each with its own box.
func cellRow(project protocol.Project, r row, focus Focus, mode keyboard.Mode, width int) []string {
	widths := splitWidth(len(r.cells), width)
	boxes := make([][]string, len(r.cells))
	for i, index := range r.cells {
		focused := r.project == focus.Project && index == focus.Cell
		boxes[i] = cellBox(project.Cells[index], mode, focused, widths[i], r.height)
	}

	lines := make([]string, r.height)
	for l := range lines {
		var line strings.Builder
		for i, box := range boxes {
			if l < len(box) {
				line.WriteString(box[l])
				continue
			}
			line.WriteString(strings.Repeat(" ", widths[i]))
		}
		lines[l] = line.String()
	}
	return lines
}

// markHidden reports, at the bottom of the grid, how many cells fell off
// screen. It hides the drawing, never the count.
func markHidden(line string, count, width int) string {
	notice := scrollColor.Render(" +" + strconv.Itoa(count) + " cell(s) off screen ")
	cut := max(width-lipgloss.Width(notice), 0)
	return truncate(line, cut) + notice
}

// cellBox draws one cell: name and marker on the top border, content
// inside. One border, one meaning — thick and green when it holds the
// keyboard.
func cellBox(item protocol.Cell, mode keyboard.Mode, focused bool, width, height int) []string {
	dash := [6]string{"┌", "┐", "└", "┘", "─", "│"}
	if focused && mode == keyboard.Type {
		dash = [6]string{"┏", "┓", "┗", "┛", "━", "┃"}
	}
	// An unfocused cell is dim end to end: content and border alike. The
	// structural color is reserved for the one holding the cursor.
	paintFrame := faintColor
	switch {
	case focused && mode == keyboard.Type:
		paintFrame = typingColor
	case focused:
		paintFrame = focusedBorderColor
	}

	marker := markerFor(item.State)
	label := " " + cellLabel(item) + " "
	state := " " + marker.symbol + " " + strings.ToUpper(item.State) + " "
	paintState := marker.badge()
	if !item.Live {
		state = " ▲ " + strconv.Itoa(item.Scroll) + " "
		paintState = chip(scrollColor)
	}

	inner := max(width-2, 1)
	filler := inner - lipgloss.Width(label) - lipgloss.Width(state)
	if filler < 0 {
		state, paintState = " "+marker.symbol+" ", marker.color()
		filler = inner - lipgloss.Width(label) - lipgloss.Width(state)
	}
	if filler < 0 {
		label = truncate(label, max(inner-lipgloss.Width(state), 1))
		filler = max(inner-lipgloss.Width(label)-lipgloss.Width(state), 0)
	}

	paintName := titleColor
	switch {
	case focused && mode == keyboard.Type:
		paintName = typingColor.Bold(true)
	case !focused:
		paintName = barColor
	}

	// Urgency is a filled area, not a hue: the cell blocking work becomes a
	// solid inverted bar across the whole header line. Only the sides stay
	// out of it, and the cell holding the keyboard is spared — there green
	// rules.
	paintTop := paintFrame
	if marker.blocks && item.Live && !(focused && mode == keyboard.Type) {
		bar := marker.color()
		paintTop, paintName, paintState = bar, bar, bar
	}

	lines := []string{
		paintTop.Render(dash[0]) +
			paintName.Render(label) +
			paintTop.Render(strings.Repeat(dash[4], filler)) +
			paintState.Render(state) +
			paintTop.Render(dash[1]),
	}
	// The side is the same on every line of the box: painting it once and
	// repeating costs far less than asking for the same drawing on every
	// line, and a cell's content has dozens of them.
	side := paintFrame.Render(dash[5])
	for i := range max(height-2, 0) {
		content := ""
		if i < len(item.Lines) {
			content = item.Lines[i]
			if !focused {
				content = dim(content)
			}
		}
		lines = append(lines, side+pad(content, inner)+side)
	}
	if height >= 2 {
		lines = append(lines, paintFrame.Render(dash[2]+strings.Repeat(dash[4], inner)+dash[3]))
	}
	return lines
}

// frame is the rectangle a cell occupies on screen. It's what navigation
// uses to know what's really above, below and beside — the arrow follows the
// drawing, not the list order.
type frame struct {
	project, cell       int
	x, y, width, height int
}

func (q frame) center() (int, int) {
	return q.x + q.width/2, q.y + q.height/2
}

// frames lists the rectangle of every visible cell, in project order.
func (d Layout) frames(state protocol.State) []frame {
	var all []frame
	for p, project := range state.Projects {
		for c, item := range project.Cells {
			origin, has := d.origins[item.ID]
			if !has {
				continue
			}
			inner := d.inners[item.ID]
			all = append(all, frame{
				project: p, cell: c,
				x: origin[0], y: origin[1],
				width: inner.Cols, height: inner.Rows,
			})
		}
	}
	return all
}

// Neighbor says where the arrow takes the focus: the cell that is literally
// in that direction on the grid. On a 2x2 grid, the down arrow drops a row
// instead of moving sideways. Returns false when there's nothing on that
// side — whoever calls it decides what to do then.
func Neighbor(state protocol.State, focus Focus, width, height, dx, dy int) (Focus, bool) {
	d := Arrange(state, focus, width, height)
	all := d.frames(state)
	var current frame
	foundCurrent := false
	for _, q := range all {
		if q.project == focus.Project && q.cell == focus.Cell {
			current, foundCurrent = q, true
		}
	}
	if !foundCurrent {
		return focus, false
	}

	ax, ay := current.center()
	best, found := Focus{}, false
	var bestAdvance, bestDeviation int
	for _, q := range all {
		if q.project == current.project && q.cell == current.cell {
			continue
		}
		// Only counts whoever shares the same row (horizontally) or the same
		// column (vertically). The arrow doesn't cut diagonally: a cell
		// above isn't a cell to the left.
		if !intersects(current, q, dx, dy) {
			continue
		}
		qx, qy := q.center()
		advance := (qx-ax)*dx + (qy-ay)*dy
		if advance <= 0 {
			continue
		}
		deviation := abs((qy-ay)*dx) + abs((qx-ax)*dy)
		if !found || advance < bestAdvance || (advance == bestAdvance && deviation < bestDeviation) {
			best, bestAdvance, bestDeviation, found = Focus{Project: q.project, Cell: q.cell}, advance, deviation, true
		}
	}
	return best, found
}

// intersects says whether two frames meet on the axis perpendicular to the
// arrow.
func intersects(a, b frame, dx, dy int) bool {
	if dx != 0 {
		return a.y < b.y+b.height && b.y < a.y+a.height
	}
	return a.x < b.x+b.width && b.x < a.x+a.width
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Adjust clamps the focus to what exists.
func Adjust(state protocol.State, focus Focus) Focus {
	if len(state.Projects) == 0 {
		return Focus{}
	}
	focus.Project = min(max(focus.Project, 0), len(state.Projects)-1)
	cells := len(state.Projects[focus.Project].Cells)
	if cells == 0 {
		focus.Cell = 0
		return focus
	}
	focus.Cell = min(max(focus.Cell, 0), cells-1)
	return focus
}

// OriginInGrid says at which row and column of the screen a cell's content
// begins. It's what puts the cursor in the right spot inside the box.
func OriginInGrid(state protocol.State, focus Focus, width, height int, id string) (int, int, bool) {
	origin, has := Arrange(state, focus, width, height).origins[id]
	return origin[0], origin[1], has
}
