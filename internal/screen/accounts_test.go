package screen

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// TestAccountSitsOnTheTopLine — the account rides on the cell's top line,
// ahead of the answer and the state. It outlives the answer's summary, gives
// way to the window's warning, and a cell too narrow for it keeps its name
// and glyph.
func TestAccountSitsOnTheTopLine(t *testing.T) {
	header := func(item protocol.Cell, width int) string {
		item.ID, item.Type, item.Name, item.Live = "c1", "claude", "auth", true
		return noStyle(cellBox(item, theme.ProjectHue(0), keyboard.Browse, false, width, 4)[0])
	}

	if got := header(protocol.Cell{State: "working", Account: "norven"}, 40); !strings.HasSuffix(got, "─ norven ─ ▸ ╮") {
		t.Errorf("the account sits before the glyph: %q", got)
	}
	if got := header(protocol.Cell{State: "idle", Account: "norven"}, 40); !strings.HasSuffix(got, "─ norven ─╮") {
		t.Errorf("an idle cell still says its account: %q", got)
	}
	if got := header(protocol.Cell{State: "working", Account: "norven", Context: 86}, 40); !strings.HasSuffix(got, " norven ─ ctx 86% ▸ ╮") {
		t.Errorf("the window's warning and the account side by side: %q", got)
	}
	if got := header(protocol.Cell{State: "working"}, 40); strings.Contains(got, "norven") || strings.Contains(got, "primary") {
		t.Errorf("a single account says nothing: %q", got)
	}

	answered := protocol.Cell{State: "answered", Detail: "Migrated the three tables", Account: "norven"}
	if got := header(answered, 70); !strings.HasSuffix(got, "─ norven ─ Migrated the three tables ⬤ ╮") {
		t.Errorf("with room, the account, then the summary, then the glyph: %q", got)
	}
	if got := header(answered, 34); strings.Contains(got, "Migrated") || !strings.HasSuffix(got, " norven ─ ⬤ ╮") {
		t.Errorf("the summary goes before the account does: %q", got)
	}
	narrow := header(protocol.Cell{State: "working", Account: "norven"}, 24)
	if strings.Contains(narrow, "norven") || !strings.HasSuffix(narrow, " ▸ ╮") || !strings.Contains(narrow, "auth") {
		t.Errorf("too narrow for the account, the name and the glyph stay: %q", narrow)
	}
	for _, width := range []int{12, 20, 24, 30, 40, 70} {
		if got := lipgloss.Width(header(answered, width)); got != width {
			t.Errorf("at %d columns the top line is %d wide", width, got)
		}
	}

	free := protocol.Cell{ID: "c1", Type: "claude", Name: "auth", Live: true, State: "working", Account: "primary"}
	out := free
	out.AccountOut = true
	painted := func(item protocol.Cell) string {
		return cellBox(item, theme.ProjectHue(0), keyboard.Browse, false, 40, 4)[0]
	}
	if noStyle(painted(out)) != noStyle(painted(free)) || painted(out) == painted(free) {
		t.Error("an account at its limit is the same word in another color")
	}
}

// TestStatusLineShowsEachAccount — with two accounts each one has its own
// number; the one at its limit reads 100% and when it's back, in amber.
func TestStatusLineShowsEachAccount(t *testing.T) {
	line := func(accounts ...protocol.Account) string {
		return noStyle(footer(protocol.State{Accounts: accounts}, keyboard.Browse, 120, ""))
	}
	out := protocol.Account{Name: "primary", Quota: &protocol.Quota{Percent: 100, Rollover: "2:47"}, Until: "15:00"}
	norven := protocol.Account{Name: "norven", Quota: &protocol.Quota{Percent: 34, Rollover: "4:10"}}

	got := line(out, norven)
	if !strings.HasSuffix(got, "primary 100% · 2:47 │ norven 34%") || strings.Contains(got, "quota") {
		t.Errorf("each account says its own usage: %q", got)
	}
	if got := line(out, protocol.Account{Name: "norven"}); !strings.HasSuffix(got, "│ norven") {
		t.Errorf("an account nobody measured is its name alone: %q", got)
	}
	weekly := protocol.Account{Name: "norven", Quota: &protocol.Quota{Percent: 20, Weekly: 91}}
	if got := line(out, weekly); !strings.HasSuffix(got, "│ norven 7d 91%") {
		t.Errorf("the 7-day window takes the figure, after the account's name: %q", got)
	}
	// The old badge still speaks for an engine that sends no accounts.
	if got := noStyle(footer(protocol.State{Quota: &protocol.Quota{Percent: 12}}, keyboard.Browse, 120, "")); !strings.HasSuffix(got, "quota 12%") {
		t.Errorf("a single account keeps its badge: %q", got)
	}
}

// TestAccountPickerMovesTheConversation — a opens the list on the account the
// cell runs on; an account at its limit is refused with when it's back, the
// current one closes, and a free one sends the move.
func TestAccountPickerMovesTheConversation(t *testing.T) {
	accounts := []protocol.Account{
		{Name: "primary", Quota: &protocol.Quota{Percent: 100}, Until: "15:00"},
		{Name: "norven", Quota: &protocol.Quota{Percent: 34}},
		{Name: "work"},
	}
	item := protocol.Cell{ID: "c1", Name: "doxar", State: "idle", Account: "norven"}

	if _, why := NewAccountPicker(item, accounts[:1]); !strings.Contains(why, "Only one Claude account") {
		t.Errorf("one account, nothing to pick: %q", why)
	}
	if _, why := NewAccountPicker(protocol.Cell{Name: "doxar"}, accounts); !strings.Contains(why, "no Claude running") {
		t.Errorf("a cell with no Claude has nothing to move: %q", why)
	}
	if _, why := NewAccountPicker(protocol.Cell{Name: "doxar", Tabs: []string{"claude", "bash"}}, accounts); !strings.Contains(why, "claude tab isn't up") {
		t.Errorf("a session with its claude tab asleep says so: %q", why)
	}

	picker, why := NewAccountPicker(item, accounts)
	if picker == nil || why != "" || picker.Selected != 1 {
		t.Fatalf("the picker opens on the current account: %+v %q", picker, why)
	}
	drawn := noStyle(strings.Join(picker.Draw(100, accounts, item), "\n"))
	for _, want := range []string{"ACCOUNT · doxar", "primary", "at its limit until 15:00", "norven", "← current", "↵", "esc"} {
		if !strings.Contains(drawn, want) {
			t.Errorf("the picker should show %q:\n%s", want, drawn)
		}
	}
	if strings.Contains(drawn, "cuts the turn") {
		t.Errorf("a cell at rest loses nothing by moving:\n%s", drawn)
	}
	busy := item
	busy.State = "working"
	if drawn := noStyle(strings.Join(picker.Draw(100, accounts, busy), "\n")); !strings.Contains(drawn, "moving cuts the turn in progress") {
		t.Errorf("a cell working is told its turn will be cut:\n%s", drawn)
	}

	if move, _, open := picker.Key(keyboard.Confirm, accounts, "norven"); move != nil || open {
		t.Error("the current account just closes the picker")
	}
	picker.Key(keyboard.PreviousField, accounts, "norven")
	move, refusal, open := picker.Key(keyboard.Confirm, accounts, "norven")
	if move != nil || !open || !strings.Contains(refusal, "primary is at its limit until 15:00") {
		t.Errorf("an account at its limit is refused and the picker stays: move %+v, open %v, %q", move, open, refusal)
	}
	picker.Key(keyboard.NextField, accounts, "norven")
	picker.Key(keyboard.NextField, accounts, "norven")
	picker.Key(keyboard.NextField, accounts, "norven")
	if move, _, open := picker.Key(keyboard.Confirm, accounts, "norven"); open || move == nil || *move != (protocol.Move{Cell: "c1", Account: "work"}) {
		t.Errorf("a free account sends the move: %+v open=%v", move, open)
	}
	if _, _, open := picker.Key(keyboard.Cancel, accounts, "norven"); open {
		t.Error("esc leaves")
	}
}

// TestAKeyOpensThePicker — on the grid, a opens the picker over the focused
// cell, which takes the keyboard as a dialog does, and esc gives it back.
func TestAKeyOpensThePicker(t *testing.T) {
	state := testGrid()
	state.Accounts = []protocol.Account{{Name: "primary"}, {Name: "norven"}}
	state.Projects[0].Cells[0].Account = "norven"
	m := NewModel(nil, state)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if m.accountPicker == nil || m.Mode() != keyboard.Form {
		t.Fatalf("a should open the picker and take the keyboard, err %q", m.err)
	}
	if view := noStyle(m.View().Content); !strings.Contains(view, "ACCOUNT · refatora auth") {
		t.Errorf("the picker is drawn over the grid:\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.accountPicker != nil || m.Mode() != keyboard.Browse {
		t.Fatal("esc closes the picker")
	}

	m.focus = Focus{Project: 2}
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if m.accountPicker != nil || !strings.Contains(m.err, "no Claude running") {
		t.Fatalf("a cell with no Claude says why instead of opening, got %q", m.err)
	}
}
