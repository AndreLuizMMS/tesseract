package screen

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// Question is a single-line box: rename, send a prompt, search. Its keyboard
// is the form's, so enter confirms and esc cancels.
type Question struct {
	Title string
	Label string
	Hint  string
	Text  string
	// Action remembers who opened the question, so the model knows what to do
	// with the answer.
	Action keyboard.Action
}

// Key handles a key for the question and says whether it stays open and
// whether it was confirmed.
func (p *Question) Key(action keyboard.Action, text string, erased bool) (confirmed, open bool) {
	switch action {
	case keyboard.Cancel:
		return false, false
	case keyboard.Confirm:
		return true, false
	}
	if erased {
		if runes := []rune(p.Text); len(runes) > 0 {
			p.Text = string(runes[:len(runes)-1])
		}
		return false, true
	}
	p.Text += text
	return false, true
}

func (p *Question) Draw(width int) []string {
	inner := min(width-8, 60)
	body := []string{selectedBar + " " + mutedColor.Render(pad(p.Label, 8)) + p.Text + caret}
	body = append(body, hintLines(p.Hint, inner)...)
	return floatingBox(p.Title, append(body, "", formKeys()), inner)
}

// formKeys is the last line of a dialog that asks for an answer: the two
// keys that settle it, as the keymap names them. The status line says
// nothing under a dialog, so the dialog carries its own keys.
func formKeys() string { return dialogKeys(keyboard.Form, 0)[0] }

// dialogKeys writes a dialog's keys in their short form, from the keymap, on
// as few lines as the room takes: one whenever it fits. The sentences that
// explain each key are the help's, and a dialog doesn't repeat them. No room
// given means one line.
func dialogKeys(mode keyboard.Mode, room int) []string {
	lines, width := []string{"  "}, 0
	for _, shortcut := range keyboard.Shortcuts(mode) {
		if shortcut.Short == "" {
			continue
		}
		piece := widthOf(shortcut.Short)
		switch {
		case width == 0:
		case room > 0 && width+3+piece > room:
			lines, width = append(lines, "  "), 0
		default:
			lines[len(lines)-1] += "   "
			width += 3
		}
		lines[len(lines)-1] += hint(shortcut.Short)
		width += piece
	}
	return lines
}

// hintIndent lines a field's hint up under its value, past the label.
const hintIndent = 10

// hintLines writes a field's hint under it, broken on the words when the box
// is too narrow for it: a hint cut by the border says half of what it means.
func hintLines(text string, inner int) []string {
	if text == "" {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(ansi.Wordwrap(text, max(inner-2-hintIndent, 8), ""), "\n") {
		lines = append(lines, strings.Repeat(" ", hintIndent)+mutedColor.Render(line))
	}
	return lines
}

// Confirmation is the yes-or-no of an action that cannot be undone.
type Confirmation struct {
	Title  string
	Lines  []string
	Action keyboard.Action
	Target string
}

func (c *Confirmation) Draw(width int) []string {
	var body []string
	for _, line := range c.Lines {
		body = append(body, "  "+line)
	}
	return floatingBox(c.Title, append(body, "", formKeys()), min(width-8, 60))
}

// helpLines lists every key, grouped by mode, and then the alphabet of
// states: a cell wears a glyph and no word, and this is where the word is.
// No key that doesn't exist shows up here, because the list comes straight
// from the keymap.
func helpLines() []string {
	var body []string
	for _, mode := range keyboard.Modes() {
		if len(body) > 0 {
			body = append(body, "")
		}
		body = append(body, "  "+titleColor.Render(mode.String()))
		for _, line := range keyboard.HelpLines(mode) {
			body = append(body, "  "+keyColor.Render(pad(line.Keys, 18))+mutedColor.Render(line.Help))
		}
	}
	// Not a key of the mosaic: a command the cell's mods add, typed in the
	// agent's own prompt. It's listed here because this is the help a cell has.
	body = append(body, "", "  "+titleColor.Render("IN A CLAUDE CELL"),
		"  "+keyColor.Render(pad("/dlog <service>", 18))+mutedColor.Render("puts the service's last 20 log lines in the prompt, unsent; alone, lists the services"))

	body = append(body, "", "  "+titleColor.Render("STATES"))
	for _, state := range theme.States() {
		found := theme.Do(state)
		if found.Inverted {
			// The one state that is an area: the legend shows it as the cell
			// does, a filled bar with its word.
			// The bar starts one column early so its glyph falls under the
			// others': the fill has a margin, the legend's column doesn't move.
			body = append(body, " "+pad(found.Style().Render(" "+found.Glyph+" "+found.Label+" "), 19)+
				mutedColor.Render("blocked, waiting on you — the only solid bar on the screen"))
			continue
		}
		body = append(body, "  "+theme.Paint(found.Color, "").Render(pad(found.Glyph, 18))+
			mutedColor.Render(strings.ToLower(found.Label)))
	}
	body = append(body,
		"  "+keyColor.Render(pad("▲ 120", 18))+mutedColor.Render("reading the history, that many lines back"),
		"  "+warnColor.Render(pad("ctx 86%", 18))+mutedColor.Render("the conversation's window is about to run out"),
		"  "+errorColor.Render(pad(theme.Do(theme.Down).Glyph+" docker", 18))+mutedColor.Render("a service of the project's stack fell"))
	return body
}

// helpRoom is how many of the help's lines fit on a screen of this height,
// leaving out the status line, the box's own frame and the box's last line,
// which is always its keys.
func helpRoom(height int) int { return max(height-7, 1) }

// helpLastScroll is how far the help scrolls: until its last line is the
// last one showing. Zero when everything fits.
func helpLastScroll(height int) int {
	return max(len(helpLines())-helpRoom(height), 0)
}

// helpBox draws the help, scrolled. Its last line is the way out — the
// status line says nothing under a dialog — and, when the help doesn't fit,
// which lines are showing and which keys move it: every key is reachable on
// any screen, even one that shows two of them at a time.
func helpBox(width, height, scroll int) []string {
	body := helpLines()
	keys := "  " + hint("esc close")
	if room := helpRoom(height); len(body) > room {
		total := len(body)
		scroll = min(max(scroll, 0), helpLastScroll(height))
		body = body[scroll : scroll+room : scroll+room]
		keys = "  " + hints("↑↓ scroll", "esc close") + faintColor.Render(" · "+strconv.Itoa(scroll+1)+"–"+
			strconv.Itoa(scroll+room)+" of "+strconv.Itoa(total))
	}
	return floatingBox(theme.Glyph+" HELP", append(body, keys), min(width-8, 118))
}

// resultsBox shows the outcome of the search across the cell's history.
func resultsBox(results protocol.Matches, chosen, width int) []string {
	var body []string
	if len(results.Lines) == 0 {
		body = append(body, "  "+mutedColor.Render("No match for "+strconv.Quote(results.Term)+" in this cell's history."))
	}
	for i, result := range results.Lines {
		if i >= 12 {
			body = append(body, "  "+faintColor.Render("… "+strconv.Itoa(len(results.Lines)-12)+" more"))
			break
		}
		line := pad(strconv.Itoa(result.Line), 7)
		text := truncate(plain(result.Text), width-24)
		if i == chosen {
			body = append(body, selectedBar+selectedRowColor.Render(" "+line+pad(text, max(width-24, 1))))
			continue
		}
		body = append(body, "  "+mutedColor.Render(line)+text)
	}
	if len(results.Lines) == 0 {
		body = append(body, "", "  "+hints("esc close"))
	} else {
		body = append(body, "", "  "+hints("↑↓ choose", "↵ go there", "esc close"))
	}
	return floatingBox("SEARCH · "+results.Term, body, min(width-8, 76))
}

// overlay puts a box on top of the background, centered, and clears the
// rows it sits on from edge to edge. What's left beside a dialog is a slice
// of a cell a few columns wide — half a word, a border with no box — and a
// slice reads as a broken screen. The rows above and below stay whole, so
// the grid is still there behind it.
func overlay(background string, box []string, width, height int) string {
	lines := strings.Split(background, "\n")
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	top := max((height-len(box))/2, 0)
	boxWidth := min(visibleWidth(box), width)
	margin := strings.Repeat(" ", max((width-boxWidth)/2, 0))
	for i, line := range box {
		if top+i >= len(lines) {
			break
		}
		lines[top+i] = pad(margin+pad(line, boxWidth), width)
	}
	return strings.Join(lines, "\n")
}

func visibleWidth(box []string) int {
	biggest := 0
	for _, line := range box {
		if width := lipgloss.Width(line); width > biggest {
			biggest = width
		}
	}
	return biggest
}
