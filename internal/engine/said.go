package engine

import (
	"fmt"
	"math"

	"github.com/charmbracelet/x/vt"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// summaryLimit caps the one-line answer a mod writes for the cell's header.
const summaryLimit = 60

// asks are the kinds of approve a mod may name. Anything else is taken as
// no kind at all, and a cell with no kind is never answered from the mosaic.
var asks = map[string]bool{"tool": true, "question": true, "plan": true}

// said is what a cell's mod told about the state it reported last, kept for
// as long as the cell stays in that state: what the approve is waiting for,
// or the answer in one line. The notice takes its own copy once (detail);
// this one is what the screen keeps showing.
type said struct {
	state  cell.State
	tab    string
	detail string
	ask    string
}

// note keeps what the mod said along with a state. Called with the lock
// held, after detail was set from the report.
func (c *liveCell) note(report protocol.Report, state cell.State) {
	c.said = said{state: state, tab: report.Tab}
	switch state {
	case cell.Approve:
		c.said.detail = c.detail
		if asks[report.Ask] {
			c.said.ask = report.Ask
		}
	case cell.Replied:
		// A summary the mod wrote beats the opening of the answer in the
		// notice, and it's the only text of an answer the header shows.
		if summary := cleanDetail(report.Summary); summary != "" {
			if runes := []rune(summary); len(runes) > summaryLimit {
				summary = string(runes[:summaryLimit-1]) + "…"
			}
			c.detail, c.said.detail = summary, summary
		}
	}
}

// saidFor is what the screen shows next to the state the cell is in now:
// nothing when the mod spoke about another state, or for another tab.
func (s said) saidFor(state, tab string, hasTabs bool) (detail, ask string) {
	if s.state == "" || string(s.state) != state || (hasTabs && s.tab != tab) {
		return "", ""
	}
	return s.detail, s.ask
}

// reportContext keeps how full the conversation's window is. It belongs to
// the conversation, so on a cell with tabs each tab has its own.
func (e *Engine) reportContext(c *liveCell, report protocol.Report) error {
	if math.IsNaN(report.Context) || report.Context < 0 {
		return fmt.Errorf("invalid context percentage")
	}
	e.mu.Lock()
	if c.context == nil {
		c.context = map[string]int{}
	}
	c.context[report.Tab] = min(int(report.Context+0.5), 100)
	e.mu.Unlock()
	e.markDirty()
	return nil
}

// Answer settles the tool approval a cell is blocked on without entering it:
// enter approves, esc declines, typed into the cell's own terminal — the
// dialog is Claude Code's, and those are its keys.
//
// It only acts on what the cell's mod reported as a tool's permission dialog,
// still up, on the tab showing. A question with options or a plan to review
// isn't a yes or no, and an approve read off the screen has no one to say
// what it's asking: for those the user enters the cell.
func (e *Engine) Answer(request protocol.Answer) error {
	c := e.findCell(request.Cell)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", request.Cell)
	}
	tab, hasTabs := "", false
	if withTabs, ok := c.proc.(cell.WithTabs); ok {
		tab, hasTabs = withTabs.ActiveTab(), true
	}
	e.mu.Lock()
	_, ask := c.said.saidFor(string(c.proc.State()), tab, hasTabs)
	if ask == "tool" {
		// One dialog, one answer: a second key would land on whatever the
		// agent shows next.
		c.said = said{}
	}
	e.mu.Unlock()
	if ask != "tool" {
		return fmt.Errorf("Nothing to approve from here. ↵ enters the cell.")
	}
	code := vt.KeyEscape
	if request.Approve {
		code = vt.KeyEnter
	}
	return e.Key(protocol.Key{Cell: request.Cell, Code: rune(code)})
}
