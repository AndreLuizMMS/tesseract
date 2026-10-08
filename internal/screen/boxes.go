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

// The keys aren't repeated inside the box: while a dialog holds the keyboard
// the footer already lists them, and two copies of one hint drift apart.
func (p *Question) Draw(width int) []string {
	inner := min(width-8, 60)
	body := []string{selectedBar + " " + borderColor.Render(pad(p.Label, 8)) + p.Text + caret}
	body = append(body, hintLines(p.Hint, inner)...)
	return floatingBox(p.Title, body, inner)
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
	return floatingBox(c.Title, body, min(width-8, 60))
}

// helpLines lists every key, grouped by mode. No key that doesn't exist
// shows up here, because the list comes straight from the keymap.
func helpLines() []string {
	var body []string
	for _, mode := range keyboard.Modes() {
		if len(body) > 0 {
			body = append(body, "")
		}
		// A heading, not a badge: the one inverted badge the screen has
		// room for is the mode's own.
		body = append(body, "  "+borderColor.Bold(true).Render(mode.String()))
		for _, line := range keyboard.HelpLines(mode) {
			body = append(body, "  "+keyColor.Render(pad(line.Keys, 18))+mutedColor.Render(line.Help))
		}
	}
	return body
}

// helpRoom is how many of the help's lines fit on a screen of this height,
// leaving the header, the footer and the box's own frame out.
func helpRoom(height int) int { return max(height-6, 1) }

// helpLastScroll is how far the help scrolls: until its last line is the
// last one showing. Zero when everything fits.
func helpLastScroll(height int) int {
	total, room := len(helpLines()), helpRoom(height)
	if total <= room {
		return 0
	}
	// One line of the room goes to saying where the reading is.
	return total - max(room-1, 1)
}

// helpBox draws the help, scrolled. When it doesn't fit, the last line of
// the box says which lines are showing and which keys move it — every key is
// reachable on any screen, even one that shows two of them at a time.
func helpBox(width, height, scroll int) []string {
	body := helpLines()
	if room := helpRoom(height); len(body) > room {
		total, window := len(body), max(room-1, 1)
		scroll = min(max(scroll, 0), helpLastScroll(height))
		body = append(body[scroll:scroll+window:scroll+window],
			"  "+hint("↑↓ scroll")+faintColor.Render(" · "+strconv.Itoa(scroll+1)+"–"+
				strconv.Itoa(scroll+window)+" of "+strconv.Itoa(total)))
	}
	return floatingBox(theme.Glyph+" HELP", body, min(width-8, 118))
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
		// The line number is numbering: cyan.
		body = append(body, "  "+borderColor.Render(line)+text)
	}
	if len(results.Lines) == 0 {
		body = append(body, "", "  "+hints("esc close"))
	} else {
		body = append(body, "", "  "+hints("↑↓ choose", "↵ go there", "esc close"))
	}
	return floatingBox("SEARCH · "+results.Term, body, min(width-8, 76))
}

// overlay puts a box on top of the background, centered. Only the box's own
// rectangle is replaced: what's drawn beside it stays, so a dialog doesn't
// slice the cells on its row in half.
func overlay(background string, box []string, width, height int) string {
	lines := strings.Split(background, "\n")
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	top := max((height-len(box))/2, 0)
	boxWidth := min(visibleWidth(box), width)
	margin := max((width-boxWidth)/2, 0)
	for i, line := range box {
		if top+i >= len(lines) {
			break
		}
		behind := lines[top+i]
		// Both halves are padded back to their width: a double-wide letter
		// the box's edge lands on is dropped whole, and without the column
		// it leaves the box and the line's right side would shift.
		from, room := margin+boxWidth, width-margin-boxWidth
		right := ansi.Cut(behind, from, width)
		if lipgloss.Width(right) > room {
			// The cut brought along the whole letter its edge fell on.
			right = " " + ansi.Cut(behind, from+1, width)
		}
		if widthOf(behind) >= width {
			right = pad(right, room)
		}
		lines[top+i] = pad(truncate(behind, margin), margin) + "\x1b[0m" +
			pad(line, boxWidth) + right
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
