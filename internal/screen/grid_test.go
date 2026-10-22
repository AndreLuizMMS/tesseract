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
		if !strings.Contains(drawing, project.Name) {
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

	for _, piece := range []string{"cortz-web", "fix nav", "posso mexer no Header?", "spec-m7.md"} {
		if !strings.Contains(withFocusOnFirst, piece) {
			t.Errorf("with focus on the first project, %q is still visible", piece)
		}
	}
	for _, piece := range []string{"doxar-api", "refatora auth", "testes"} {
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

// TestOneCellProjectsBecomeColumns — on a wide screen, three one-session
// projects split vertically: each with the screen's full height, instead of
// three strips stacked on top of each other.
func TestOneCellProjectsBecomeColumns(t *testing.T) {
	state := protocol.State{}
	for i := range 3 {
		state.Projects = append(state.Projects, protocol.Project{
			ID: "p" + strconv.Itoa(i), Name: "project" + strconv.Itoa(i), Path: "/dev/p",
			Cells: []protocol.Cell{{
				ID: "c" + strconv.Itoa(i), Type: "session", Name: "cel" + strconv.Itoa(i),
				State: "working", Live: true,
			}},
		})
	}

	d := Arrange(state, Focus{}, 190, 57)
	if len(d.columns) != 3 {
		t.Fatalf("three projects on a 190-column screen should become three columns: %d", len(d.columns))
	}
	for id, inner := range d.Inners() {
		if inner.Rows < 40 {
			t.Fatalf("cell %s should take up the column's full height: %#v", id, inner)
		}
		if inner.Cols < minCellWidth {
			t.Fatalf("cell %s got too narrow: %#v", id, inner)
		}
	}
}

// TestCellsStayCloseToSquare — a project splits its cells into whatever rows
// leave them closest to square: no cell becomes a strip, lying down or
// standing up.
func TestCellsStayCloseToSquare(t *testing.T) {
	state := protocol.State{Projects: []protocol.Project{{ID: "p1", Name: "regula-mais", Path: "/dev/rm"}}}
	for total := 2; total <= 9; total++ {
		state.Projects[0].Cells = nil
		for i := range total {
			state.Projects[0].Cells = append(state.Projects[0].Cells, protocol.Cell{
				ID: "c" + strconv.Itoa(i), Type: "session", Name: "session" + strconv.Itoa(i),
				State: "working", Live: true,
			})
		}
		for _, size := range [][2]int{{200, 57}, {200, 40}, {300, 40}} {
			for id, inner := range Arrange(state, Focus{}, size[0], size[1]).Inners() {
				if stretch(inner.Cols, inner.Rows) > 3 {
					t.Fatalf("%d cells on %dx%d: cell %s is a strip: %#v", total, size[0], size[1], id, inner)
				}
			}
		}
	}

	// Beside another project: each project keeps a side of the screen to
	// itself, and the one with two cells stacks them on its side, since side
	// by side they'd be two tall strips.
	state.Projects[0].Cells = state.Projects[0].Cells[:2]
	state.Projects = append(state.Projects, protocol.Project{ID: "p2", Name: "tesseract", Path: "/dev/ts",
		Cells: []protocol.Cell{{ID: "alone", Type: "session", Name: "session 1", State: "working", Live: true}}})
	d := Arrange(state, Focus{}, 200, 57)
	if len(d.columns) != 2 || len(d.columns[0].rows) != 2 || len(d.columns[1].rows) != 1 {
		t.Fatalf("two projects should be two columns, the first with two stacked cells: %#v", d.columns)
	}
	if down, found := Neighbor(state, Focus{}, 200, 57, 0, 1); !found || down.Cell != 1 {
		t.Fatalf("the down arrow should reach the cell underneath: %#v", down)
	}
}

// TestTabsShowUpInsteadOfType — a cell with tabs shows the tabs, with the
// active one highlighted.
func TestTabsShowUpInsteadOfType(t *testing.T) {
	state := testGrid()
	drawing := noStyle(Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Browse, 120, 30, ""))
	for _, tab := range []string{"claude", "cursor", "bash"} {
		if !strings.Contains(drawing, tab) {
			t.Errorf("tab %q should show up on the cell's border:\n%s", tab, drawing)
		}
	}
	if !strings.Contains(drawing, "refatora auth") {
		t.Error("the cell's name stays next to the tabs")
	}
}

// TestGridInTypeDimsTheRest — a cell holding the keyboard is said three
// ways that need no color: its own bottom line wears the mode's word, the
// status line is down to the one key that isn't the cell's, and its line —
// and only its line — is drawn heavy.
func TestGridInTypeDimsTheRest(t *testing.T) {
	state := testGrid()
	drawing := noStyle(Draw(state, Focus{Project: 0, Cell: 1}, keyboard.Type, 120, 30, ""))
	lines := strings.Split(drawing, "\n")
	if got := strings.Count(drawing, " TYPE "); got != 1 {
		t.Fatalf("the keyboard has one owner: the word shows up once, and it's there %d times", got)
	}
	if strings.ContainsAny(drawing, "╔║═┌┐") {
		t.Error("no box is drawn with a double or a square thin line")
	}
	if got := strings.Count(drawing, "┏"); got != 1 {
		t.Errorf("the heavy line is the owner's alone, and %d boxes wear it", got)
	}
	if browse := noStyle(Draw(state, Focus{Project: 0, Cell: 1}, keyboard.Browse, 120, 30, "")); strings.ContainsAny(browse, "┏┓┗┛━┃") {
		t.Error("with the keyboard on the application, every box is the thin line")
	}
	if last := strings.TrimSpace(lines[len(lines)-1]); !strings.HasPrefix(last, "ctrl+l") || strings.Contains(last, "?") {
		t.Errorf("in TYPE the status line names the one key that isn't the cell's: %q", last)
	}
	// The word is on the bottom line of the cell that owns the keyboard.
	for i, line := range lines {
		if strings.Contains(line, " TYPE ") {
			if !strings.Contains(strings.Join(lines[:i], "\n"), "testes") || !strings.HasPrefix(strings.TrimSpace(line), "┗") {
				t.Errorf("the word belongs on the owner's bottom line: %q", line)
			}
		}
	}
	if browse := noStyle(Draw(state, Focus{Project: 0, Cell: 1}, keyboard.Browse, 120, 30, "")); strings.Contains(browse, "TYPE") {
		t.Error("while the application has the keyboard no cell wears the word")
	}
}

// TestOnlyTheOwnerStaysLit — in TYPE the cell holding the keyboard keeps its
// project's hue and its surface, and every other cell loses both, whatever
// project it belongs to.
func TestOnlyTheOwnerStaysLit(t *testing.T) {
	inFullColor(t)
	state := testGrid()
	// No cell blocked: the bar is the one thing that never sinks.
	state.Projects[1].Cells[0].State = "working"
	typing := Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Type, 120, 30, "")
	browse := Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Browse, 120, 30, "")

	own := theme.ProjectHue(0)
	if !strings.Contains(typing, codeOf(own.Vivid, false)) {
		t.Error("the cell that holds the keyboard keeps its project's vivid line")
	}
	for color := 0; color < 3; color++ {
		hue := theme.ProjectHue(color)
		if !strings.Contains(browse, codeOf(hue.Dim, false)) {
			t.Errorf("in BROWSE project %d shows its hue", color)
		}
		if strings.Contains(typing, codeOf(hue.Dim, false)) {
			t.Errorf("in TYPE the cells without the keyboard lose their hue, and project %d kept it", color)
		}
	}
	if !strings.Contains(typing, codeOf(theme.LineDim, false)) {
		t.Error("in TYPE the other cells' lines sink to the dim line")
	}
}

// inFullColor pins the palette for the tests that read the color codes: the
// styles frozen when the package loaded already are whatever the environment
// decided, so those tests only say something when the environment had color.
func inFullColor(t *testing.T) {
	t.Helper()
	if theme.Current != theme.FullColor {
		t.Skip("the environment has no color")
	}
}

// codeOf returns the escape code a token turns into, as a foreground (38) or
// a background (48), so the tests never write a number by hand.
func codeOf(token string, background bool) string {
	style := theme.Paint(token, "")
	if background {
		style = theme.Paint("", token)
	}
	return strings.TrimSuffix(strings.SplitN(style.Render("x"), "x", 2)[0], "m")[2:]
}

// TestPhosphorIsWhereTheTypingLands — one cell per screen at most: the line
// and the word of the cell in TYPE, and nothing while the application has
// the keyboard. It is never a filled area, and neither is anything else but
// the approve bar.
func TestPhosphorIsWhereTheTypingLands(t *testing.T) {
	inFullColor(t)
	state := testGrid()
	// No cell blocked: with none, nothing on the screen may be filled in
	// amber.
	state.Projects[1].Cells[0].State = "working"
	amber := codeOf(theme.StateBlock, true)
	phosphor := codeOf(theme.BrandPhosphor, false)

	browse := Draw(state, Focus{}, keyboard.Browse, 120, 30, "")
	if strings.Contains(browse, amber) {
		t.Error("with no cell blocked, nothing may be filled in the approve color")
	}
	if strings.Contains(browse, phosphor) {
		t.Error("in BROWSE no cell holds the keyboard: there's no phosphor on the screen")
	}
	typing := Draw(state, Focus{}, keyboard.Type, 120, 30, "")
	if !strings.Contains(typing, phosphor) {
		t.Error("in TYPE the owner's line and word are phosphor")
	}
	for _, line := range strings.Split(typing, "\n") {
		// Every phosphor run on the screen is a piece of the owner's box.
		if strings.Contains(line, phosphor) && !strings.ContainsAny(noStyle(line), "┏┓┗┛┃") {
			t.Errorf("phosphor outside the cell that holds the keyboard: %q", noStyle(line))
		}
	}
	if strings.Contains(typing, codeOf(theme.BrandPhosphor, true)) {
		t.Error("the mode's word is a word, not a filled badge")
	}
	help := strings.Join(helpBox(120, 80, 0), "\n")
	if got := strings.Count(help, amber); got != 1 {
		t.Errorf("the help's legend shows the approve bar once, and nothing else is filled: %d", got)
	}
	if strings.Contains(help, phosphor) {
		t.Error("the help's headings are labels, and nothing is typed there")
	}
	// Cyan is the brand's own mark: it paints nothing on a working screen.
	for _, cyan := range []string{theme.Flux, theme.FluxCore} {
		if strings.Contains(browse+typing+help, codeOf(cyan, false)) {
			t.Errorf("%s is the brand mark's, and it painted the screen", cyan)
		}
	}
}

// TestApproveIsTheOnlySolidBar — urgency is filled area: the blocked cell's
// header is the one line filled in a state color, whatever the mode.
func TestApproveIsTheOnlySolidBar(t *testing.T) {
	inFullColor(t)
	before := theme.Dimmed
	theme.Dimmed = false
	defer func() { theme.Dimmed = before }()

	amber := codeOf(theme.StateBlock, true)
	for _, mode := range []keyboard.Mode{keyboard.Browse, keyboard.Type} {
		lines := strings.Split(Draw(testGrid(), Focus{}, mode, 120, 30, ""), "\n")
		filled := 0
		for _, line := range lines {
			if strings.Contains(line, amber) {
				filled++
				if !strings.Contains(noStyle(line), "fix nav") {
					t.Errorf("%s: a line other than the blocked cell's header is filled: %q", mode, noStyle(line))
				}
			}
		}
		if filled != 1 {
			t.Errorf("%s: the approve bar should fill exactly one line, and it fills %d", mode, filled)
		}
	}
	for _, other := range []string{theme.StateRead, theme.StateDead, theme.BrandPhosphor, theme.ProjectHue(0).Vivid} {
		if strings.Contains(Draw(testGrid(), Focus{}, keyboard.Browse, 120, 30, ""), codeOf(other, true)) {
			t.Errorf("only approve fills area, and %s became a background", other)
		}
	}
}

// TestFocusedCellSitsOnTheSurface — focus is a vivid line and a body one
// step above the screen: bg.surface under the focused cell, unbroken to the
// right border even when the agent resets its own colors mid-line, and no
// background at all under the others.
func TestFocusedCellSitsOnTheSurface(t *testing.T) {
	inFullColor(t)
	item := protocol.Cell{
		ID: "c1", Type: "bash", Name: "um", State: "working",
		Live: true, Lines: []string{"\x1b[31mred\x1b[0m plain"},
	}
	hue := theme.ProjectHue(0)
	surface := codeOf(theme.BgSurface, true)
	focused := cellBox(item, hue, keyboard.Browse, true, 30, 4)
	for _, line := range focused[1:3] {
		if !strings.Contains(line, surface) {
			t.Errorf("the focused body should sit on the surface: %q", line)
		}
	}
	// After the agent's own reset the surface has to come back, or the rest
	// of the line would fall through to the screen's background.
	if after := focused[1][strings.Index(focused[1], "red")+len("red"):]; !strings.Contains(after, surface) {
		t.Errorf("the surface didn't come back after the content's reset: %q", focused[1])
	}
	for name, box := range map[string][]string{
		"unfocused": cellBox(item, hue, keyboard.Browse, false, 30, 4),
		"silent":    cellBox(item, hue, keyboard.Type, false, 30, 4),
	} {
		if strings.Contains(strings.Join(box, ""), "\x1b[48;") {
			t.Errorf("a %s cell paints no background of its own", name)
		}
	}
	// The top line is the box's own edge: no band behind it.
	if strings.Contains(focused[0], "\x1b[48;") {
		t.Errorf("the top line has no raised band: %q", focused[0])
	}
}

// TestLineIsTheProjectsHue — a cell's line is its project's and nothing
// else's: dim while the keys act elsewhere, vivid and heavy on the cell they
// act on. A state's color never reaches the line, and a hue never reaches
// the state.
func TestLineIsTheProjectsHue(t *testing.T) {
	inFullColor(t)
	item := protocol.Cell{ID: "c1", Type: "bash", Name: "um", State: "answered", Live: true}
	mine, other := theme.ProjectHue(1), theme.ProjectHue(2)

	unfocused := cellBox(item, mine, keyboard.Browse, false, 30, 4)
	focused := cellBox(item, mine, keyboard.Browse, true, 30, 4)
	for i, line := range unfocused {
		if !strings.Contains(line, codeOf(mine.Dim, false)) || strings.Contains(line, codeOf(mine.Vivid, false)) {
			t.Errorf("line %d of an unfocused cell is the dim hue: %q", i, line)
		}
	}
	for i, line := range focused {
		if !strings.Contains(line, "\x1b[1;"+codeOf(mine.Vivid, false)) {
			t.Errorf("line %d of the focused cell is the vivid hue, heavy: %q", i, line)
		}
	}
	all := strings.Join(append(unfocused, focused...), "")
	for _, foreign := range []string{other.Vivid, other.Dim, theme.BrandPhosphor} {
		if strings.Contains(all, codeOf(foreign, false)) {
			t.Errorf("%s painted a cell that isn't its own", foreign)
		}
	}
	// The glyph is the state's color; the edges on either side of it aren't.
	glyph := strings.Index(focused[0], "⬤")
	if before := focused[0][:glyph]; !strings.HasSuffix(before, codeOf(theme.StateRead, false)+"m ") {
		t.Errorf("the state's glyph wears the state's color: %q", focused[0])
	}
	if strings.Contains(focused[1]+focused[2]+focused[3], codeOf(theme.StateRead, false)) {
		t.Error("a state's color never paints a line")
	}
}

// TestProjectDividerIsItsName — the name in the project's hue and nothing
// else: no number, no path, no rule, no count. The one thing that may sit
// beside it is a stack with a service that fell.
func TestProjectDividerIsItsName(t *testing.T) {
	project := testGrid().Projects[0] // compose, 4/5 up, one cell answered
	for _, focused := range []bool{true, false} {
		if got := noStyle(projectDivider(project, 60, keyboard.Browse, focused)); got != "  doxar-api" {
			t.Errorf("focused=%v: the divider is the project's name, got %q", focused, got)
		}
	}
	project.DockerDown = true
	if got := noStyle(projectDivider(project, 60, keyboard.Browse, false)); got != "  doxar-api  ✖ docker" {
		t.Errorf("a fallen service is said beside the name: %q", got)
	}
	if narrow := projectDivider(project, 14, keyboard.Browse, false); lipgloss.Width(narrow) != 14 || !strings.HasSuffix(noStyle(narrow), "✖ docker") {
		t.Errorf("narrow, the name gives way and the signal stays: %q", noStyle(narrow))
	}

	if theme.Current != theme.FullColor {
		return
	}
	hue := theme.ProjectHue(project.Color)
	if !strings.Contains(projectDivider(project, 60, keyboard.Browse, true), codeOf(hue.Vivid, false)) ||
		!strings.Contains(projectDivider(project, 60, keyboard.Browse, false), codeOf(hue.Dim, false)) {
		t.Error("the name wears the project's hue: vivid with the focus, dim without")
	}
	// While a cell holds the keyboard the other projects sink — the fallen
	// service doesn't.
	silent := projectDivider(project, 60, keyboard.Type, false)
	if strings.Contains(silent, codeOf(hue.Dim, false)) || !strings.Contains(silent, codeOf(theme.StateDead, false)) {
		t.Errorf("in TYPE the name sinks and the fallen service stays lit: %q", silent)
	}
}

// TestStateWordsLiveInTheHelp — a cell wears a glyph and no word, so the
// help is where each glyph is spelled out, the whole alphabet of it.
func TestStateWordsLiveInTheHelp(t *testing.T) {
	help := noStyle(strings.Join(helpLines(), "\n"))
	for _, state := range theme.States() {
		found := theme.Do(state)
		if found.Glyph == "" {
			t.Errorf("%s has no glyph to wear", state)
		}
		word := strings.ToLower(found.Label)
		if state == theme.Approve {
			word = found.Glyph + " " + found.Label
		}
		if !strings.Contains(help, found.Glyph) || !strings.Contains(help, word) {
			t.Errorf("the help doesn't spell out %s (%s)", state, found.Glyph)
		}
	}
	if len(theme.States()) != len(markers)-1 {
		t.Errorf("the legend lists %d states and the alphabet has %d with a glyph", len(theme.States()), len(markers)-1)
	}
}

// TestNarrowGridKeepsTheSignals — on a narrow screen the header shrinks to
// the glyph and the divider drops what it must, but nothing overflows and the
// state's shape survives.
func TestNarrowGridKeepsTheSignals(t *testing.T) {
	state := testGrid()
	for _, width := range []int{40, 52, 60, 72} {
		for _, mode := range []keyboard.Mode{keyboard.Browse, keyboard.Type} {
			drawing := Draw(state, Focus{}, mode, width, 24, "")
			for _, line := range strings.Split(drawing, "\n") {
				if visible := lipgloss.Width(noStyle(line)); visible > width {
					t.Fatalf("%d columns, %s: line with %d columns: %q", width, mode, visible, noStyle(line))
				}
			}
			clean := noStyle(drawing)
			for _, glyph := range []string{"⬤", "▸", "⏵", "○"} {
				// A cell that fell off screen is counted, not drawn.
				if !strings.Contains(clean, glyph) && !strings.Contains(clean, "off screen") {
					t.Errorf("%d columns, %s: the glyph %s vanished:\n%s", width, mode, glyph, clean)
				}
			}
			if got := strings.Count(clean, " TYPE "); (mode == keyboard.Type) != (got == 1) {
				t.Errorf("%d columns, %s: the mode's word shows up %d times:\n%s", width, mode, got, clean)
			}
		}
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
	if strings.Contains(drawing, "cortz-web") {
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
			if used > height-1 {
				t.Errorf("%dx%d: the column adds up to %d lines, the body has %d", width, height, used, height-1)
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
	if x < 1 || y < 1 {
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
	if focused := strings.Join(cellBox(item, theme.ProjectHue(0), keyboard.Browse, true, 30, 4), ""); !strings.Contains(focused, "\x1b[31m") {
		t.Fatal("the focused cell had to keep its content's color")
	}
	dimmed := strings.Join(cellBox(item, theme.ProjectHue(0), keyboard.Browse, false, 30, 4), "")
	if strings.Contains(dimmed, "\x1b[31m") {
		t.Fatal("the unfocused cell had to lose its content's color")
	}
	if !strings.Contains(history.StripCodes(dimmed), "vermelho") {
		t.Fatal("dimming the color can't eat the text")
	}
}

// TestModeWordEndsTheOwnersBottomLine — the word sits at the right end of
// the bottom line, where an agent's prompt is, and gives way whole on a cell
// too narrow for it rather than being cut.
func TestModeWordEndsTheOwnersBottomLine(t *testing.T) {
	item := protocol.Cell{ID: "c1", Type: "bash", Name: "um", State: "working", Live: true}
	hue := theme.ProjectHue(0)
	bottom := func(mode keyboard.Mode, focused bool, width int) string {
		box := cellBox(item, hue, mode, focused, width, 4)
		return noStyle(box[len(box)-1])
	}
	if got := bottom(keyboard.Type, true, 30); got != "┗"+strings.Repeat("━", 21)+" TYPE ━┛" {
		t.Errorf("the owner's bottom line ends in the word: %q", got)
	}
	plainLine := "╰" + strings.Repeat("─", 28) + "╯"
	if bottom(keyboard.Type, false, 30) != plainLine || bottom(keyboard.Browse, true, 30) != plainLine {
		t.Error("only the cell that holds the keyboard wears the word")
	}
	for width := 3; width <= 12; width++ {
		got := bottom(keyboard.Type, true, width)
		if lipgloss.Width(got) != width {
			t.Errorf("at %d columns the bottom line is %d wide: %q", width, lipgloss.Width(got), got)
		}
		if strings.Contains(got, "T") && !strings.Contains(got, " TYPE ") {
			t.Errorf("at %d columns the word was cut: %q", width, got)
		}
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

	d := Arrange(state, Focus{}, 200, 57)
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

	d := Arrange(state, Focus{}, 200, 10)
	if rows := d.allRows(); len(rows) != 1 || len(rows[0].cells) != 4 {
		t.Fatalf("with no height, the four cells end up in a single row: %#v", rows)
	}
	if d.hidden != 0 {
		t.Fatalf("no cell should fall off screen: %d", d.hidden)
	}
}

// TestFullScreenCursorOriginLandsOnTheFirstContentLine — in full screen the
// cursor was landing one line above the text.
func TestFullScreenCursorOriginLandsOnTheFirstContentLine(t *testing.T) {
	state := testGrid()
	focus := Focus{Project: 0, Cell: 0, Full: true}
	x, y, has := OriginInGrid(state, focus, 120, 30, "c1")
	if !has {
		t.Fatal("the full-screen cell needs an origin")
	}
	lines := strings.Split(noStyle(Draw(state, focus, keyboard.Browse, 120, 30, "")), "\n")
	if y >= len(lines) {
		t.Fatalf("the origin fell off screen: line %d of %d", y, len(lines))
	}
	if got := string([]rune(lines[y])[x:]); !strings.HasPrefix(got, "Moved the token check") {
		t.Fatalf("the origin doesn't sit on the first content line: %q", got)
	}
}

// TestStatusLineFillsAnyWidth — whatever is on it and however narrow the
// screen, the status line is exactly as wide as the screen.
func TestStatusLineFillsAnyWidth(t *testing.T) {
	state := testGrid()
	state.Quota = &protocol.Quota{Percent: 43, Rollover: "2:41"}
	blocked := &state.Projects[1].Cells[0]
	blocked.Ask, blocked.Detail = "tool", "Bash: pnpm migrate"
	for width := 4; width <= 160; width++ {
		for name, painted := range map[string]string{
			"rest":   footer(state, keyboard.Browse, width, ""),
			"type":   footer(state, keyboard.Type, width, ""),
			"error":  footer(state, keyboard.Browse, width, "Engine did not respond: broken pipe"),
			"offer":  footerFor(state, blocked, keyboard.Browse, width, "", 0),
			"hidden": footerFor(state, &state.Projects[0].Cells[0], keyboard.Browse, width, "", 3),
		} {
			if got := lipgloss.Width(painted); got != width {
				t.Fatalf("%s at %d columns is %d wide: %q", name, width, got, noStyle(painted))
			}
		}
	}
}

// TestEveryDialogCarriesItsOwnKeys — the status line lists no keys, so
// whatever holds the keyboard says, inside its own box, how to settle it and
// how to leave. The help is where every other key lives.
func TestEveryDialogCarriesItsOwnKeys(t *testing.T) {
	panel := NewDockerPanel("p1", "api")
	panel.Received(protocol.Services{Action: "list"})
	picker := NewIDEPicker("p1", "api", nil)
	picker.Received(protocol.IDEs{})
	for name, box := range map[string][]string{
		"form":         NewForm(nil, "/tmp").Draw(100),
		"question":     (&Question{Title: "SEARCH", Label: "TERM"}).Draw(100),
		"confirmation": killConfirmation(testGrid().Projects[0], testGrid().Projects[0].Cells[0]).Draw(100),
		"docker":       panel.Draw(100),
		"ide":          picker.Draw(100),
		"results":      resultsBox(protocol.Matches{Term: "x"}, 0, 100),
	} {
		drawn := noStyle(strings.Join(box, "\n"))
		if !strings.Contains(drawn, "esc") {
			t.Errorf("%s doesn't say how to leave it:\n%s", name, drawn)
		}
		if name != "results" && name != "docker" && !strings.Contains(drawn, "↵") {
			t.Errorf("%s doesn't say how to settle it:\n%s", name, drawn)
		}
		if !strings.HasPrefix(drawn, "╭") || !strings.Contains(drawn, "╰") {
			t.Errorf("%s isn't drawn with the screen's one line:\n%s", name, drawn)
		}
	}
	// What the old footer listed, the help lists.
	help := noStyle(strings.Join(helpLines(), "\n"))
	for _, key := range []string{"↑↓←→ cell", "tab · shift+tab", "↵", "v ", "n ", "d ", "?", "ctrl+l", "esc"} {
		if !strings.Contains(help, key) {
			t.Errorf("the help doesn't list %q", key)
		}
	}
}

// TestHelpScrollsToItsLastLine — the help is longer than most screens. Every
// line of it has to be reachable, down to the last key of the last mode, on
// a tall screen and on one that shows two lines at a time.
func TestHelpScrollsToItsLastLine(t *testing.T) {
	all := helpLines()
	last := noStyle(all[len(all)-1])
	for _, height := range []int{50, 24, 8} {
		seen := map[string]bool{}
		for scroll := 0; scroll <= helpLastScroll(height); scroll++ {
			box := helpBox(200, height, scroll)
			if len(box) > height {
				t.Fatalf("at height %d the help box is %d lines tall", height, len(box))
			}
			for _, line := range box {
				seen[strings.TrimSpace(strings.Trim(noStyle(line), "│"))] = true
			}
		}
		for _, line := range all {
			if text := strings.TrimSpace(noStyle(line)); text != "" && !seen[text] {
				t.Fatalf("at height %d the help never shows %q", height, text)
			}
		}
		if !seen[strings.TrimSpace(last)] {
			t.Fatalf("at height %d the last line is out of reach", height)
		}
	}
}

// TestOverlayClearsItsRows — a dialog takes the rows it sits on from edge to
// edge: a few columns of a cell left beside it are half a word and a border
// with no box. The rows above and below it stay as they were.
func TestOverlayClearsItsRows(t *testing.T) {
	background := strings.Join([]string{
		strings.Repeat("a", 40), "╭ claude  bash ─╮╭ cursor ──────────────╮", strings.Repeat("c", 40),
	}, "\n")
	lines := strings.Split(overlay(background, []string{"[box]"}, 40, 3), "\n")
	if lines[0] != strings.Repeat("a", 40) || lines[2] != strings.Repeat("c", 40) {
		t.Fatal("the lines outside the box were touched")
	}
	left := (40 - 5) / 2
	want := strings.Repeat(" ", left) + "[box]" + strings.Repeat(" ", 40-left-5)
	if lines[1] != want {
		t.Fatalf("the box's row should hold the box and nothing else:\n got %q\nwant %q", lines[1], want)
	}
}

// TestNarrowScreenKeepsWholeCells — when the projects fit neither stacked nor
// side by side, the width has the last word: a column is never narrower than
// a tight cell. What doesn't fit is counted on the status line instead of
// every project becoming a strip with its top line cut off.
func TestNarrowScreenKeepsWholeCells(t *testing.T) {
	var state protocol.State
	for i, name := range []string{"doxar-api", "cortz-web", "api-legado", "site", "infra", "oitavo", "x", "tesseract"} {
		state.Projects = append(state.Projects, protocol.Project{ID: name, Name: name, Color: i % 6, Cells: []protocol.Cell{{
			ID: "c" + name, Type: "session", Name: "session 1", State: "working", Live: true,
			Tabs: []string{"claude", "cursor", "bash", "md"}, Tab: "claude",
		}}})
	}
	for _, width := range []int{40, 60, 80} {
		for _, mode := range []keyboard.Mode{keyboard.Browse, keyboard.Type} {
			lines := strings.Split(noStyle(Draw(state, Focus{}, mode, width, 12, "")), "\n")
			for _, line := range lines {
				if widthOf(line) > width {
					t.Fatalf("at %d columns a line is %d wide: %q", width, widthOf(line), line)
				}
			}
			open, shut, corner := "╭", "╮", "─╯"
			if mode == keyboard.Type {
				open, shut, corner = "┏", "┓", "━┛"
			}
			if top := lines[1]; !strings.HasPrefix(top, open+" claude  session 1 ") || !(strings.HasSuffix(top, " ▸ ╮") || strings.HasSuffix(top, " ▸ "+shut)) {
				t.Errorf("at %d columns the cell's top line should be whole: %q", width, top)
			}
			if mode == keyboard.Type && !strings.Contains(lines[len(lines)-2], " TYPE "+corner) {
				t.Errorf("at %d columns the cell with the keyboard keeps its word: %q", width, lines[len(lines)-2])
			}
		}
		if status := noStyle(Draw(state, Focus{}, keyboard.Browse, width, 12, "")); !strings.Contains(status, "off screen") {
			t.Errorf("at %d columns the projects left out are counted", width)
		}
	}

	// A name that doesn't fit ends in an ellipsis instead of a cut word.
	long := protocol.Cell{ID: "c1", Type: "claude", Name: "refatora autenticacao do guard", State: "working", Live: true}
	if top := noStyle(cellBox(long, theme.ProjectHue(0), keyboard.Browse, false, 24, 4)[0]); !strings.Contains(top, "… ▸ ╮") {
		t.Errorf("a cut name should say it was cut: %q", top)
	}
}

// TestNeighborCellsDoNotTouch — two cells of one project side by side keep a
// column between their boxes, and the place the screen reports for the second
// one is where it's drawn.
func TestNeighborCellsDoNotTouch(t *testing.T) {
	cells := []protocol.Cell{
		{ID: "a", Type: "claude", Name: "one", State: "working", Live: true, Lines: []string{"first"}},
		{ID: "b", Type: "claude", Name: "two", State: "working", Live: true, Lines: []string{"second"}},
	}
	state := protocol.State{Projects: []protocol.Project{{ID: "p", Name: "p", Cells: cells}}}
	const width, height = 80, 12
	lines := strings.Split(noStyle(Draw(state, Focus{}, keyboard.Browse, width, height, "")), "\n")
	if strings.Contains(lines[1], "╮╭") || !strings.Contains(lines[1], "╮ ╭") {
		t.Fatalf("the two boxes should have a column between them: %q", lines[1])
	}
	if got := widthOf(lines[1]); got != width {
		t.Fatalf("the row should still fill the screen, got %d columns", got)
	}
	x, y, found := OriginInGrid(state, Focus{}, width, height, "b")
	if !found || !strings.HasPrefix(string([]rune(lines[y])[x:]), "second") {
		t.Fatalf("the second cell's content should be at the origin reported (%d,%d): %q", x, y, lines[y])
	}
}
