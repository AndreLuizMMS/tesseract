package screen

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// No color is chosen here: all of them come from internal/theme, the only
// place in the project where a hex code exists. In TYPE the app goes silent
// and shows that it is silent: the bar and stripes dim, the badge appears,
// and the focused cell's border thickens.
var (
	faintColor = theme.Paint(theme.FgFaint, "")
	barColor   = theme.Paint(theme.FgDefault, "")
	badgeColor = theme.Paint(theme.BgVoid, theme.StateBlock).Bold(true)
	titleColor = theme.Paint(theme.FgBright, "").Bold(true)
	// The cell grid is cyan: structure. It's the second square of the symbol
	// drawn across the whole screen, and it's half the identity — in gray it
	// vanishes and the Tesseract becomes just another dark panel.
	borderColor        = theme.Paint(theme.FluxCore, "")
	focusedBorderColor = theme.Paint(theme.BrandLive, "").Bold(true)
	scrollColor        = theme.Paint(theme.StateBlock, "")
	// In TYPE the cell holding the keyboard turns phosphor green: it's the
	// mode's fourth signal, and the only one lit on the screen. It's the only
	// place in the app that uses this color — once per screen, always.
	typingColor    = theme.Paint(theme.BrandPhosphor, "")
	errorColor     = theme.Paint(theme.StateDead, "")
	quotaColor     = theme.Paint(theme.FgMuted, "")
	quotaHighColor = theme.Paint(theme.StateBlock, "")
	// activeGridColor is the stripe of the project that has focus.
	activeGridColor = theme.Paint(theme.LineActive, "")
	// faintTint is the gray of unfocused cells' content, frozen into a
	// terminal code: it's the app's most repeated paint job, once per line
	// of every cell that doesn't hold the keyboard.
	faintTint = theme.Tint(faintColor)
)

// projectColor is the project's name on the stripe: uppercase in fg.bright,
// the top of the text hierarchy. One color per project turned into a rainbow
// and fought the one signal that needs to shout — the cell holding the
// keyboard. What tells one project from another is the stripe, the name and
// the position, not the hue.
func projectColor(int) lipgloss.Style {
	return theme.Paint(theme.FgBright, "")
}

type marker struct {
	symbol string
	state  theme.State
	// blocks marks the state that won't move without you. It doesn't change
	// hue to draw attention — it changes area, becoming a solid inverted
	// bar — and it's the only one that blinks.
	blocks bool
}

// markers translates a cell's state into its symbol and theme state. Colors
// all come from theme.Do — no state is green or cyan, since those two mean
// keyboard ownership and structure.
var markers = map[string]marker{
	"working":  fromTheme(theme.Working),
	"answered": fromTheme(theme.Answered),
	"approve":  fromTheme(theme.Approve),
	"down":     fromTheme(theme.Down),
	"stopped":  fromTheme(theme.Stopped),
	"orphan":   fromTheme(theme.Orphan),
}

func fromTheme(state theme.State) marker {
	m := theme.Do(state)
	return marker{symbol: m.Glyph, state: state, blocks: m.Inverted}
}

// color asks the theme every frame, instead of caching the finished style:
// the bar that blinks changes look between one frame and the next, and a
// style frozen at program start would never blink.
func (m marker) color() lipgloss.Style { return theme.Do(m.state).Style() }

// chip turns a text color into the cell state's filled badge: the same
// raised background for every state, bold, only the text color changes.
func chip(color lipgloss.Style) lipgloss.Style {
	return theme.WithBackground(color, theme.BgRaised).Bold(true)
}

// badge is how the marker shows up on the cell's border. A blocking state
// already comes inverted from the theme and doesn't get the chip's
// background: it is the filled area.
func (m marker) badge() lipgloss.Style {
	if m.blocks {
		return m.color()
	}
	return chip(m.color())
}

func markerFor(state string) marker {
	if found, exists := markers[state]; exists {
		return found
	}
	return markers["stopped"]
}

// markColor is the brand glyph on the bar. In TYPE it dims along with
// everything else: the app is silent, and so is the mark.
func markColor(mode keyboard.Mode) lipgloss.Style {
	if mode == keyboard.Type {
		return faintColor
	}
	return theme.Paint(theme.Flux, "")
}

// FullCellInner is the usable space of a cell taking up the whole screen:
// it subtracts the title bar, the footer and the box's borders.
func FullCellInner(width, height int) Dims {
	return Dims{Cols: max(width-2, 1), Rows: max(height-barHeight-3, 1)}
}

// DrawFull puts one cell on the whole screen. It's like copying a block of
// text without grabbing its neighbors, and it's what the full-screen key
// does.
func DrawFull(project protocol.Project, cell protocol.Cell, mode keyboard.Mode, width, height int, err string) string {
	lines := titleBar(mode, width, countProjectCalls(project), nil)
	lines = append(lines, cellBox(cell, mode, true, width, height-barHeight-1)...)
	lines = append(lines, footer(mode, width, err))
	return strings.Join(lines, "\n")
}

// barHeight is how much room the header takes: the brand stripe and the
// status line. Every layout calculation subtracts this, instead of
// repeating the number.
const barHeight = 2

// titleBar is the header: a stripe with the brand in the middle of a rule,
// and below it who's calling you and who owns the keyboard. It's the first
// of the mode's three redundant signals.
func titleBar(mode keyboard.Mode, width int, calls map[string]int, quota *protocol.Quota) []string {
	return []string{
		brandStripe(mode, width),
		statusLine(mode, width, calls, quota),
	}
}

// brandStripe is the rule that crosses the screen with the name in the
// middle. The rule is cyan because it's structure, and the name comes
// letter-spaced — that's how the brand writes itself, and on a terminal the
// space between letters is all the typography there is.
func brandStripe(mode keyboard.Mode, width int) string {
	// The rule is cyan, like the whole grid: it's the frame at the top of the
	// screen, and using the focused project's green here would say "keyboard
	// ownership" where there's no keyboard at all.
	name := "T E S S E R A C T"
	paintName, paintRule := titleColor, borderColor
	dash := "─"
	if mode == keyboard.Type {
		// In TYPE the app is silent, and the stripe dims along with the rest.
		name, paintName, paintRule = "t e s s e r a c t", faintColor, faintColor
	}

	middle := " " + markColor(mode).Render(theme.Glyph) + "  " + paintName.Render(name) + " "
	leftover := width - lipgloss.Width(middle)
	if leftover < 4 {
		return pad(centerIn(middle, width), width)
	}
	left := leftover / 2
	return paintRule.Render(strings.Repeat(dash, left)) + middle +
		paintRule.Render(strings.Repeat(dash, leftover-left))
}

// statusLine is the header's second line: who's calling on one side, the
// window's state on the other.
func statusLine(mode keyboard.Mode, width int, calls map[string]int, quota *protocol.Quota) string {
	paintSignals := barColor
	if mode == keyboard.Type {
		paintSignals = faintColor
	}

	// On the left, who's calling you — the only piece of the bar that asks
	// for action.
	var signals []string
	for _, state := range []string{"answered", "approve"} {
		if calls[state] > 0 {
			signals = append(signals, markerFor(state).symbol+" "+strconv.Itoa(calls[state]))
		}
	}
	left := ""
	if len(signals) > 0 {
		left = " " + paintSignals.Render(strings.Join(signals, "   "))
	}

	// On the right, the window's state: quota and who owns the keyboard. The
	// top-right corner is where the eye looks for this, and that's where the
	// one inverted badge that can be visible at a time lives.
	right := paintSignals.Render(mode.String())
	if mode == keyboard.Type {
		right = badgeColor.Render(" ▓ TYPE ▓ ")
	} else if quota != nil {
		paint := quotaColor
		if quota.Percent >= 80 {
			paint = quotaHighColor
		}
		right = paint.Render("⏳ "+strconv.Itoa(quota.Percent)+"% "+quota.Rollover) + "   " + right
	}
	return threeParts(left, "", right, width)
}

// footer shows what you can do right now. In TYPE it shrinks to a single
// line, the mode's third redundant signal.
func footer(mode keyboard.Mode, width int, err string) string {
	if err != "" {
		return pad(errorColor.Render(" "+truncate(err, width-1)), width)
	}
	if mode == keyboard.Type {
		return pad(faintColor.Render(" ctrl-l gives back the keyboard"), width)
	}
	var hints []string
	for _, shortcut := range keyboard.Shortcuts(mode) {
		if !shortcut.Footer {
			continue
		}
		hints = append(hints, barColor.Render(shortcut.Short))
	}
	return pad(" "+strings.Join(hints, "   "), width)
}

// emptyScreen is what shows up when the grid has no cell at all.
func emptyScreen(mode keyboard.Mode, width, height int, err, warning string) string {
	lines := titleBar(mode, width, nil, nil)
	message := "no cell on screen — n creates the first one"
	if warning != "" {
		message = warning
	}
	// The empty grid is the only moment the brand fits whole on the screen
	// without stealing workspace. Every empty state ends in a key.
	middle := append(theme.PaintedSymbol(), "", faintColor.Render(message))

	body := max(height-barHeight-1, 1)
	top := max((body-len(middle))/2, 0)
	for i := range body {
		if i >= top && i-top < len(middle) {
			lines = append(lines, pad(centerIn(middle[i-top], width), width))
			continue
		}
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(append(lines, footer(mode, width, err)), "\n")
}

// countCalls sums, across the whole grid, how many cells are asking for
// attention.
func countCalls(state protocol.State) map[string]int {
	counts := map[string]int{}
	for _, project := range state.Projects {
		for _, item := range project.Cells {
			counts[item.State]++
		}
	}
	return counts
}

func countProjectCalls(project protocol.Project) map[string]int {
	counts := map[string]int{}
	for _, item := range project.Cells {
		counts[item.State]++
	}
	return counts
}

// threeParts spreads left, middle and right across a single line.
func threeParts(left, middle, right string, width int) string {
	le, lm, ld := lipgloss.Width(left), lipgloss.Width(middle), lipgloss.Width(right)
	if le+lm+ld+2 > width {
		return pad(left+" "+middle, width)
	}
	before := (width-lm)/2 - le
	after := width - le - before - lm - ld - 1
	if before < 1 {
		before = 1
	}
	if after < 1 {
		after = 1
	}
	line := left + strings.Repeat(" ", before) + middle + strings.Repeat(" ", after) + right
	return pad(line, width)
}

// widthOf measures the line on screen. Terminal output is almost all ASCII,
// and in that case a byte is a column — the proper grapheme count, which
// knows about composed accents and double-wide letters, only kicks in when
// needed.
func widthOf(text string) int {
	for i := range len(text) {
		if text[i] < 0x20 || text[i] > 0x7e {
			return lipgloss.Width(text)
		}
	}
	return len(text)
}

// pad fills the line with spaces up to the width, truncating whatever
// overflows.
func pad(text string, width int) string {
	current := widthOf(text)
	switch {
	case current == width:
		return text
	case current < width:
		return text + strings.Repeat(" ", width-current)
	}
	return truncate(text, width)
}

// centerIn puts the text in the middle of the width.
func centerIn(text string, width int) string {
	leftover := width - lipgloss.Width(text)
	if leftover <= 0 {
		return text
	}
	return strings.Repeat(" ", leftover/2) + text + strings.Repeat(" ", leftover-leftover/2)
}

// truncate cuts the text at the visible width, without breaking color codes
// in the middle or leaking color into the rest of the line.
func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	var out strings.Builder
	visible, hadColor := 0, false
	inCode := false
	for _, r := range text {
		if r == 0x1b {
			inCode, hadColor = true, true
		}
		if inCode {
			out.WriteRune(r)
			if r >= 0x40 && r <= 0x7e && r != '[' {
				inCode = false
			}
			continue
		}
		if visible >= width {
			if hadColor {
				out.WriteString("\x1b[0m")
			}
			return out.String()
		}
		out.WriteRune(r)
		visible++
	}
	return out.String()
}

// dim strips the color from the content and returns the whole line in gray.
// It's what makes only the focused cell stay lit: with many sections on
// screen, color turns into noise if everything shouts at once.
func dim(text string) string {
	var out strings.Builder
	inCode := false
	for _, r := range text {
		if r == 0x1b {
			inCode = true
			continue
		}
		if inCode {
			if r >= 0x40 && r <= 0x7e && r != '[' {
				inCode = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return faintTint.Render(out.String())
}

// userHome is the account's home directory, read only once.
var userHome = sync.OnceValue(func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
})

func home() string { return userHome() }

// cellLabel is what shows on the top border: the type and the name. When the
// cell has tabs, the tabs become the label — the active one highlighted.
func cellLabel(item protocol.Cell) string {
	if len(item.Tabs) == 0 {
		return item.Type + " · " + item.Name
	}
	var tabs []string
	for _, tab := range item.Tabs {
		if tab == item.Tab {
			tabs = append(tabs, badgeColor.Render(" "+tab+" "))
			continue
		}
		tabs = append(tabs, faintColor.Render(" "+tab+" "))
	}
	return strings.Join(tabs, "") + " " + item.Name
}
