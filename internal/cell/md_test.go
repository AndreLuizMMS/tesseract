package cell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"

	"github.com/andreluiz/tesseract/internal/engine/history"
)

// cleanScreenOf is the cell's screen without color codes — rendered markdown
// comes full of them, and what the test cares about is the text.
func cleanScreenOf(c Cell) string {
	var cleaned []string
	for _, line := range c.Draw().Lines {
		cleaned = append(cleaned, history.StripCodes(line))
	}
	return strings.Join(cleaned, "\n")
}

// projectWithDocuments sets up a project with markdown scattered around,
// including in a folder that shouldn't be scanned.
func projectWithDocuments(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"README.md":                   "# Leia\n\nbeginning of everything\n",
		"docs/spec-m7.md":             "# Module 7\n\nclinical records\n",
		"docs/spec-m8.md":             "# Module 8\n\nschedule\n",
		"docs/guias/como-rodar.md":    "# How to run\n\npnpm dev\n",
		"node_modules/pacote/LEIA.md": "# doesn't count\n",
		"src/codigo.go":               "package main\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("prepare: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("prepare: %v", err)
		}
	}
	return dir
}

func markdownTab(t *testing.T, dir string) Cell {
	t.Helper()
	item, err := New("md")
	if err != nil {
		t.Fatalf("manufacture: %v", err)
	}
	if err := item.Spawn(Config{ID: "c1", Directory: dir, Columns: 70, Lines: 20}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	t.Cleanup(func() { item.Kill() })
	return item
}

func typeIntoSearch(c Cell, text string) {
	for _, letter := range text {
		c.Key(Keystroke{Code: letter, Text: string(letter)})
	}
}

// TestMdListsTheProjectsMarkdownFiles — the tab opens into a list of
// everything markdown in there, with the search box on top.
func TestMdListsTheProjectsMarkdownFiles(t *testing.T) {
	item := markdownTab(t, projectWithDocuments(t))
	screen := cleanScreenOf(item)

	if !strings.Contains(screen, "search:") {
		t.Errorf("the search bar should be at the top:\n%s", screen)
	}
	for _, file := range []string{"README.md", "docs/spec-m7.md", "docs/spec-m8.md", "docs/guias/como-rodar.md"} {
		if !strings.Contains(screen, file) {
			t.Errorf("the list should show %q:\n%s", file, screen)
		}
	}
	if strings.Contains(screen, "node_modules") {
		t.Errorf("node_modules doesn't enter the scan:\n%s", screen)
	}
	if strings.Contains(screen, "codigo.go") {
		t.Errorf("the list is markdown only:\n%s", screen)
	}
}

// TestMdSearchFiltersByName — typing filters the list.
func TestMdSearchFiltersByName(t *testing.T) {
	item := markdownTab(t, projectWithDocuments(t))

	typeIntoSearch(item, "m7")
	screen := cleanScreenOf(item)
	if !strings.Contains(screen, "spec-m7.md") {
		t.Errorf("the search should find the file:\n%s", screen)
	}
	if strings.Contains(screen, "spec-m8.md") || strings.Contains(screen, "README.md") {
		t.Errorf("the search should hide what doesn't match:\n%s", screen)
	}
	if !strings.Contains(screen, "1 of 4 documents") {
		t.Errorf("the list should count what matched:\n%s", screen)
	}

	// Backspacing brings everything back.
	item.Key(Keystroke{Code: vt.KeyBackspace})
	item.Key(Keystroke{Code: vt.KeyBackspace})
	if screen := cleanScreenOf(item); !strings.Contains(screen, "README.md") {
		t.Errorf("clearing the search should bring back the whole list:\n%s", screen)
	}
}

// TestMdEnterOpensTheChosenFile — this is the gesture the tab exists for.
func TestMdEnterOpensTheChosenFile(t *testing.T) {
	item := markdownTab(t, projectWithDocuments(t))

	typeIntoSearch(item, "como-rodar")
	item.Key(Keystroke{Code: vt.KeyEnter})

	screen := cleanScreenOf(item)
	// The title becomes uppercase on the page, like a chapter.
	if !strings.Contains(screen, "HOW TO RUN") || !strings.Contains(screen, "pnpm dev") {
		t.Errorf("the file should be open and rendered:\n%s", screen)
	}
	if !strings.Contains(screen, "esc back to list") {
		t.Errorf("the reading view should say how to go back:\n%s", screen)
	}

	// And esc goes back to the list, with the search still up.
	item.Key(Keystroke{Code: vt.KeyEscape})
	screen = cleanScreenOf(item)
	if !strings.Contains(screen, "search:") || !strings.Contains(screen, "como-rodar.md") {
		t.Errorf("esc should go back to the list:\n%s", screen)
	}
}

// TestMdArrowsMoveThroughTheList — the selection moves before opening.
func TestMdArrowsMoveThroughTheList(t *testing.T) {
	item := markdownTab(t, projectWithDocuments(t))

	typeIntoSearch(item, "spec")
	item.Key(Keystroke{Code: vt.KeyDown})
	item.Key(Keystroke{Code: vt.KeyEnter})

	if screen := cleanScreenOf(item); !strings.Contains(screen, "MODULE 8") {
		t.Errorf("the arrow should have moved the selection to the second one:\n%s", screen)
	}
}

// TestMdOpensDirectlyIntoTheRequestedFile — created pointing at a file, the
// tab is born already in it.
func TestMdOpensDirectlyIntoTheRequestedFile(t *testing.T) {
	dir := projectWithDocuments(t)
	item, err := New("md")
	if err != nil {
		t.Fatalf("manufacture: %v", err)
	}
	if err := item.Spawn(Config{ID: "c1", Directory: dir, Target: "docs/spec-m7.md", Columns: 70, Lines: 20}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer item.Kill()

	if screen := cleanScreenOf(item); !strings.Contains(screen, "clinical records") {
		t.Errorf("the tab should open straight into the requested file:\n%s", screen)
	}
}

// TestMdReloadsWhenTheDiskChanges — the agent edits the file, and the
// markdown next to it updates on its own.
func TestMdReloadsWhenTheDiskChanges(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(file, []byte("# Before\n\nfirst version\n"), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	item, _ := New("md")
	notifications := make(chan struct{}, 32)
	if err := item.Spawn(Config{
		ID: "c1", Directory: dir, Target: file, Columns: 60, Lines: 20,
		Notify: func() {
			select {
			case notifications <- struct{}{}:
			default:
			}
		},
	}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer item.Kill()

	if !strings.Contains(cleanScreenOf(item), "first version") {
		t.Fatalf("the first version didn't show up:\n%s", cleanScreenOf(item))
	}

	if err := os.WriteFile(file, []byte("# After\n\nsecond version\n"), 0o644); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if !waitFor(t, time.Second, func() bool {
		return strings.Contains(cleanScreenOf(item), "second version")
	}) {
		t.Fatalf("the file changed and the cell didn't follow within 1s:\n%s", cleanScreenOf(item))
	}
	select {
	case <-notifications:
	default:
		t.Error("the cell changed without notifying the screen")
	}
}

// TestMdWithDeletedFileDoesNotPanic — it vanishes from disk, becomes a
// readable error.
func TestMdWithDeletedFileDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "some.md")
	if err := os.WriteFile(file, []byte("# Exists\n"), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	item, _ := New("md")
	if err := item.Spawn(Config{ID: "c1", Directory: dir, Target: file, Columns: 60, Lines: 20}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer item.Kill()

	if err := os.Remove(file); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !waitFor(t, 2*time.Second, func() bool { return item.State() == Crashed }) {
		t.Fatalf("a deleted file should turn into an error state, it's %q", item.State())
	}
	if screen := cleanScreenOf(item); !strings.Contains(screen, "gone from disk") {
		t.Fatalf("the error needs to be readable, got:\n%s", screen)
	}
}

// TestMdSpawnsWithoutATarget — the tab doesn't need a file to exist.
func TestMdSpawnsWithoutATarget(t *testing.T) {
	item, _ := New("md")
	if err := item.Spawn(Config{ID: "c1", Directory: t.TempDir(), Columns: 60, Lines: 20}); err != nil {
		t.Fatalf("the markdown tab spawns without a target: %v", err)
	}
	defer item.Kill()
	if screen := cleanScreenOf(item); !strings.Contains(screen, "0 of 0 documents") {
		t.Errorf("a project without markdown shows the empty list:\n%s", screen)
	}
}

// TestMdScrollsThroughTheText — a large file scrolls and goes back to the
// start.
func TestMdScrollsThroughTheText(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "grande.md")
	var text strings.Builder
	text.WriteString("# Grande\n\n")
	for i := range 200 {
		text.WriteString("line number " + string(rune('a'+i%26)) + "\n\n")
	}
	if err := os.WriteFile(file, []byte(text.String()), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	item, _ := New("md")
	if err := item.Spawn(Config{ID: "c1", Directory: dir, Target: file, Columns: 60, Lines: 10}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer item.Kill()

	top := cleanScreenOf(item)
	item.Scroll(-20, false)
	scrolled := item.Draw()
	if scrolled.Live {
		t.Fatal("after scrolling, the reading isn't at the start")
	}
	if strings.Join(scrolled.Lines, "\n") == top {
		t.Fatal("scrolling didn't change what's on screen")
	}
	item.Scroll(0, true)
	if cleanScreenOf(item) != top {
		t.Fatal("going back to live should bring back the start of the file")
	}
}

// TestPageKeepsAReadableMeasure — on a wide cell the text doesn't stretch
// to the edges: it stays at book width, centered, with the leftover space
// turned into margin. A long line the eye can't track back is the thing
// that makes reading in the terminal worse than in the IDE.
func TestPageKeepsAReadableMeasure(t *testing.T) {
	text := "# Title\n\n" + strings.Repeat("word ", 200) + "\n"
	const columns = 160
	lines := renderPage(text, columns)

	longest := 0
	leftEdge := columns
	for _, line := range lines {
		clean := history.StripCodes(line)
		if strings.TrimSpace(clean) == "" {
			continue
		}
		if length := len([]rune(strings.TrimRight(clean, " "))); length > longest {
			longest = length
		}
		if margin := len(clean) - len(strings.TrimLeft(clean, " ")); margin < leftEdge {
			leftEdge = margin
		}
	}
	if longest > columns {
		t.Fatalf("the page went past the cell's width: %d columns", longest)
	}
	if measure := longest - leftEdge; measure > maxPageWidth {
		t.Fatalf("the measure went past the readable width: %d columns", measure)
	}
	if leftEdge < (columns-maxPageWidth)/2 {
		t.Fatalf("the page should be centered, started at column %d", leftEdge)
	}
}

// TestNarrowPageUsesEverythingItHas — the cap only bites on a wide cell; a
// narrow one still fills what it has, minus the margin.
func TestNarrowPageUsesEverythingItHas(t *testing.T) {
	text := strings.Repeat("word ", 200) + "\n"
	const columns = 60
	lines := renderPage(text, columns)

	longest := 0
	for _, line := range lines {
		clean := history.StripCodes(line)
		if length := len([]rune(strings.TrimRight(clean, " "))); length > longest {
			longest = length
		}
	}
	if longest > columns {
		t.Fatalf("the page went past the cell's width: %d columns", longest)
	}
	// Word wrap always leaves a chunk of the last word that didn't fit.
	if minimum := columns - 2*pageMargin - 10; longest < minimum {
		t.Fatalf("the page should fill the narrow cell, came out with %d of %d columns", longest, columns)
	}
}

// TestPageDoesNotWrapWideCode — a diagram or a terminal screen gets
// truncated, not scrambled across several lines.
func TestPageDoesNotWrapWideCode(t *testing.T) {
	wide := strings.Repeat("─", 200)
	text := "# Doc\n\n```\n" + wide + "\n```\n"
	lines := renderPage(text, 80)

	for _, line := range lines {
		clean := strings.TrimRight(history.StripCodes(line), " ")
		if len([]rune(clean)) > 80 {
			t.Fatalf("line wider than the cell: %d columns", len([]rune(clean)))
		}
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "›") {
		t.Fatalf("the code truncation should be marked:\n%s", joined)
	}
	withCode := 0
	for _, line := range lines {
		if strings.Contains(history.StripCodes(line), "──") {
			withCode++
		}
	}
	if withCode > 1 {
		t.Fatalf("the code line was wrapped into %d lines instead of truncated:\n%s", withCode, joined)
	}
}

// TestPageDrawsTheTitleAsAHeading — the H1 becomes a heading the way the
// brand writes one, not a "#" — and not a filled bar: inside the terminal the
// only solid area is the approve bar.
func TestPageDrawsTheTitleAsAHeading(t *testing.T) {
	lines := renderPage("# Module 7\n\ntext\n", 100)
	joined := strings.Join(lines, "\n")
	if strings.Contains(history.StripCodes(joined), "# Module") {
		t.Fatalf("the hash can't survive onto the page:\n%s", joined)
	}
	if !strings.Contains(history.StripCodes(joined), "// MODULE 7") {
		t.Fatalf("the title should become an uppercase heading:\n%s", joined)
	}
	// "48;" covers the background in 256 colors and in 24-bit: what matters
	// is that no heading paints one, at whatever depth the terminal writes.
	for _, line := range lines {
		if strings.Contains(history.StripCodes(line), "MODULE 7") && strings.Contains(line, "48;") {
			t.Fatalf("the title can't be a filled bar:\n%q", line)
		}
	}
}

// TestCodeBlockIsHighlightedInThePalette — syntax highlighting writes the
// palette's own tokens, not the nearest of 256 colors.
func TestCodeBlockIsHighlightedInThePalette(t *testing.T) {
	joined := strings.Join(renderPage("```go\nfunc main() { fmt.Println(\"hi\") }\n```\n", 100), "\n")
	if strings.Contains(joined, "38;5;") || strings.Contains(joined, "48;5;") {
		t.Fatalf("the code box wrote an indexed color instead of a token:\n%q", joined)
	}
	// 34;224;208 is flux, the keyword's token, written out in full.
	if !strings.Contains(joined, "38;2;34;224;208") {
		t.Fatalf("the keyword should come out in its 24-bit token:\n%q", joined)
	}
	// Text and structure only: no state color tints the code, and no green.
	for name, token := range map[string]string{
		"state.orphan": "199;125;255", "state.read": "125;183;232", "state.block": "255;180;84",
		"state.dead": "255;59;71", "brand.core": "31;122;76", "brand.phosphor": "85;255;166",
	} {
		if strings.Contains(joined, "38;2;"+token) {
			t.Fatalf("the code box used %s, which isn't text nor structure:\n%q", name, joined)
		}
	}
}

// TestSearchCaretIsTheRealCursor — the list's search field has no drawn
// caret: the cursor is the terminal's, right after what was typed, and only
// the cell that holds the keyboard shows it.
func TestSearchCaretIsTheRealCursor(t *testing.T) {
	item := markdownTab(t, projectWithDocuments(t))
	frame := item.Draw()
	if strings.Contains(frame.Lines[0], "▏") {
		t.Fatalf("the search field drew a caret of its own: %q", frame.Lines[0])
	}
	if frame.CursorY != 0 || frame.CursorX != len([]rune(history.StripCodes(frame.Lines[0]))) {
		t.Fatalf("the cursor should sit at the end of the search field, got %d,%d for %q",
			frame.CursorX, frame.CursorY, history.StripCodes(frame.Lines[0]))
	}
}

// TestMdListUsesOnlyThePalette — the list is drawn by the cell, outside the
// screen, and it still can't pick a color of its own: no indexed color, and
// the picked row is the palette's selection.
func TestMdListUsesOnlyThePalette(t *testing.T) {
	item := markdownTab(t, projectWithDocuments(t))
	drawing := strings.Join(item.Draw().Lines, "\n")
	if strings.Contains(drawing, "38;5;") || strings.Contains(drawing, "48;5;") {
		t.Fatalf("the list wrote an indexed color instead of a token:\n%q", drawing)
	}
	if !strings.Contains(history.StripCodes(drawing), "▌ README.md") {
		t.Fatalf("the picked document should be a row with a bar on its left:\n%s", history.StripCodes(drawing))
	}
}

// TestPageHeaderSaysWhatIsOnScreen — the count is of the lines the cell
// shows, from the first frame; and a file outside the project is named from
// home, not as a climb of "../" from the project.
func TestPageHeaderSaysWhatIsOnScreen(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "work", "beta")
	plan := filepath.Join(home, ".claude", "plans", "plan.md")
	for _, dir := range []string{project, filepath.Dir(plan)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("prepare: %v", err)
		}
	}
	if err := os.WriteFile(plan, []byte(strings.Repeat("line\n\n", 20)), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	md := &Md{}
	if err := md.Spawn(Config{Directory: project, Target: plan, Columns: 80, Lines: 6}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer md.Kill()
	header := history.StripCodes(md.Draw().Lines[0])
	if !strings.Contains(header, " ~/.claude/plans/plan.md ") || strings.Contains(header, "..") {
		t.Errorf("a file outside the project is named from home: %q", header)
	}
	if !strings.Contains(header, " 1–5 of ") {
		t.Errorf("a cell of six lines shows five of the page under its header: %q", header)
	}
}
