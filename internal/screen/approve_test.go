package screen

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

func blockedCell(ask string) protocol.Cell {
	return protocol.Cell{ID: "c1", Type: "claude", Name: "auth", State: "approve", Live: true, Ask: ask, Detail: "Bash: pnpm migrate"}
}

// TestStatusLineOffersTheApproval — enter and esc only mean approve and
// decline while the status line says so: on a tool approval the mod reported,
// with the application holding the keyboard and the cell at its live end.
func TestStatusLineOffersTheApproval(t *testing.T) {
	var none protocol.State
	tool := blockedCell("tool")
	line := noStyle(footerFor(none, &tool, keyboard.Browse, 140, "", 0))
	for _, want := range []string{"⏵ Bash: pnpm migrate", "Enter approves", "Esc declines", "ctrl+l enters"} {
		if !strings.Contains(line, want) {
			t.Errorf("the status line should say %q: %q", want, line)
		}
	}
	// Narrow, what waits is cut — and says it was — and the keys stay whole.
	narrow := footerFor(none, &tool, keyboard.Browse, 56, "", 0)
	if got := noStyle(narrow); !strings.Contains(got, "ctrl+l enters") || !strings.Contains(got, "⏵ Bash") || !strings.Contains(got, "…") {
		t.Errorf("a narrow status line keeps what waits and the keys: %q", got)
	}
	if widthOf(narrow) != 56 {
		t.Errorf("the status line keeps the screen's width, got %d", widthOf(narrow))
	}

	question := blockedCell("question")
	if line := noStyle(footerFor(none, &question, keyboard.Browse, 140, "", 0)); strings.Contains(line, "approves") || !strings.Contains(line, "↵ enters the cell to answer") {
		t.Errorf("a question is answered inside the cell: %q", line)
	}

	usual := noStyle(footer(none, keyboard.Browse, 140, ""))
	scrolled := blockedCell("tool")
	scrolled.Live = false
	for name, item := range map[string]protocol.Cell{"read off the screen": blockedCell(""), "scrolled back": scrolled} {
		if line := noStyle(footerFor(none, &item, keyboard.Browse, 140, "", 0)); line != usual {
			t.Errorf("an approve %s keeps the usual status line: %q", name, line)
		}
	}
	if line := noStyle(footerFor(none, &tool, keyboard.Type, 140, "", 0)); strings.Contains(line, "approves") {
		t.Errorf("in TYPE the keys are the cell's: %q", line)
	}
}

// TestStatusLineAtRestIsOneKey — nothing to offer, nothing to count: the
// line is the key that opens the help, and in TYPE the one key that isn't
// the cell's. Under a dialog it says nothing — the dialog lists its own.
func TestStatusLineAtRestIsOneKey(t *testing.T) {
	var none protocol.State
	for mode, want := range map[keyboard.Mode]string{
		keyboard.Browse: " ?", keyboard.Type: " ctrl+l",
		keyboard.Form: "", keyboard.DockerPanel: "", keyboard.IDEPanel: "",
	} {
		if got := noStyle(footer(none, mode, 80, "")); got != want {
			t.Errorf("%s at rest: the status line is %q, got %q", mode, want, got)
		}
	}
	// An error is the one thing that takes the line over, whole.
	failed := footer(protocol.State{Quota: &protocol.Quota{Percent: 3}}, keyboard.Browse, 30, "Engine did not respond")
	if got := noStyle(failed); got != " ✖ Engine did not respond" {
		t.Errorf("an error outlives the figures beside it: %q", got)
	}
}

// TestStatusLineSaysEachThingOnce — who's calling and the quota, once each
// and on this line only; the quota gives way first on a narrow screen.
func TestStatusLineSaysEachThingOnce(t *testing.T) {
	state := testGrid()
	state.Quota = &protocol.Quota{Percent: 21, Rollover: "3:12"}
	drawing := noStyle(Draw(state, Focus{}, keyboard.Browse, 120, 30, ""))
	lines := strings.Split(drawing, "\n")
	last := lines[len(lines)-1]
	if !strings.HasSuffix(last, "⬤ 1  ⏵ 1   quota 21%") || !strings.HasPrefix(last, " ?") {
		t.Fatalf("the status line is the help's key, the tally and the quota: %q", last)
	}
	for _, once := range []string{"quota", "21%"} {
		if strings.Count(drawing, once) != 1 {
			t.Errorf("%q shows up %d times on the screen", once, strings.Count(drawing, once))
		}
	}
	// The mode's name is nowhere: BROWSE is the screen at rest.
	if strings.Contains(drawing, "BROWSE") || strings.Contains(drawing, "T E S S E R A C T") {
		t.Errorf("a screen at rest names neither its mode nor itself:\n%s", drawing)
	}

	narrow := noStyle(footerFor(state, &state.Projects[1].Cells[0], keyboard.Browse, 20, "", 0))
	if strings.Contains(narrow, "quota") || !strings.Contains(narrow, "⏵ 1") {
		t.Errorf("on a narrow screen the quota goes before the tally: %q", narrow)
	}
}

// TestEnterOnABlockedCellStaysOutOfIt — the key that enters a cell doesn't
// enter one whose footer offers the approval; ctrl+l always does.
func TestEnterOnABlockedCellStaysOutOfIt(t *testing.T) {
	grid := func(item protocol.Cell) protocol.State {
		return protocol.State{Projects: []protocol.Project{{ID: "p", Name: "p", Cells: []protocol.Cell{item}}}}
	}
	press := func(m *Model, code rune, mod tea.KeyMod) {
		m.Update(tea.KeyPressMsg{Code: code, Mod: mod})
	}

	m := NewModel(nil, grid(blockedCell("tool")))
	press(m, tea.KeyEnter, 0)
	if m.typing {
		t.Fatal("enter on a tool approval entered the cell")
	}
	press(m, 'l', tea.ModCtrl)
	if !m.typing {
		t.Fatal("ctrl+l should take the keyboard into the cell whatever it waits on")
	}

	for _, ask := range []string{"question", "plan", ""} {
		m := NewModel(nil, grid(blockedCell(ask)))
		press(m, tea.KeyEnter, 0)
		if !m.typing {
			t.Fatalf("enter on an approve of kind %q should enter the cell", ask)
		}
	}
}

// TestApproveBarFillsTheHeaderInType — the cell that holds the keyboard gets
// the same solid bar as any other: the whole top line, written out, with no
// hole between its pieces. The mode's word stays where it was, on the bottom
// line.
func TestApproveBarFillsTheHeaderInType(t *testing.T) {
	box := cellBox(blockedCell("tool"), theme.ProjectHue(0), keyboard.Type, true, 60, 4)
	header := box[0]
	bar := codeOf(theme.StateBlock, true)
	if got := strings.Count(header, bar); got < 4 {
		t.Fatalf("the bar should paint the corners, the label, the rule and the state — painted %d pieces: %q", got, header)
	}
	for _, piece := range strings.Split(header, "\x1b[m") {
		if piece != "" && !strings.Contains(piece, bar) {
			t.Fatalf("a piece of the blocked header is not part of the bar: %q in %q", piece, header)
		}
	}
	if !strings.Contains(noStyle(header), "⏵ APPROVE") {
		t.Fatalf("approve is the one state written out: %q", noStyle(header))
	}
	if !strings.Contains(noStyle(box[len(box)-1]), " TYPE ") {
		t.Fatalf("the blocked cell still says it holds the keyboard: %q", noStyle(box[len(box)-1]))
	}
}

// TestHeaderSaysOnlyWhatIsTrue — a state is its glyph and no word; an idle
// agent has none; a summary sits before the glyph while there's room; the
// context only shows up when it's about to run out.
func TestHeaderSaysOnlyWhatIsTrue(t *testing.T) {
	header := func(item protocol.Cell, width int) string {
		item.ID, item.Type, item.Name, item.Live = "c1", "claude", "auth", true
		return noStyle(cellBox(item, theme.ProjectHue(0), keyboard.Browse, false, width, 4)[0])
	}

	if got := header(protocol.Cell{State: "idle"}, 60); strings.ContainsAny(got, "▸⬤○◐◌") {
		t.Errorf("an idle cell says nothing about activity: %q", got)
	}
	for state, glyph := range map[string]string{
		"working": "▸", "answered": "⬤", "down": "✖", "stopped": "○",
		"interrupted": "◐", "background": "◌", "orphan": "⚠",
	} {
		got := header(protocol.Cell{State: state}, 60)
		if !strings.HasSuffix(got, " "+glyph+" ╮") {
			t.Errorf("%s ends the top line in its glyph: %q", state, got)
		}
		for _, word := range []string{"WORKING", "RESPONDED", "CRASHED", "STOPPED", "INTERRUPTED", "BACKGROUND", "ORPHANED"} {
			if strings.Contains(got, word) {
				t.Errorf("%s is written out, and only approve is: %q", state, got)
			}
		}
	}

	answered := protocol.Cell{State: "answered", Detail: "Migrated the three tables"}
	if got := header(answered, 60); !strings.HasSuffix(got, "─ Migrated the three tables ⬤ ╮") {
		t.Errorf("the summary sits before the glyph: %q", got)
	}
	if got := header(answered, 30); strings.Contains(got, "Migrated") || !strings.HasSuffix(got, " ⬤ ╮") {
		t.Errorf("with no room for the summary the glyph stands alone: %q", got)
	}

	if got := header(protocol.Cell{State: "working", Context: 79}, 60); strings.Contains(got, "ctx") {
		t.Errorf("a window with room left isn't worth a word: %q", got)
	}
	if got := header(protocol.Cell{State: "working", Context: 86}, 60); !strings.Contains(got, "ctx 86% ▸ ╮") {
		t.Errorf("a window about to run out says so: %q", got)
	}
}

// TestNarrowCellKeepsItsName — the tabs that aren't showing give way before
// the name does: the name is who the cell is.
func TestNarrowCellKeepsItsName(t *testing.T) {
	item := protocol.Cell{ID: "c1", Name: "refatora auth", State: "working", Live: true,
		Tabs: []string{"claude", "cursor", "bash", "md"}, Tab: "bash"}
	wide := noStyle(cellBox(item, theme.ProjectHue(0), keyboard.Browse, true, 60, 4)[0])
	if !strings.Contains(wide, "claude  cursor  bash  md  refatora auth") {
		t.Errorf("with room every tab is there: %q", wide)
	}
	tight := noStyle(cellBox(item, theme.ProjectHue(0), keyboard.Browse, true, tightCellWidth, 4)[0])
	if !strings.HasPrefix(tight, "╭ bash  refatora au") || !strings.HasSuffix(tight, " ▸ ╮") || lipgloss.Width(tight) != tightCellWidth {
		t.Errorf("a tight cell keeps the tab that shows, the name and the state: %q", tight)
	}
}

// TestQuotaIsOneFigure — a number while there's room to spare; the rollover
// once it's about to run out; the 7-day window takes its place only when
// it's the one running out.
func TestQuotaIsOneFigure(t *testing.T) {
	line := func(quota protocol.Quota) string {
		return noStyle(footer(protocol.State{Quota: &quota}, keyboard.Browse, 120, ""))
	}
	if got := line(protocol.Quota{Percent: 12, Rollover: "3:41", Weekly: 60}); !strings.HasSuffix(got, "quota 12%") || strings.Contains(got, "7d") {
		t.Errorf("with room to spare the quota is a number: %q", got)
	}
	if got := line(protocol.Quota{Percent: 86, Rollover: "0:41", Weekly: 60}); !strings.HasSuffix(got, "quota 86% · 0:41") {
		t.Errorf("about to run out, the quota says when it rolls over: %q", got)
	}
	if got := line(protocol.Quota{Percent: 12, Rollover: "3:41", Weekly: 91}); !strings.HasSuffix(got, "7d 91%") || strings.Contains(got, "12%") {
		t.Errorf("a 7-day window about to run out takes the figure: %q", got)
	}
}

// TestApprovalWaitsToBeRead — "arrow, enter" is how a cell is entered. Landing
// on a blocked cell and pressing enter in the same burst must not approve a
// command whose offer was never seen: enter and esc answer only once the
// offer has sat on the status line for offerSettle, and before that they do
// nothing.
func TestApprovalWaitsToBeRead(t *testing.T) {
	now := time.Unix(1000, 0)
	clock = func() time.Time { return now }
	t.Cleanup(func() { clock = time.Now })

	here, there := net.Pipe()
	t.Cleanup(func() { here.Close(); there.Close() })
	answers := make(chan protocol.Answer, 4)
	go func() {
		decoder := json.NewDecoder(there)
		for {
			var envelope protocol.Message
			if decoder.Decode(&envelope) != nil {
				return
			}
			if envelope.Type == protocol.TypeAnswer {
				answer, _ := protocol.Unpack[protocol.Answer](envelope)
				answers <- answer
			}
		}
	}()
	answered := func() *protocol.Answer {
		select {
		case answer := <-answers:
			return &answer
		case <-time.After(150 * time.Millisecond):
			return nil
		}
	}

	idle := protocol.Cell{ID: "c0", Type: "claude", Name: "idle", State: "idle", Live: true}
	for _, key := range []struct {
		name    string
		code    rune
		approve bool
	}{{"enter", tea.KeyEnter, true}, {"esc", tea.KeyEscape, false}} {
		state := protocol.State{Projects: []protocol.Project{{ID: "p", Name: "p", Cells: []protocol.Cell{idle, blockedCell("tool")}}}}
		m := NewModel(&Client{conn: here, encoder: json.NewEncoder(here)}, state)
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		m.View()

		// The burst: the focus lands on the blocked cell, one frame is drawn,
		// and the key follows before anyone could read it.
		m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		if item := m.focusedCell(); item == nil || item.ID != "c1" {
			t.Fatalf("the arrow should have landed on the blocked cell")
		}
		m.View()
		now = now.Add(offerSettle - time.Millisecond)
		m.Update(tea.KeyPressMsg{Code: key.code})
		if got := answered(); got != nil {
			t.Fatalf("%s in the same burst as the arrow answered the approval: %+v", key.name, got)
		}
		if m.typing {
			t.Fatalf("%s on an offer not yet read entered the cell", key.name)
		}

		m.View()
		now = now.Add(time.Millisecond)
		m.Update(tea.KeyPressMsg{Code: key.code})
		got := answered()
		if got == nil || got.Cell != "c1" || got.Approve != key.approve {
			t.Fatalf("%s on an offer that was read should answer it, got %+v", key.name, got)
		}

		// An error takes the status line over: with the offer off screen,
		// the keys stop answering until it's back and read again.
		m.err = "Engine did not respond"
		m.View()
		m.err = ""
		m.View()
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if got := answered(); got != nil {
			t.Fatalf("an offer that just came back was answered unread: %+v", got)
		}
	}
}

// TestHelpCarriesItsOwnWayOut — under the help the status line says nothing,
// so the box's last line is where esc is, whether or not the help scrolls;
// and the one state drawn as a bar keeps its glyph in the legend's column.
func TestHelpCarriesItsOwnWayOut(t *testing.T) {
	for _, height := range []int{12, 40, 200} {
		box := strings.Split(noStyle(strings.Join(helpBox(120, height, 0), "\n")), "\n")
		last := box[len(box)-3]
		if !strings.Contains(last, "esc close") {
			t.Errorf("at height %d the help's last line should carry esc: %q", height, last)
		}
		if scrolls := helpLastScroll(height) > 0; scrolls != strings.Contains(last, "↑↓ scroll") {
			t.Errorf("at height %d the scroll keys show only when the help scrolls: %q", height, last)
		}
		if len(box) > height-1 {
			t.Errorf("at height %d the help is %d lines tall and covers the status line", height, len(box))
		}
	}

	column := func(line, glyph string) int { return widthOf(line[:strings.Index(line, glyph)]) }
	legend := strings.Split(noStyle(strings.Join(helpBox(120, 200, 0), "\n")), "\n")
	var approve, working string
	for _, line := range legend {
		switch {
		case strings.Contains(line, "⏵ APPROVE"):
			approve = line
		case strings.Contains(line, "▸") && strings.Contains(line, "working"):
			working = line
		}
	}
	if approve == "" || working == "" || column(approve, "⏵") != column(working, "▸") {
		t.Errorf("the approve glyph should sit in the legend's column:\n%q\n%q", approve, working)
	}

	m := NewModel(nil, protocol.State{Projects: []protocol.Project{{ID: "p", Name: "p", Cells: []protocol.Cell{blockedCell("")}}}})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.help = true
	lines := strings.Split(noStyle(m.View().Content), "\n")
	if status := lines[len(lines)-1]; strings.Contains(status, "?") {
		t.Errorf("under the help the status line offers no key: %q", status)
	}
}

// TestWindowWarningSinksInType — once a cell holds the keyboard only the
// approve bar stays lit on the others: a conversation about to run out of
// window is still said, in the same faint tone as the rest of its cell.
func TestWindowWarningSinksInType(t *testing.T) {
	inFullColor(t)
	item := protocol.Cell{ID: "c1", Type: "claude", Name: "auth", State: "working", Live: true, Context: 86}
	amber := codeOf(theme.StateBlock, false)
	lit := cellBox(item, theme.ProjectHue(0), keyboard.Browse, false, 60, 4)[0]
	if !strings.Contains(noStyle(lit), "ctx 86%") || !strings.Contains(lit, amber) {
		t.Fatalf("with the application holding the keyboard the warning is amber: %q", lit)
	}
	sunk := cellBox(item, theme.ProjectHue(0), keyboard.Type, false, 60, 4)[0]
	if !strings.Contains(noStyle(sunk), "ctx 86%") || strings.Contains(sunk, amber) {
		t.Fatalf("a cell without the keyboard keeps the warning and loses its color: %q", sunk)
	}
	if owner := cellBox(item, theme.ProjectHue(0), keyboard.Type, true, 60, 4)[0]; !strings.Contains(owner, amber) {
		t.Fatalf("the cell that holds the keyboard keeps its warning lit: %q", owner)
	}
}
