package screen

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// No color is chosen here: all of them come from internal/theme, the only
// place in the project where a hex code exists. What this file decides is
// which role each piece of the screen plays — and the roles are the brand's:
// green is whoever holds the keyboard, cyan is structure, and a state color
// only ever paints a state.
var (
	faintColor = theme.Paint(theme.FgFaint, "")
	mutedColor = theme.Paint(theme.FgMuted, "")
	barColor   = theme.Paint(theme.FgDefault, "")
	titleColor = theme.Paint(theme.FgBright, "").Bold(true)
	// Cyan is structure: the rule at the top, the corners of every box, the
	// labels. It's the second square of the symbol drawn across the whole
	// screen, and it's half the identity — in gray it vanishes and the
	// Tesseract becomes just another dark panel.
	borderColor = theme.Structure()
	// keyColor is the brighter cyan: the glyph, the numbering and the keys.
	keyColor = theme.Numbering()
	// The lines between the corners are darkness with a hint of color: the
	// grid is there to be found, not to be looked at.
	lineColor    = theme.Paint(theme.LineActive, "")
	dimLineColor = theme.Paint(theme.LineDim, "")
	// activeGridColor is the stripe of the project that has focus.
	activeGridColor = theme.Paint(theme.BrandCore, "")
	// Green is keyboard ownership. The focused cell — the one the keys act
	// on — gets the live green; a dialog, which holds the keyboard while it's
	// open, borrows the same line.
	focusedBorderColor = theme.Paint(theme.BrandLive, "").Bold(true)
	// In TYPE the cell holding the keyboard turns phosphor green. It's the
	// only place in the app that uses this color besides the mode's own
	// badge and the caret of the field being written — one owner per screen,
	// always.
	typingColor = theme.Paint(theme.BrandPhosphor, "")
	errorColor  = theme.Paint(theme.StateDead, "")
	// warnColor is the amber of whatever stops the work until you answer: a
	// form that can't be confirmed yet, a kill that takes the project along,
	// a quota about to run out. Never a decoration.
	warnColor = theme.Paint(theme.StateBlock, "")
	// selectedRowColor is the picked line of a list: raised background,
	// bright text. The bar on its left is what marks it with no color.
	selectedRowColor = theme.Paint(theme.FgBright, theme.BgRaised).Bold(true)
	selectedBar      = focusedBorderColor.Render("▌")
	// surfaceInk is the cell's body: the background that tells a cell apart
	// from the screen behind it.
	surfaceInk = theme.Tint(theme.Paint("", theme.BgSurface))
	// mutedInk is the content of a cell that doesn't hold the focus: the
	// agent's colors go quiet, the text stays readable. Frozen into a
	// terminal code because it's the app's most repeated paint job, once per
	// line of every cell that isn't the focused one.
	mutedInk = theme.Tint(theme.Paint(theme.FgMuted, theme.BgSurface))
	// faintInk is the same content once a cell takes the keyboard: the rest
	// of the screen sinks into the dark, background and all.
	faintInk = theme.Tint(faintColor)
)

type marker struct {
	symbol string
	label  string
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
	return marker{symbol: m.Glyph, label: m.Label, state: state, blocks: m.Inverted}
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

// tone is the state's color with no filled area: what a counter uses, where
// the state is being tallied and not announced. The bar stays with the cell.
func (m marker) tone() lipgloss.Style { return theme.Paint(theme.Do(m.state).Color, "") }

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
	return keyColor
}

// hint writes a shortcut the way the brand does: the key in cyan, what it
// does in the secondary tone.
func hint(short string) string {
	key, does, _ := strings.Cut(short, " ")
	if does == "" {
		return keyColor.Render(key)
	}
	return keyColor.Render(key) + " " + mutedColor.Render(does)
}

// hints lines up several shortcuts, with room between them.
func hints(shorts ...string) string {
	painted := make([]string, len(shorts))
	for i, short := range shorts {
		painted[i] = hint(short)
	}
	return strings.Join(painted, "   ")
}

// FullCellInner is the usable space of a cell taking up the whole screen:
// it subtracts the title bar, the footer and the box's borders.
func FullCellInner(width, height int) Dims {
	return Dims{Cols: max(width-2, 1), Rows: max(height-barHeight-3, 1)}
}

// DrawFull puts one cell on the whole screen. It's like copying a block of
// text without grabbing its neighbors, and it's what the full-screen key
// does.
//
// The header doesn't shrink with the view: who's calling is counted across
// the whole grid and the quota stays, since the cells left out of the
// drawing are exactly the ones this line has to speak for.
func DrawFull(state protocol.State, cell protocol.Cell, mode keyboard.Mode, width, height int, err string) string {
	lines := titleBar(mode, width, countCalls(state), state.Quota)
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
	// The rule is cyan, like every frame: it's the top of the screen, and
	// using the focused project's green here would say "keyboard ownership"
	// where there's no keyboard at all.
	paintName, paintRule := titleColor, borderColor
	dash := "─"
	if mode == keyboard.Type {
		// In TYPE the app is silent, and the stripe dims along with the rest.
		// The wordmark itself doesn't change: the brand is written one way.
		paintName, paintRule = faintColor, dimLineColor
	}

	middle := " " + markColor(mode).Render(theme.Glyph) + "  " + paintName.Render(theme.Name) + " "
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
	// On the left, who's calling you — the only piece of the bar that asks
	// for action. Each count wears its state's color, never its bar: the
	// filled area belongs to the cell that is blocked, and one tally can't
	// compete with it.
	var signals []string
	for _, state := range []string{"answered", "approve"} {
		if calls[state] == 0 {
			continue
		}
		found := markerFor(state)
		paint := found.tone()
		if mode == keyboard.Type {
			paint = faintColor
		}
		signals = append(signals, paint.Render(found.symbol+" "+strconv.Itoa(calls[state])))
	}
	left := ""
	if len(signals) > 0 {
		left = " " + strings.Join(signals, "   ")
	}

	// On the right, the window's state: quota and who owns the keyboard. The
	// top-right corner is where the eye looks for this, and that's where the
	// one inverted badge that can be visible at a time lives.
	right := borderColor.Render(mode.String())
	if mode == keyboard.Type {
		right = theme.Type.Badge()
	} else if quota != nil {
		paint := mutedColor
		if quota.Percent >= 80 {
			paint = warnColor
		}
		withQuota := paint.Render("quota "+strconv.Itoa(quota.Percent)+"% · "+quota.Rollover) + "   " + right
		// On a narrow screen the quota gives way: who owns the keyboard is
		// the one thing this line can't drop.
		if lipgloss.Width(left)+lipgloss.Width(withQuota)+2 <= width {
			right = withQuota
		}
	}
	return spread(left, right, width)
}

// footer shows what you can do right now. In TYPE it shrinks to a single
// line, the mode's third redundant signal.
func footer(mode keyboard.Mode, width int, err string) string {
	if err != "" {
		// The glyph is the dead state's: with no color, it's what tells an
		// error apart from a line of shortcuts.
		return pad(errorColor.Render(" "+truncate(theme.Do(theme.Down).Glyph+" "+err, width-1)), width)
	}
	if mode == keyboard.Type {
		// The way out is the one thing this line says, so a narrow screen
		// gets fewer words, never a sentence cut in half.
		line := "ctrl+l gives back the keyboard"
		for _, shorter := range []string{"ctrl+l gives back", "ctrl+l"} {
			if widthOf(line)+1 <= width {
				break
			}
			line = shorter
		}
		return pad(faintColor.Render(" "+line), width)
	}
	var shorts []string
	for _, shortcut := range keyboard.Shortcuts(mode) {
		if shortcut.Footer {
			shorts = append(shorts, shortcut.Short)
		}
	}
	// The last shortcut of every mode is the way out of it — help, close,
	// cancel — so on a narrow screen it's the last to go: the ones before it
	// drop first, from the end.
	for len(shorts) > 2 && lipgloss.Width(" "+hints(shorts...)) > width {
		shorts = append(shorts[:len(shorts)-2], shorts[len(shorts)-1])
	}
	return pad(" "+hints(shorts...), width)
}

// emptyScreen is what shows up when the grid has no cell at all.
func emptyScreen(state protocol.State, mode keyboard.Mode, width, height int, err string) string {
	warning := state.Notice
	lines := titleBar(mode, width, nil, state.Quota)
	// Every empty state ends in a key.
	message := mutedColor.Render("No cells yet. ") + hint("n creates the first one.")
	if warning != "" {
		message = mutedColor.Render(warning)
	}
	// The empty grid is the only moment the brand fits whole on the screen
	// without stealing workspace.
	middle := append(theme.PaintedSymbol(), "", message)

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

// spread puts one piece at each end of a line. When the two don't fit, the
// right one stays whole and the left one is cut: on the status line the
// right is who owns the keyboard, the one thing that line can't drop.
func spread(left, right string, width int) string {
	// One column of margin on the right, to match the one the left opens with.
	room := max(width-lipgloss.Width(right)-2, 0)
	return pad(pad(left, room)+" "+right, width)
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
	cut := truncate(text, width)
	// A double-wide letter left out at the edge leaves a column to fill.
	return cut + strings.Repeat(" ", max(width-lipgloss.Width(cut), 0))
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
// in the middle or leaking color into the rest of the line. The width is
// the one the terminal draws: an emoji or a CJK letter takes two columns,
// and one that would straddle the cut is left out whole — so the result can
// come out a column short, and whoever needs the exact width pads it.
func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if widthOf(text) <= width {
		return text
	}
	cut := ansi.Truncate(text, width, "")
	if strings.Contains(text, "\x1b") {
		cut += "\x1b[0m"
	}
	return cut
}

// plain strips the color codes from the content and leaves the letters. It's
// the first half of what makes only the focused cell stay lit: with many
// sections on screen, color turns into noise if everything shouts at once.
// Whoever calls it paints the result in a single tone.
func plain(text string) string {
	if !strings.Contains(text, "\x1b") {
		return text
	}
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
	return out.String()
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
// cell has tabs, the tabs become the label — the active one in the label's
// cyan and in bold, which is what still tells it apart with no color. No tab
// is ever an inverted badge: the screen has room for one, and it's the
// mode's. Whoever calls it brings the three paints, already on the header's
// background, so the label comes out as one unbroken band.
func cellLabel(item protocol.Cell, name, tabOn, tabOff lipgloss.Style) string {
	if len(item.Tabs) == 0 {
		return tabOff.Render(" "+item.Type+" · ") + name.Render(item.Name+" ")
	}
	asleep := map[string]bool{}
	for _, tab := range item.Asleep {
		asleep[tab] = true
	}
	var label strings.Builder
	label.WriteString(tabOff.Render(" "))
	for _, tab := range item.Tabs {
		switch {
		case tab == item.Tab:
			label.WriteString(tabOn.Render(" " + tab + " "))
		// A sleeping tab keeps its place and says it's sleeping. It's not a
		// state of the cell — nothing there is asking for anything — so it
		// gets a dot, not a marker.
		case asleep[tab]:
			label.WriteString(tabOff.Render(" ·" + tab + " "))
		default:
			label.WriteString(tabOff.Render(" " + tab + " "))
		}
	}
	label.WriteString(name.Render(" " + item.Name + " "))
	return label.String()
}
