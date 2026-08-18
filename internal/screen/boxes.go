package screen

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

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
	body := []string{"  " + titleColor.Render(pad(p.Label, 8)) + p.Text + "▏"}
	if p.Hint != "" {
		body = append(body, "  "+strings.Repeat(" ", 8)+faintColor.Render(p.Hint))
	}
	body = append(body, "", "  "+faintColor.Render("↵ confirm        esc cancel"))
	return floatingBox(p.Title, body, min(width-8, 60))
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
	body = append(body, "", "  "+faintColor.Render("↵ confirm        esc cancel"))
	return floatingBox(c.Title, body, min(width-8, 60))
}

// helpBox lists every key, grouped by mode. No key that doesn't exist shows
// up here, because the list comes straight from the keymap.
func helpBox(width, height int) []string {
	var body []string
	for _, mode := range keyboard.Modes() {
		if len(body) > 0 {
			body = append(body, "")
		}
		body = append(body, "  "+badgeColor.Render(" "+mode.String()+" "))
		for _, line := range keyboard.HelpLines(mode) {
			body = append(body, "  "+barColor.Render(pad(line.Keys, 16))+faintColor.Render(line.Help))
		}
	}
	if leftover := height - 6; leftover > 0 && len(body) > leftover {
		body = body[:leftover]
		body = append(body, "  "+faintColor.Render("…"))
	}
	return floatingBox(theme.Glyph+" HELP", body, min(width-8, 76))
}

// resultsBox shows the outcome of the search across the cell's history.
func resultsBox(results protocol.Matches, chosen, width int) []string {
	var body []string
	if len(results.Lines) == 0 {
		body = append(body, "  "+faintColor.Render("nothing found for "+strconv.Quote(results.Term)))
	}
	for i, result := range results.Lines {
		if i >= 12 {
			body = append(body, "  "+faintColor.Render("… and "+strconv.Itoa(len(results.Lines)-12)+" more"))
			break
		}
		mark := "  "
		if i == chosen {
			mark = "▸ "
		}
		body = append(body, mark+faintColor.Render(pad(strconv.Itoa(result.Line), 7))+truncate(result.Text, width-24))
	}
	body = append(body, "", "  "+faintColor.Render("↑↓ choose   ↵ go there   esc close"))
	return floatingBox("SEARCH · "+results.Term, body, min(width-8, 76))
}

// overlay puts a box on top of the background, centered vertically. The
// box's stripe of lines is fully replaced — a modal is a modal.
func overlay(background string, box []string, width, height int) string {
	lines := strings.Split(background, "\n")
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	top := max((height-len(box))/2, 0)
	margin := max((width-visibleWidth(box))/2, 0)
	for i, line := range box {
		if top+i >= len(lines) {
			break
		}
		lines[top+i] = pad(strings.Repeat(" ", margin)+line, width)
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
