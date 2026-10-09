package screen

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
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
	// contextWarnAt is how full a conversation's window gets, in percent,
	// before its cell says so.
	contextWarnAt = 80
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
		d.origins[item.ID] = [2]int{1, 1}
		return d
	}

	body := height - 1 // the status line
	d.columns = splitColumns(state, focus, width, body)

	for _, c := range d.columns {
		d.hidden += c.hidden
		y := 0
		for _, r := range c.rows {
			if r.opens {
				y++
			}
			project := state.Projects[r.project]
			x := c.x
			for i, cellWidth := range cellWidths(len(r.cells), c.width) {
				item := project.Cells[r.cells[i]]
				d.inners[item.ID] = Dims{
					Cols: max(cellWidth-2, 1),
					Rows: max(r.height-2, 1),
				}
				d.origins[item.ID] = [2]int{x + 1, y + 1}
				x += cellWidth + 1
			}
			y += r.height
		}
	}
	return d
}

// splitColumns splits the projects into columns and plans each one's rows.
// It's what makes the grid grow sideways before it grows downward.
func splitColumns(state protocol.State, focus Focus, width, body int) []column {
	how := columnCount(state.Projects, width, body)
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

// columnCount decides how many columns the screen splits into. The terminal
// is wide and short: stacking projects on top of each other leaves each cell
// three lines tall, so the screen splits vertically as long as the width
// backs up what each project wants. It takes the most columns the width can
// actually serve — three projects on a wide screen become three columns,
// each with the whole height — instead of a single width for the fullest
// project, which used to leave two projects sharing a column while a third
// had one to itself. Height still forces a column when the projects don't
// fit stacked.
func columnCount(projects []protocol.Project, width, body int) int {
	if len(projects) <= 1 {
		return 1
	}
	// The most columns the width can serve wins: a column all to itself is
	// what gives a project the screen's whole height. Splitting the projects
	// evenly by count is what used to leave two of them sharing a column,
	// half as tall, while a third had one to itself.
	for how := len(projects); how > 1; how-- {
		if columnsFit(projects, how, width) {
			return how
		}
	}
	// The height asks for a column when the projects don't fit stacked, but
	// the width has the last word: a column narrower than a tight cell is a
	// strip with no room for its own top line. Projects that fit neither way
	// stay off screen, counted on the status line, and the list has them.
	fitInHeight := max(body/minProjectHeight, 1)
	fitInWidth := max((width+1)/(tightCellWidth+1), 1)
	return min(max((len(projects)+fitInHeight-1)/fitInHeight, 1), len(projects), fitInWidth)
}

// columnsFit says whether splitting the projects into this many columns
// leaves every column wide enough for the projects that land in it: their
// fullest row, with every cell in it at least tight.
func columnsFit(projects []protocol.Project, how, width int) bool {
	perColumn := (len(projects) + how - 1) / how
	how = (len(projects) + perColumn - 1) / perColumn
	widths := splitWidth(how, width-(how-1))
	for i, start := 0, 0; start < len(projects); i, start = i+1, start+perColumn {
		want := minColumnWidth
		for _, project := range projects[start:min(start+perColumn, len(projects))] {
			want = max(want, widthProjectWants(project))
		}
		if i >= len(widths) || widths[i] < want {
			return false
		}
	}
	return true
}

// widthProjectWants is the tightest width that still serves a project's
// fullest row.
func widthProjectWants(project protocol.Project) int {
	total := len(project.Cells)
	if total == 0 {
		return 0
	}
	rows := squareRows(total)
	return ((total + rows - 1) / rows) * tightCellWidth
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
	// Everything fits at its shortest: nothing to choose. Counted row by row,
	// because only a project's first row spends a line on its name.
	need := 0
	for _, r := range rows {
		need += minCellHeight
		if r.opens {
			need++
		}
	}
	if need <= body {
		return rows
	}
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
		return emptyScreen(state, mode, width, height, err)
	}
	focus = Adjust(state, focus)
	project := state.Projects[focus.Project]
	if focus.Full && len(project.Cells) > 0 {
		return DrawFull(state, project, project.Cells[focus.Cell], mode, width, height, err)
	}

	d := Arrange(state, focus, width, height)
	if len(d.columns) == 0 {
		return emptyScreen(state, mode, width, height, err)
	}

	body := height - 1
	blocks := make([][]string, len(d.columns))
	for i, c := range d.columns {
		blocks[i] = columnBlock(state, c, focus, mode)
	}

	lines := make([]string, 0, height)
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
	for len(lines) < height-1 {
		lines = append(lines, strings.Repeat(" ", width))
	}
	lines = lines[:height-1]
	return strings.Join(append(lines, footerFor(state, &project.Cells[focus.Cell], mode, width, err, d.hidden)), "\n")
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

// projectDivider is the line that opens a project: its name, in its hue,
// and nothing else. No rule runs from it — the boxes under it already say
// where the project ends. It stands over its cells' labels, vivid and heavy
// on the project the keys act on.
//
// The one thing that may sit beside the name is a stack with a service that
// fell: it's there only while it's true, in the dead state's glyph and color,
// and it stays lit even while a cell has the keyboard.
func projectDivider(project protocol.Project, width int, mode keyboard.Mode, focused bool) string {
	paintName := projectHue(project).Line(focused)
	if mode == keyboard.Type && !focused {
		paintName = faintColor
	}
	fallen := ""
	if project.DockerDown {
		fallen = "  " + errorColor.Render(theme.Do(theme.Down).Glyph+" docker")
	}
	name := truncate(project.Name, max(width-2-lipgloss.Width(fallen), 1))
	return pad("  "+paintName.Render(name)+fallen, width)
}

// projectHue is the hue the project was given when it was created.
func projectHue(project protocol.Project) theme.Hue { return theme.ProjectHue(project.Color) }

// cellWidths splits a row's width among its cells, leaving one column of
// breathing room between neighbors: two boxes of the same project touching
// (`╮╭`) read as one box with a seam.
func cellWidths(cells, width int) []int {
	return splitWidth(cells, width-(cells-1))
}

// cellRow draws one row: the cells side by side, each with its own box.
func cellRow(project protocol.Project, r row, focus Focus, mode keyboard.Mode, width int) []string {
	widths := cellWidths(len(r.cells), width)
	boxes := make([][]string, len(r.cells))
	for i, index := range r.cells {
		focused := r.project == focus.Project && index == focus.Cell
		boxes[i] = cellBox(project.Cells[index], projectHue(project), mode, focused, widths[i], r.height)
	}

	lines := make([]string, r.height)
	for l := range lines {
		var line strings.Builder
		for i, box := range boxes {
			if i > 0 {
				line.WriteString(" ")
			}
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

// cellBox draws one cell: its tabs, its name and its state on the top line,
// its content inside.
//
// The line is the project's: every cell of a project wears its hue, dim, and
// the one the keys act on wears it vivid, over a body one step above the
// screen. Once that cell holds the keyboard its line turns heavy and phosphor
// and gains the mode's word on its bottom line, and every other cell sinks
// into the dark, hue and all.
func cellBox(item protocol.Cell, hue theme.Hue, mode keyboard.Mode, focused bool, width, height int) []string {
	owns := focused && mode == keyboard.Type
	// silent is every cell but the owner while a cell holds the keyboard.
	silent := mode == keyboard.Type && !focused

	dash := theme.Frame
	paintLine := hue.Line(focused)
	paintName, paintTabOn, paintTabOff := mutedColor, mutedColor.Bold(true), faintColor
	switch {
	case owns:
		// The cell being written in: heavy stroke, in the phosphor.
		dash, paintLine = theme.FrameHeavy, typingColor
		paintName, paintTabOn, paintTabOff = titleColor, barColor.Bold(true), mutedColor
	case focused:
		paintName, paintTabOn, paintTabOff = titleColor, barColor.Bold(true), mutedColor
	case silent:
		paintLine = dimLineColor
		paintName, paintTabOn, paintTabOff = faintColor, faintColor.Bold(true), faintColor
	}

	marker := markerFor(item.State)
	// The state is its glyph, and the glyph is the last thing on the line:
	// the same column on every cell, to be read down the screen. An idle
	// agent has none. The answer in one line, when the agent wrote one, sits
	// before it while there's room.
	paintState, paintNote := marker.tone().Bold(true), mutedColor
	glyph, note := "", ""
	if marker.symbol != "" {
		glyph = " " + marker.symbol + " "
		if item.State == "answered" {
			note = item.Detail
		}
	}
	if !item.Live {
		// Reading the history isn't a state: it's a position.
		glyph, note = " ▲ "+strconv.Itoa(item.Scroll)+" ", ""
		paintState = keyColor
	}
	// While another cell holds the keyboard every state goes quiet with the
	// rest of the screen — except the one that blocks, which is a solid bar
	// and can't wait for you to come back.
	if silent {
		paintState, paintNote = faintColor, faintColor
	}

	// Urgency is a filled area, not a hue: the cell blocking work becomes a
	// solid bar across the whole top line, written out, and only the sides
	// and the bottom keep the project's hue. It's the one state with a word.
	paintTop, paintWarn := paintLine, warnColor.Bold(true)
	if silent {
		// The window's warning sinks with the rest: nobody can act on it
		// from a cell that doesn't hold the keyboard.
		paintWarn = faintColor
	}
	if marker.blocks && item.Live {
		bar := marker.color()
		paintTop, paintState, paintNote, paintWarn = bar, bar, bar, bar
		paintName, paintTabOn, paintTabOff = bar, bar, bar
		glyph = " " + marker.symbol + " " + marker.label + " "
	}
	label := cellLabel(item, paintName, paintTabOn, paintTabOff)

	inner := max(width-2, 1)
	fits := func() bool { return lipgloss.Width(label)+lipgloss.Width(glyph) <= inner }
	if !fits() && len(item.Tabs) > 1 {
		// Too narrow for every tab: the name outlives the tabs that aren't
		// showing, and the one that is stays.
		alone := item
		alone.Tabs = []string{item.Tab}
		label = cellLabel(alone, paintName, paintTabOn, paintTabOff)
	}
	if !fits() && marker.blocks && item.Live {
		// Too narrow for the word: the bar is still the bar.
		glyph = " " + marker.symbol + " "
	}
	filler := inner - lipgloss.Width(label) - lipgloss.Width(glyph)
	if filler < 0 {
		// A name cut short says it was: the last column becomes an ellipsis.
		room := max(inner-lipgloss.Width(glyph), 1)
		label = truncate(label, room)
		if room >= 2 {
			label = truncate(label, room-1) + paintName.Render("…")
		}
		filler = max(inner-lipgloss.Width(label)-lipgloss.Width(glyph), 0)
	}
	// A conversation about to run out of window is the one figure a cell
	// carries on its own top line, and only then: below the line it's nothing
	// anyone has to act on.
	warning := ""
	if text := " ctx " + strconv.Itoa(item.Context) + "%"; item.Context >= contextWarnAt && filler >= lipgloss.Width(text)+1 {
		warning = text
		filler -= lipgloss.Width(text)
	}
	if note != "" {
		// One dash of the line always stays between the name and the answer.
		if room := filler - 2; room >= 12 {
			note = " " + shorten(note, room)
			filler -= lipgloss.Width(note)
		} else {
			note = ""
		}
	}

	lines := []string{
		paintTop.Render(dash[0]) +
			label +
			paintTop.Render(strings.Repeat(dash[4], filler)) +
			paintNote.Render(note) +
			paintWarn.Render(warning) +
			paintState.Render(glyph) +
			paintTop.Render(dash[1]),
	}
	// The side is the same on every line of the box: painting it once and
	// repeating costs far less than asking for the same drawing on every
	// line, and a cell's content has dozens of them.
	side := paintLine.Render(dash[5])
	for i := range max(height-2, 0) {
		content := ""
		if i < len(item.Lines) {
			content = item.Lines[i]
		}
		switch {
		case focused:
			// The focused cell keeps what the agent painted; the surface goes
			// underneath it.
			content = surfaceInk.Under(pad(content, inner))
		case silent:
			content = pad(faintInk.Render(plain(content)), inner)
		default:
			content = mutedInk.Render(pad(plain(content), inner))
		}
		lines = append(lines, side+content+side)
	}
	if height >= 2 {
		// The cell that holds the keyboard says so in a word, at the end of
		// its bottom line: the one place the phosphor shows up, and the shape
		// that still says it with no color.
		tag := ""
		if owns && inner >= len(theme.TypeTag)+3 {
			tag = " " + theme.TypeTag + " "
		}
		rule := inner - len(tag)
		tail := 0
		if tag != "" {
			tail = 1
		}
		bottom := paintLine.Render(dash[2] + strings.Repeat(dash[4], rule-tail))
		if tag != "" {
			bottom += typingColor.Render(tag)
		}
		lines = append(lines, bottom+paintLine.Render(strings.Repeat(dash[4], tail)+dash[3]))
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
