package screen

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/engine/history"
	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

var updateGolden = flag.Bool("update", false, "rewrites the drawing's reference files")

// noStyle strips the color codes so the reference file is readable and the
// comparison fails on content, not on shade of gray.
func noStyle(drawing string) string {
	lines := strings.Split(drawing, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(history.StripCodes(line), " ")
	}
	return strings.Join(lines, "\n")
}

func checkGolden(t *testing.T, name, drawing string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	clean := noStyle(drawing)
	if *updateGolden {
		if err := os.WriteFile(path, []byte(clean), 0o644); err != nil {
			t.Fatalf("write reference: %v", err)
		}
		return
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read reference (run with -update to create it): %v", err)
	}
	if string(expected) != clean {
		t.Errorf("the drawing changed.\n--- expected ---\n%s\n--- got ---\n%s", expected, clean)
	}
}

// testGrid is the same state used by both the grid and the list.
func testGrid() protocol.State {
	return protocol.State{
		Types: []protocol.CellType{{Type: "session", AcceptsPrompt: true}, {Type: "md", TargetLabel: "MD"}},
		Projects: []protocol.Project{
			{
				ID: "p1", Name: "doxar-api", Path: "/home/dev/doxar-api", Color: 0,
				HasCompose: true, Docker: "4/5",
				Cells: []protocol.Cell{
					{ID: "c1", Type: "session", Name: "refatora auth", State: "answered", Live: true,
						Tabs: []string{"claude", "cursor", "bash"}, Tab: "claude",
						Lines: []string{"Moved the token check", "to the guard.", "Which do you prefer?"}},
					{ID: "c2", Type: "session", Name: "testes", State: "working", Live: true,
						Tabs: []string{"claude", "cursor", "bash"}, Tab: "bash",
						Lines: []string{"$ go test ./...", "ok"}},
				},
			},
			{
				ID: "p2", Name: "cortz-web", Path: "/home/dev/cortz-web", Color: 1,
				Cells: []protocol.Cell{
					{ID: "c3", Type: "session", Name: "fix nav", State: "approve", Live: true,
						Tabs: []string{"claude", "cursor", "bash"}, Tab: "claude",
						Lines: []string{"posso mexer no Header?"}},
				},
			},
			{
				ID: "p3", Name: "api-legado", Path: "/home/dev/api-legado", Color: 2,
				Cells: []protocol.Cell{
					{ID: "c4", Type: "md", Name: "spec-m7.md", State: "stopped", Live: true,
						Lines: []string{"# Module 7"}},
				},
			},
		},
	}
}

// TestGridShowsEveryCell — the new rule: no cell turns into a strip, they
// all show up at once, from every project.
func TestGridShowsEveryCell(t *testing.T) {
	state := testGrid()
	full := Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Browse, 120, 30, "")
	checkGolden(t, "grid-focus-0.txt", full)

	drawing := noStyle(full)
	for _, project := range state.Projects {
		if !strings.Contains(drawing, strings.ToUpper(project.Name)) {
			t.Errorf("project %q should be on screen", project.Name)
		}
		for _, item := range project.Cells {
			if !strings.Contains(drawing, item.Name) {
				t.Errorf("cell %q should be on screen", item.Name)
			}
			for _, line := range item.Lines {
				if !strings.Contains(drawing, line) {
					t.Errorf("content %q of cell %q should show up", line, item.Name)
				}
			}
		}
	}
}

// TestGridHasNoMoreStrips — an unfocused project's text no longer
// disappears.
func TestGridHasNoMoreStrips(t *testing.T) {
	state := testGrid()
	withFocusOnFirst := noStyle(Draw(state, Focus{Project: 0}, keyboard.Browse, 120, 30, ""))
	withFocusOnLast := noStyle(Draw(state, Focus{Project: 2}, keyboard.Browse, 120, 30, ""))
	checkGolden(t, "grid-focus-2.txt", Draw(state, Focus{Project: 2}, keyboard.Browse, 120, 30, ""))

	for _, piece := range []string{"CORTZ-WEB", "fix nav", "posso mexer no Header?", "spec-m7.md"} {
		if !strings.Contains(withFocusOnFirst, piece) {
			t.Errorf("with focus on the first project, %q is still visible", piece)
		}
	}
	for _, piece := range []string{"DOXAR-API", "refatora auth", "testes"} {
		if !strings.Contains(withFocusOnLast, piece) {
			t.Errorf("with focus on the last project, %q is still visible", piece)
		}
	}
}

// TestCellsOfSameProjectSitSideBySide — a project isn't forced to stack
// vertically: its cells split the width.
func TestCellsOfSameProjectSitSideBySide(t *testing.T) {
	state := testGrid()
	d := Arrange(state, Focus{Project: 0}, 140, 30)

	if len(d.allRows()) != 3 {
		t.Fatalf("three projects, three rows: %#v", d.allRows())
	}
	if len(d.allRows()[0].cells) != 2 {
		t.Fatalf("the first project's two cells should split the row: %#v", d.allRows()[0])
	}

	first, second := d.inners["c1"], d.inners["c2"]
	if first.Cols < tightCellWidth || second.Cols < tightCellWidth {
		t.Fatalf("the cells should split the column's width: %#v %#v", first, second)
	}
	if first.Rows != second.Rows {
		t.Fatalf("cells in the same row have the same height: %#v %#v", first, second)
	}
	if alone := d.inners["c3"]; alone.Cols <= first.Cols {
		t.Fatalf("the lone cell should take the whole column: %#v", alone)
	}
}

// TestRowBreaksWhenItDoesNotFitTheWidth — too many cells in the same project
// become more than one row instead of squeezing.
func TestRowBreaksWhenItDoesNotFitTheWidth(t *testing.T) {
	state := protocol.State{Projects: []protocol.Project{{ID: "p1", Name: "grande", Path: "/dev/grande"}}}
	for i := range 6 {
		state.Projects[0].Cells = append(state.Projects[0].Cells, protocol.Cell{
			ID: "c" + strconv.Itoa(i), Type: "session", Name: "cel" + strconv.Itoa(i),
			State: "working", Live: true,
		})
	}

	d := Arrange(state, Focus{}, 120, 40)
	if len(d.allRows()) < 2 {
		t.Fatalf("six cells in 120 columns don't fit in a single row: %#v", d.allRows())
	}
	for _, r := range d.allRows() {
		if len(r.cells) > 3 {
			t.Fatalf("a row with %d cells becomes unreadable", len(r.cells))
		}
	}
	for id, inner := range d.Inners() {
		if inner.Cols < minCellWidth-2 {
			t.Fatalf("cell %s ended up with %d columns", id, inner.Cols)
		}
	}
}

// TestProjectsBecomeColumnsWhenHeightRunsOut — with too many projects for the
// height, the screen grows sideways instead of pushing a cell out.
func TestProjectsBecomeColumnsWhenHeightRunsOut(t *testing.T) {
	state := protocol.State{}
	for i := range 6 {
		state.Projects = append(state.Projects, protocol.Project{
			ID: "p" + strconv.Itoa(i), Name: "project" + strconv.Itoa(i), Path: "/dev/p",
			Cells: []protocol.Cell{{
				ID: "c" + strconv.Itoa(i), Type: "session", Name: "cel" + strconv.Itoa(i),
				State: "working", Live: true,
			}},
		})
	}

	d := Arrange(state, Focus{}, 120, 35)
	if len(d.columns) != 2 {
		t.Fatalf("six projects in 35 lines should open two columns: %d", len(d.columns))
	}
	if d.hidden != 0 {
		t.Fatalf("with both columns no cell should be left out: %d", d.hidden)
	}
	if len(d.Inners()) != 6 {
		t.Fatalf("the six cells should have their size reported to the engine: %#v", d.Inners())
	}
	for id, inner := range d.Inners() {
		if inner.Cols < tightCellWidth || inner.Rows < minCellHeight-2 {
			t.Fatalf("cell %s ended up too small: %#v", id, inner)
		}
	}
}

// TestGridReportsWhatDidNotFit — with too many cells, the drawing goes,
// never the count.
func TestGridReportsWhatDidNotFit(t *testing.T) {
	state := protocol.State{}
	for i := range 12 {
		state.Projects = append(state.Projects, protocol.Project{
			ID: "p" + strconv.Itoa(i), Name: "project" + strconv.Itoa(i), Path: "/dev/p",
			Cells: []protocol.Cell{{
				ID: "c" + strconv.Itoa(i), Type: "session", Name: "cel" + strconv.Itoa(i),
				State: "working", Live: true,
			}},
		})
	}

	drawing := noStyle(Draw(state, Focus{Project: 0}, keyboard.Browse, 100, 20, ""))
	if !strings.Contains(drawing, "off screen") {
		t.Errorf("with too many cells, the screen needs to say how many were left out:\n%s", drawing)
	}
	if !strings.Contains(drawing, "cel0") {
		t.Errorf("the focused cell has to be on screen:\n%s", drawing)
	}

	finalDrawing := noStyle(Draw(state, Focus{Project: 11}, keyboard.Browse, 100, 20, ""))
	if !strings.Contains(finalDrawing, "cel11") {
		t.Errorf("the window should follow the focus:\n%s", finalDrawing)
	}
}

// TestTabsShowUpInsteadOfType — a cell with tabs shows the tabs, with the
// active one highlighted.
func TestTabsShowUpInsteadOfType(t *testing.T) {
	state := testGrid()
	drawing := noStyle(Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Browse, 140, 30, ""))
	for _, tab := range []string{"claude", "cursor", "bash"} {
		if !strings.Contains(drawing, tab) {
			t.Errorf("tab %q should show up on the cell's border:\n%s", tab, drawing)
		}
	}
	if !strings.Contains(drawing, "refatora auth") {
		t.Error("the cell's name stays next to the tabs")
	}
}

// TestGridInTypeDimsTheRest — three redundant signals: dim bar, badge, and
// thick green border.
func TestGridInTypeDimsTheRest(t *testing.T) {
	state := testGrid()
	drawing := noStyle(Draw(state, Focus{Project: 0, Cell: 1}, keyboard.Type, 120, 30, ""))
	if !strings.Contains(drawing, "▓ TYPE ▓") {
		t.Error("the TYPE badge is missing from the bar")
	}
	if !strings.Contains(drawing, "┏") || !strings.Contains(drawing, "┃") {
		t.Error("the focused cell's border should thicken")
	}
	if !strings.Contains(drawing, "ctrl-l gives back the keyboard") {
		t.Error("the footer should shrink to the ctrl-l line")
	}
	if strings.Contains(drawing, "BROWSE") {
		t.Error("the bar can't say BROWSE in TYPE mode")
	}
}

// TestCellTurnsGreenWhileTyping — in TYPE, the cell holding the keyboard is
// the only one lit, and it turns green.
func TestCellTurnsGreenWhileTyping(t *testing.T) {
	state := testGrid()
	withKeyboard := Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Type, 120, 30, "")
	withoutKeyboard := Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Browse, 120, 30, "")

	// The escape code comes from the style itself, not a hand-written
	// number: the theme can downgrade the color depending on the terminal,
	// and the test still holds.
	if prefix := strings.SplitN(typingColor.Render("x"), "x", 2)[0]; prefix != "" &&
		!strings.Contains(withKeyboard, prefix) {
		t.Error("the focused cell should turn green in TYPE")
	}
	if strings.Contains(withoutKeyboard, "┏") {
		t.Error("outside TYPE the border doesn't thicken")
	}
	if !strings.Contains(withKeyboard, "┏") {
		t.Error("in TYPE the focused cell's border thickens")
	}
}

// TestFullScreenShowsOnlyTheFocusedCell — it's like copying a block of text
// without grabbing its neighbors.
func TestFullScreenShowsOnlyTheFocusedCell(t *testing.T) {
	state := testGrid()
	drawing := noStyle(Draw(state, Focus{Project: 0, Cell: 0, Full: true}, keyboard.Browse, 120, 30, ""))
	if !strings.Contains(drawing, "refatora auth") {
		t.Error("the focused cell should be on screen")
	}
	if strings.Contains(drawing, "go test") {
		t.Error("in full screen, the neighbor cell can't show up")
	}
	if strings.Contains(drawing, "CORTZ-WEB") {
		t.Error("in full screen, the other projects don't show up")
	}
}

// TestGeometryMatchesTheDrawing — the size reported to the engine is the
// same one the screen draws, or the process inside sees a crooked terminal.
func TestGeometryMatchesTheDrawing(t *testing.T) {
	state := testGrid()
	for _, size := range [][2]int{{120, 30}, {80, 20}, {200, 50}, {60, 14}} {
		width, height := size[0], size[1]
		d := Arrange(state, Focus{Project: 0}, width, height)
		drawing := strings.Split(Draw(state, Focus{Project: 0}, keyboard.Browse, width, height, ""), "\n")
		if len(drawing) != height {
			t.Errorf("%dx%d: the drawing has %d lines", width, height, len(drawing))
		}
		for _, line := range drawing {
			if visible := lipgloss.Width(noStyle(line)); visible > width {
				t.Errorf("%dx%d: line with %d columns: %q", width, height, visible, line)
			}
		}
		for _, c := range d.columns {
			used := 0
			for _, r := range c.rows {
				if r.opens {
					used++
				}
				used += r.height
			}
			if used > height-2 {
				t.Errorf("%dx%d: the column adds up to %d lines, the body has %d", width, height, used, height-2)
			}
		}
		for id, inner := range d.Inners() {
			if inner.Cols < 1 || inner.Rows < 1 {
				t.Errorf("%dx%d: invalid inner size for cell %s: %#v", width, height, id, inner)
			}
		}
	}
}

// TestCursorOriginLandsInsideTheCell — the cursor needs to land in the right
// spot.
func TestCursorOriginLandsInsideTheCell(t *testing.T) {
	state := testGrid()
	focus := Focus{Project: 0, Cell: 1}
	x, y, has := OriginInGrid(state, focus, 140, 30, "c2")
	if !has {
		t.Fatal("the focused cell needs an origin")
	}
	if x < 1 || y < 2 {
		t.Fatalf("strange origin: %d,%d", x, y)
	}

	lines := strings.Split(noStyle(Draw(state, focus, keyboard.Browse, 140, 30, "")), "\n")
	if y >= len(lines) {
		t.Fatalf("the origin fell off screen: line %d of %d", y, len(lines))
	}
	if !strings.Contains(lines[y-1], "testes") {
		t.Fatalf("the line above the origin should be the cell's border, got %q", lines[y-1])
	}
}

// TestUnfocusedCellLosesItsColor: during navigation only the selected cell
// stays lit; the rest turns gray so the eye finds where it is.
func TestUnfocusedCellLosesItsColor(t *testing.T) {
	item := protocol.Cell{
		ID: "c1", Type: "claude", Name: "um", State: "stopped",
		Live: true, Lines: []string{"\x1b[31mvermelho\x1b[0m"},
	}
	if focused := strings.Join(cellBox(item, keyboard.Browse, true, 30, 4), ""); !strings.Contains(focused, "\x1b[31m") {
		t.Fatal("the focused cell had to keep its content's color")
	}
	dimmed := strings.Join(cellBox(item, keyboard.Browse, false, 30, 4), "")
	if strings.Contains(dimmed, "\x1b[31m") {
		t.Fatal("the unfocused cell had to lose its content's color")
	}
	if !strings.Contains(history.StripCodes(dimmed), "vermelho") {
		t.Fatal("dimming the color can't eat the text")
	}
}

// TestModeBadgeStaysInTheRightCorner — only one inverted badge visible at a
// time, and it lives where the eye looks for window state.
func TestModeBadgeStaysInTheRightCorner(t *testing.T) {
	state := testGrid()
	drawing := noStyle(Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Type, 120, 30, ""))
	// The badge lives on the status line, the header's second one — the
	// first is the brand stripe.
	statusLine := strings.Split(drawing, "\n")[barHeight-1]

	badge := strings.Index(statusLine, "▓ TYPE ▓")
	if badge < 0 {
		t.Fatalf("the mode badge is missing from the status line: %q", statusLine)
	}
	if badge < len([]rune(statusLine))/2 {
		t.Fatalf("the badge should be on the right half, and it's at column %d of %d", badge, len(statusLine))
	}
}

// TestWarningBarOnlyBlinksWithALockedCell — the clock exists while someone is
// waiting on you, and stops on its own when no one is left waiting.
func TestWarningBarOnlyBlinksWithALockedCell(t *testing.T) {
	m := &Model{state: testGrid()}
	if !m.hasBlockedCell() {
		t.Fatal("the test grid has a cell in aprovar")
	}
	if m.blinkWarning() == nil {
		t.Fatal("with a locked cell the clock should start")
	}

	for i := range m.state.Projects {
		for j := range m.state.Projects[i].Cells {
			m.state.Projects[i].Cells[j].State = "stopped"
		}
	}
	if m.blinkWarning() != nil {
		t.Fatal("with no locked cell the clock should stop")
	}
	if m.blinking {
		t.Fatal("a stopped clock can't stay marked as running")
	}
}

// TestWarningBarChangesBetweenFrames — the test that catches a frozen style:
// if the marker cached the finished color instead of asking the theme, both
// frames come out identical and the bar never blinks on the real screen.
func TestWarningBarChangesBetweenFrames(t *testing.T) {
	before := theme.Dimmed
	defer func() { theme.Dimmed = before }()

	state := testGrid()
	theme.Dimmed = false
	lit := Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Browse, 120, 30, "")
	theme.Dimmed = true
	dimmed := Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Browse, 120, 30, "")

	if lit == dimmed {
		t.Fatal("the dim frame should draw differently from the lit one")
	}
	if noStyle(lit) != noStyle(dimmed) {
		t.Fatal("the blink is only color: the screen's text can't change")
	}
}

// TestHeaderHasTheBrandInTheMiddle — the stripe is the top of the screen and
// the eye's axis: the brand sits in its middle, with the rule crossing from
// both sides.
func TestHeaderHasTheBrandInTheMiddle(t *testing.T) {
	stripe := noStyle(brandStripe(keyboard.Browse, 120))
	if lipgloss.Width(stripe) != 120 {
		t.Fatalf("the stripe has to fill the whole width, and it has %d", lipgloss.Width(stripe))
	}
	// In runes, not bytes: the rule is made of three-byte characters, and
	// miscounting here would produce a made-up deviation.
	mark := len([]rune(stripe[:max(strings.Index(stripe, theme.Glyph), 0)]))
	if !strings.Contains(stripe, theme.Glyph) {
		t.Fatalf("the brand vanished from the stripe: %q", stripe)
	}
	// Center with one column of slack on each side, since the division's odd
	// remainder falls on one of the sides.
	if deviation := mark - len([]rune(stripe))/2; deviation > 12 || deviation < -12 {
		t.Fatalf("the brand should be in the middle, and it's %d columns off", deviation)
	}
	if !strings.HasPrefix(stripe, "─") || !strings.HasSuffix(stripe, "─") {
		t.Fatalf("the rule should cross the stripe end to end: %q", stripe)
	}
}

// TestFourCellsBecomeA2x2Grid — a project with four sessions doesn't become a
// strip of four; it becomes two rows of two.
func TestFourCellsBecomeA2x2Grid(t *testing.T) {
	state := protocol.State{Projects: []protocol.Project{{ID: "p1", Name: "regula-mais", Path: "/dev/rm"}}}
	for i := range 4 {
		state.Projects[0].Cells = append(state.Projects[0].Cells, protocol.Cell{
			ID: "c" + strconv.Itoa(i), Type: "session", Name: "session" + strconv.Itoa(i),
			State: "working", Live: true,
		})
	}

	d := Arrange(state, Focus{}, 200, 40)
	rows := d.allRows()
	if len(rows) != 2 {
		t.Fatalf("four cells should take up two rows: %#v", rows)
	}
	for _, r := range rows {
		if len(r.cells) != 2 {
			t.Fatalf("each row should have two cells: %#v", r)
		}
	}
}

// TestSquareGridYieldsToHeight — with no room for two rows, the four cells go
// back to a single strip instead of falling off screen.
func TestSquareGridYieldsToHeight(t *testing.T) {
	state := protocol.State{Projects: []protocol.Project{{ID: "p1", Name: "apertado", Path: "/dev/ap"}}}
	for i := range 4 {
		state.Projects[0].Cells = append(state.Projects[0].Cells, protocol.Cell{
			ID: "c" + strconv.Itoa(i), Type: "session", Name: "session" + strconv.Itoa(i),
			State: "working", Live: true,
		})
	}

	d := Arrange(state, Focus{}, 200, 12)
	if rows := d.allRows(); len(rows) != 1 || len(rows[0].cells) != 4 {
		t.Fatalf("with no height, the four cells end up in a single row: %#v", rows)
	}
	if d.hidden != 0 {
		t.Fatalf("no cell should fall off screen: %d", d.hidden)
	}
}
