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
// which role each piece of the screen plays. There are four: a project's hue
// paints its name and the line of its cells, a state's color paints its
// glyph, the phosphor marks where the typing lands, and everything else is
// the neutral ladder below.
var (
	faintColor = theme.Paint(theme.FgFaint, "")
	mutedColor = theme.Paint(theme.FgMuted, "")
	barColor   = theme.Paint(theme.FgDefault, "")
	titleColor = theme.Paint(theme.FgBright, "").Bold(true)
	// keyColor is a key's own name, wherever one is written: the weight is
	// what tells it from what the key does.
	keyColor = theme.Paint(theme.FgDefault, "").Bold(true)
	// dimLineColor is every line but the owner's while a cell holds the
	// keyboard: the rest of the screen sinks, hue and all.
	dimLineColor = theme.Paint(theme.LineDim, "")
	// boxLineColor is the line of a dialog. A dialog belongs to no project,
	// so it has no hue: the brightest neutral line on the screen.
	boxLineColor = theme.Paint(theme.FgDefault, "")
	// typingColor is where the typing lands: the word on the cell in TYPE
	// and the caret of the field being written. One per screen, always.
	typingColor = theme.Typing()
	// brandColor is the brand's own mark, and the only cyan on the screen.
	brandColor = theme.Paint(theme.Flux, "")
	errorColor = theme.Paint(theme.StateDead, "")
	// warnColor is the amber of whatever stops the work until you answer: a
	// form that can't be confirmed yet, a kill that takes the project along,
	// a quota about to run out. Never a decoration.
	warnColor = theme.Paint(theme.StateBlock, "")
	// selectedRowColor is the picked line of a list: raised background,
	// bright text. The bar on its left is what marks it with no color.
	selectedRowColor = theme.Paint(theme.FgBright, theme.BgRaised).Bold(true)
	selectedBar      = titleColor.Render("▌")
	// surfaceInk is the body of what has the focus — the focused cell, a
	// dialog: one step above the screen behind it. Nothing else paints a
	// background of its own.
	surfaceInk = theme.Tint(theme.Paint("", theme.BgSurface))
	// mutedInk is the content of a cell that doesn't hold the focus: the
	// agent's colors go quiet, the text stays readable. Frozen into a
	// terminal code because it's the app's most repeated paint job, once per
	// line of every cell that isn't the focused one.
	mutedInk = theme.Tint(mutedColor)
	// faintInk is the same content once a cell takes the keyboard: the rest
	// of the screen sinks into the dark.
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
// all come from theme.Do — no state wears a project's hue or the phosphor.
var markers = map[string]marker{
	"working":  fromTheme(theme.Working),
	"answered": fromTheme(theme.Answered),
	"approve":  fromTheme(theme.Approve),
	"down":     fromTheme(theme.Down),
	"stopped":  fromTheme(theme.Stopped),
	"orphan":   fromTheme(theme.Orphan),
	"idle":     fromTheme(theme.Idle),

	"interrupted": fromTheme(theme.Interrupted),
	"background":  fromTheme(theme.Background),
}

func fromTheme(state theme.State) marker {
	m := theme.Do(state)
	return marker{symbol: m.Glyph, label: m.Label, state: state, blocks: m.Inverted}
}

// color asks the theme every frame, instead of caching the finished style:
// the bar that blinks changes look between one frame and the next, and a
// style frozen at program start would never blink.
func (m marker) color() lipgloss.Style { return theme.Do(m.state).Style() }

// tone is the state's color with no filled area: what a counter uses, where
// the state is being tallied and not announced. The bar stays with the cell.
func (m marker) tone() lipgloss.Style { return theme.Paint(theme.Do(m.state).Color, "") }

func markerFor(state string) marker {
	if found, exists := markers[state]; exists {
		return found
	}
	return markers["stopped"]
}

// hint writes a shortcut: the key in its own weight, what it does in the
// secondary tone.
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
// it subtracts the status line and the box's borders.
func FullCellInner(width, height int) Dims {
	return Dims{Cols: max(width-2, 1), Rows: max(height-3, 1)}
}

// DrawFull puts one cell on the whole screen. It's like copying a block of
// text without grabbing its neighbors, and it's what the full-screen key
// does.
//
// The status line doesn't shrink with the view: who's calling is counted
// across the whole grid and the quota stays, since the cells left out of the
// drawing are exactly the ones this line has to speak for.
func DrawFull(state protocol.State, project protocol.Project, cell protocol.Cell, mode keyboard.Mode, width, height int, err string) string {
	lines := cellBox(cell, theme.ProjectHue(project.Color), mode, true, width, height-1)
	lines = append(lines, footerFor(state, &cell, mode, width, err, 0))
	return strings.Join(lines, "\n")
}

// keyFor is the name of the key that does this in this mode, as the keymap
// has it: the screen never writes a key's name by hand.
func keyFor(mode keyboard.Mode, action keyboard.Action) string {
	for _, shortcut := range keyboard.Shortcuts(mode) {
		if shortcut.Action == action {
			return shortcut.Key
		}
	}
	return ""
}

// statusLine is the one line of the screen that isn't a cell. On the left,
// what the line has to say right now — at rest, nothing but the key that
// opens the help. On the right, once each: who's calling across the whole
// grid, and the quota.
//
// What's being said to you outlives a figure: when the two don't fit, the
// quota goes first and the tally after it, and only then is the note cut —
// by whoever wrote it, when it knows which half matters, or at the edge.
func statusLine(note string, cut func(room int) string, figures []string, width int) string {
	for ; len(figures) > 0; figures = figures[:len(figures)-1] {
		right := strings.Join(figures, "   ")
		if room := width - lipgloss.Width(right) - 3; lipgloss.Width(note) <= room {
			return pad(pad(note, width-lipgloss.Width(right)-1)+right, width)
		}
	}
	if cut != nil && lipgloss.Width(note) > width {
		note = cut(width)
	}
	return pad(note, width)
}

// statusFigures is the right side of the status line, the last one being
// the first to go on a narrow screen. It opens with how many cells fell off
// the screen and the key that shows them all: the grid hides the drawing,
// never the count.
func statusFigures(state protocol.State, mode keyboard.Mode, hidden int) []string {
	var figures []string
	// Only while the key it names works: in TYPE every key is the cell's.
	if hidden > 0 && mode == keyboard.Browse {
		figures = append(figures, keyColor.Render("+"+strconv.Itoa(hidden))+
			mutedColor.Render(" off screen · ")+hint(keyFor(keyboard.Browse, keyboard.SwitchScreen)+" lists them"))
	}
	// Who's calling you — the only figure here that asks for action. Each
	// count wears its state's color, never its bar: the filled area belongs
	// to the cell that is blocked, and a tally can't compete with it.
	calls := countCalls(state)
	var tally []string
	for _, state := range []string{"answered", "approve"} {
		if calls[state] == 0 {
			continue
		}
		found := markerFor(state)
		paint := found.tone()
		if mode == keyboard.Type {
			paint = faintColor
		}
		tally = append(tally, paint.Render(found.symbol+" "+strconv.Itoa(calls[state])))
	}
	if len(tally) > 0 {
		figures = append(figures, strings.Join(tally, "  "))
	}

	if quota := state.Quota; quota != nil {
		// A figure and nothing else while there's room to spare; once the
		// window is about to run out it turns amber and says when it rolls
		// over. One window at a time: the 7-day one takes the place of the
		// 5-hour one only when it's the one about to run out.
		paint, figure := mutedColor, "quota "+strconv.Itoa(quota.Percent)+"%"
		if quota.Percent >= 80 {
			paint, figure = warnColor, figure+" · "+quota.Rollover
		}
		if quota.Weekly >= 80 && quota.Weekly > quota.Percent {
			paint, figure = warnColor, "7d "+strconv.Itoa(quota.Weekly)+"%"
		}
		if mode == keyboard.Type {
			paint = faintColor
		}
		figures = append(figures, paint.Render(figure))
	}
	return figures
}

// footer is the status line of a screen with nothing to offer: an error
// when there is one, and otherwise the single key that still means something
// — the help's while you command the application, the way out while a cell
// has the keyboard. A dialog lists its own keys, so under one it says
// nothing.
func footer(state protocol.State, mode keyboard.Mode, width int, err string) string {
	return statusLine(restNote(mode, err), nil, statusFigures(state, mode, 0), width)
}

func restNote(mode keyboard.Mode, err string) string {
	if err != "" {
		// The glyph is the dead state's: with no color, it's what tells an
		// error apart from the rest of the line.
		return errorColor.Render(" " + theme.Do(theme.Down).Glyph + " " + err)
	}
	switch mode {
	case keyboard.Browse:
		return " " + mutedColor.Render(keyFor(mode, keyboard.Help))
	case keyboard.Type:
		return " " + mutedColor.Render(keyFor(mode, keyboard.ExitType))
	}
	return ""
}

// waitingOn says what the focused cell is blocked on, when its agent said
// so itself: "tool", "question" or "plan". Empty for a cell that isn't
// blocked, one scrolled back into its history, and an approve read off the
// screen — nobody said what that one is asking.
func waitingOn(item *protocol.Cell) string {
	if item == nil || item.State != "approve" || !item.Live {
		return ""
	}
	return item.Ask
}

// footerFor is the status line of a screen with a focused cell. It says
// what that cell offers, and only while it does: the keys that settle a
// blocked cell — enter and esc mean something else here only while this line
// says so — where a conversation in the background went, the markdown its
// agent just wrote.
func footerFor(state protocol.State, item *protocol.Cell, mode keyboard.Mode, width int, err string, hidden int) string {
	figures := statusFigures(state, mode, hidden)
	if err != "" || mode != keyboard.Browse || item == nil {
		return statusLine(restNote(mode, err), nil, figures, width)
	}
	note := restNote(mode, "")
	var cut func(int) string
	switch ask := waitingOn(item); {
	case item.State == "background" && item.Live:
		// The cell shows the agent's list of sessions. Nothing typed from
		// here would reach the conversation, so the line says where it went
		// and the way back.
		note = mutedColor.Render(" "+theme.Do(theme.Background).Glyph+" The conversation runs in the background.") +
			"   " + hints("↵ enters the cell", "Esc there brings it back")
	case ask != "":
		keys := hint("↵ enters the cell to answer")
		if ask == "tool" {
			keys = hints("Enter approves", "Esc declines", "ctrl+l enters")
		}
		what := " " + theme.Do(theme.Approve).Glyph + " " + item.Detail
		// What the cell is waiting for gives way to the keys, never the
		// other way around.
		cut = func(room int) string {
			return warnColor.Render(shorten(what, max(room-lipgloss.Width(keys)-3, 3))) + "   " + keys
		}
		note = cut(lipgloss.Width(what) + lipgloss.Width(keys) + 3)
	case item.Doc != "":
		note += "   " + docHint(item.Doc)
	}
	return statusLine(note, cut, figures, width)
}

// emptyScreen is what shows up when the grid has no cell at all.
func emptyScreen(state protocol.State, mode keyboard.Mode, width, height int, err string) string {
	// Every empty state ends in a key.
	message := mutedColor.Render("No cells yet. ") + hint("n creates the first one.")
	if state.Notice != "" {
		message = mutedColor.Render(state.Notice)
	}
	// The empty grid is the only moment the brand fits whole on the screen
	// without stealing workspace.
	middle := append(theme.PaintedSymbol(), "", message)

	body := max(height-1, 1)
	top := max((body-len(middle))/2, 0)
	lines := make([]string, 0, height)
	for i := range body {
		if i >= top && i-top < len(middle) {
			lines = append(lines, pad(centerIn(middle[i-top], width), width))
			continue
		}
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(append(lines, footer(state, mode, width, err)), "\n")
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

// shorten cuts the text at the width and says it did: the last column
// becomes an ellipsis, so a sentence cut short never reads as a whole one.
func shorten(text string, width int) string {
	if widthOf(text) <= width {
		return text
	}
	return truncate(text, width-1) + "…"
}

// plain strips every escape sequence from the content and leaves the
// letters: the colors (CSI) and the string sequences too — a hyperlink is an
// OSC 8 around its text, and one left half-read shows its address as text.
// It's the first half of what makes only the focused cell stay lit: with
// many sections on screen, color turns into noise if everything shouts at
// once. Whoever calls it paints the result in a single tone.
func plain(text string) string {
	if !strings.Contains(text, "\x1b") {
		return text
	}
	return ansi.Strip(text)
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

// cellLabel is what shows on the top border: the tabs and the name. The
// active tab is the heavy one, which is what still tells it apart with no
// color; the others stay, a step down. A cell with no tabs says its type
// where the tabs would be. Whoever calls it brings the three paints.
func cellLabel(item protocol.Cell, name, tabOn, tabOff lipgloss.Style) string {
	// The gaps are painted too: on the approve bar the three paints are the
	// bar's own, and an unpainted gap would be a hole in it.
	if len(item.Tabs) == 0 {
		return tabOff.Render(" "+item.Type+"  ") + name.Render(item.Name+" ")
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
			label.WriteString(tabOn.Render(tab) + tabOff.Render("  "))
		// A sleeping tab keeps its place and says it's sleeping. It's not a
		// state of the cell — nothing there is asking for anything — so it
		// gets a dot, not a marker.
		case asleep[tab]:
			label.WriteString(tabOff.Render("·" + tab + "  "))
		default:
			label.WriteString(tabOff.Render(tab + "  "))
		}
	}
	label.WriteString(name.Render(item.Name + " "))
	return label.String()
}
