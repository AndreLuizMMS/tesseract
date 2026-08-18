// Package theme is the only place in Tesseract where color turns into
// meaning. Outside of here there's no loose hex: whoever draws asks for the
// token by name, and this package is the one that decides what goes out to
// the terminal.
//
// Three rules govern the palette and none of them is aesthetic:
//
//   - green is KEYBOARD OWNERSHIP. Never state. #55FFA6 (BrandPhosphor)
//     shows up at most once per screen — on the cell that holds its
//     keyboard, and nowhere else.
//   - cyan is STRUCTURE. Never state. Grid, corners, numbering, labels.
//   - state never uses green or cyan, and urgency is filled area, not hue:
//     "approve" comes out as a solid, inverted bar across the whole line,
//     while "answered" is just a glyph.
//
// No glow, no scanline, no chromatic aberration: those effects exist on the
// brand surface (README, site, banner), never inside the terminal. No
// ligature, no emoji, no rounded corner.
package theme

import (
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// Tokens. Single source of truth — no other file in the project writes hex.
// ---------------------------------------------------------------------------

// Base: the ten shades of background, line and text.
const (
	BgVoid     = "#030507" // TYPE mode background
	BgBase     = "#070B0C" // default background
	BgSurface  = "#0C1315" // cell body
	BgRaised   = "#121C1F" // header, selection
	LineDim    = "#16282A" // unfocused grid
	LineActive = "#205047" // grid of the focused project
	FgFaint    = "#3E534E" // off, shortcuts
	FgMuted    = "#6C8076" // secondary text
	FgDefault  = "#BFD1C6" // primary text
	FgBright   = "#E8F4EC" // titles
)

// Green neon: keyboard ownership. Never state.
const (
	BrandDeep     = "#0B3322"
	BrandCore     = "#1F7A4C" // grid of the active project, logo
	BrandLive     = "#35C27A" // focused cell
	BrandPhosphor = "#55FFA6" // keyboard owner — one per screen, always
)

// Cyan neon: structure. Never state.
const (
	FluxDeep = "#082F31"
	FluxCore = "#128C86" // grid, corners, labels
	Flux     = "#22E0D0" // glyph, numbering, second dimension
)

// States: no green, no cyan.
const (
	StateWorking = "#6C8076" // ▸ working
	StateRead    = "#7DB7E8" // ⬤ answered
	StateBlock   = "#FFB454" // ⏵ approve
	StateDead    = "#FF3B47" // ✖ down
	StateOff     = "#3E534E" // ○ stopped
	StateOrphan  = "#C77DFF" // ⚠ orphan
)

// The four ANSI 16 colors that have no role of their own in the palette.
// They exist for the handful of places that need more distinct hues than the
// roles offer — the project color band, for example — without falling back
// on green or cyan, which are spoken for.
const (
	AnsiRed     = "#C22F38" // ANSI 1
	AnsiYellow  = "#C9A227" // ANSI 3
	AnsiBlue    = "#3E7FA8" // ANSI 4
	AnsiMagenta = "#8B4FC4" // ANSI 5
)

// Glyph is the brand's single-character glyph, for spots that only fit one
// character: prompt, window title, bar.
const Glyph = "⧉"

// Symbol is the brand mark in characters, 7×5 — the version that lives
// inside the product. Two squares offset on the diagonal and a lit tessera
// in the middle of the overlap.
// Every line has the same width, with trailing space where the drawing ends
// early — otherwise whoever centers line by line skews the symbol.
var Symbol = []string{
	"┌────┐ ",
	"│┌───┼┐",
	"││ ▓ ││",
	"└┼───┘│",
	" └────┘",
}

// symbolMask says who owns each character of the drawing: v is the front
// square, c is the back one, f is the tessera, space paints nothing. Where
// the two squares cross, the front one wins — that's what "being in front"
// means.
var symbolMask = []string{
	"vvvvvv ",
	"vccccvc",
	"vc f vc",
	"vvvvvvc",
	" cccccc",
}

// SymbolColor is the style of the symbol's character at that position: the
// back square in flux, the front one in brand.core, the tessera in
// brand.phosphor. The second return says whether there's a drawing there —
// an empty position doesn't get painted.
func SymbolColor(row, col int) (lipgloss.Style, bool) {
	if row < 0 || row >= len(symbolMask) {
		return lipgloss.NewStyle(), false
	}
	mask := []rune(symbolMask[row])
	if col < 0 || col >= len(mask) {
		return lipgloss.NewStyle(), false
	}
	switch mask[col] {
	case 'v':
		return Paint(BrandCore, ""), true
	case 'c':
		return Paint(Flux, ""), true
	case 'f':
		return Paint(BrandPhosphor, "").Bold(true), true
	}
	return lipgloss.NewStyle(), false
}

// PaintedSymbol is the 7×5 mark already colored. With no color, it returns
// the plain drawing — it was made to read fine with no color at all.
func PaintedSymbol() []string {
	if Current == NoColor {
		return append([]string(nil), Symbol...)
	}
	lines := make([]string, len(Symbol))
	for i, line := range Symbol {
		var out strings.Builder
		for j, r := range []rune(line) {
			paint, has := SymbolColor(i, j)
			if !has {
				out.WriteRune(r)
				continue
			}
			out.WriteString(paint.Render(string(r)))
		}
		lines[i] = out.String()
	}
	return lines
}

// Tagline, Name and Version are the signature that goes with the symbol in
// the banner.
const (
	Tagline = "the mosaic never falls apart"
	Name    = "T E S S E R A C T"
	Version = "ts 0.1.0 // MIT"
)

// ---------------------------------------------------------------------------
// Color profile: 24 bit, 16 colors, or none.
// ---------------------------------------------------------------------------

// Profile is how much color the terminal accepts right now.
type Profile int

const (
	// FullColor: 24 bit, the hex codes go out as they are.
	FullColor Profile = iota
	// Color16: only ANSI 16, each token falls onto an ansi16 index.
	Color16
	// NoColor: NO_COLOR=1 or TERM=dumb. Only bold, reverse and underline.
	NoColor
)

// ansi16 is the destination of each token when the terminal only has 16
// colors.
//
// Two pairs collide on purpose, because ANSI 16 has no in-between shade:
// StateWorking and StateOff both land on 8. That's why the glyph (▸ against
// ○) is mandatory: the state alphabet has to read fine with no color at all.
var ansi16 = map[string]string{
	BgVoid:     "0",
	BgBase:     "0",
	BgSurface:  "0",
	BgRaised:   "8",
	LineDim:    "8",
	LineActive: "2",
	FgFaint:    "8",
	FgMuted:    "8",
	FgDefault:  "7",
	FgBright:   "15",

	BrandDeep:     "2",
	BrandCore:     "2",
	BrandLive:     "2",
	BrandPhosphor: "10",

	FluxDeep: "6",
	FluxCore: "6",
	Flux:     "14",

	StateRead:   "12",
	StateBlock:  "11",
	StateDead:   "9",
	StateOrphan: "13",

	AnsiRed:     "1",
	AnsiYellow:  "3",
	AnsiBlue:    "4",
	AnsiMagenta: "5",
}

// Current is the profile currently in effect. It's a variable so the test
// can pin it.
var Current = Detect()

// Detect reads the environment and decides the profile. NO_COLOR beats
// everything — it's enough for the variable to exist, its value doesn't
// matter (no-color.org).
func Detect() Profile {
	if _, has := os.LookupEnv("NO_COLOR"); has {
		return NoColor
	}
	term := os.Getenv("TERM")
	if term == "dumb" {
		return NoColor
	}
	if strings.Contains(term, "16color") {
		return Color16
	}
	// Outside these two declared cases, the theme hands out full color and
	// lets lipgloss downgrade to whatever the terminal can take — it already
	// knows how to do that, and guessing here would just create a second
	// source of truth.
	return FullColor
}

// Ink is a style already translated into the codes the terminal receives:
// what comes before the text and what comes after. Asking lipgloss to
// re-render costs copying the whole style on every call, and the mosaic
// paints hundreds of lines per frame — the same pair of codes, repeated.
type Ink struct{ before, after string }

// Tint freezes a style into ink. Only good for color style: whoever needs
// width, border or padding needs the full lipgloss style.
func Tint(style lipgloss.Style) Ink {
	before, after, _ := strings.Cut(style.Render("\x00"), "\x00")
	return Ink{before: before, after: after}
}

// Render paints a line. Empty text comes out empty: painting nothing would
// just spend bytes on the screen.
func (t Ink) Render(text string) string {
	if text == "" {
		return ""
	}
	return t.before + text + t.after
}

// Paint returns the style with foreground and background already resolved
// for the current profile. An empty background means "don't paint a
// background".
func Paint(fg, bg string) lipgloss.Style {
	e := lipgloss.NewStyle()
	switch Current {
	case NoColor:
		return e
	case Color16:
		if c, ok := ansi16[fg]; ok {
			e = e.Foreground(lipgloss.Color(c))
		}
		if c, ok := ansi16[bg]; ok {
			e = e.Background(lipgloss.Color(c))
		}
	default:
		if fg != "" {
			e = e.Foreground(lipgloss.Color(fg))
		}
		if bg != "" {
			e = e.Background(lipgloss.Color(bg))
		}
	}
	return e
}

// ScreenBackground and ScreenForeground are the background and text the
// application imposes on the terminal while it's open. The background is
// BgVoid — the deepest color in the palette — because the grid only reads
// well when the black behind it is really black. With NO_COLOR nothing is
// imposed: they return nil, and the terminal stays as it was.
func ScreenBackground() color.Color {
	if Current == NoColor {
		return nil
	}
	return resolve(BgVoid)
}

func ScreenForeground() color.Color {
	if Current == NoColor {
		return nil
	}
	return resolve(FgDefault)
}

// WithBackground adds a background to a style that's already painted. With
// no color, it returns the style as-is — a painted background doesn't
// survive NO_COLOR.
func WithBackground(e lipgloss.Style, bg string) lipgloss.Style {
	switch Current {
	case NoColor:
		return e
	case Color16:
		return e.Background(lipgloss.Color(ansi16[bg]))
	default:
		return e.Background(lipgloss.Color(bg))
	}
}

// ---------------------------------------------------------------------------
// State alphabet.
// ---------------------------------------------------------------------------

// State is the cell's situation, the way it shows up on the marker.
type State string

const (
	Working  State = "working"
	Answered State = "answered"
	Approve  State = "approve"
	Down     State = "down"
	Stopped  State = "stopped"
	Orphan   State = "orphan"
)

// Marker is how a state shows itself: a glyph, a label, a color and a
// style. Inverted marks the state that fills the whole line instead of just
// becoming a little sign.
type Marker struct {
	Glyph    string
	Label    string
	Color    string
	Inverted bool
	// Dim draws the marker faint. It exists because StateWorking and
	// StateOff collide on the same ANSI 16 index.
	Dim bool
}

// >>> state-map
// No color in this map may be green or cyan — scripts/check-theme.sh fails
// the build if someone tries. Green is keyboard ownership, cyan is
// structure.
var stateMarkers = map[State]Marker{
	Working:  {Glyph: "▸", Label: "WORKING", Color: StateWorking},
	Answered: {Glyph: "⬤", Label: "ANSWERED", Color: StateRead},
	Approve:  {Glyph: "⏵", Label: "APPROVE", Color: StateBlock, Inverted: true},
	Down:     {Glyph: "✖", Label: "DOWN", Color: StateDead},
	Stopped:  {Glyph: "○", Label: "STOPPED", Color: StateOff, Dim: true},
	Orphan:   {Glyph: "⚠", Label: "ORPHAN", Color: StateOrphan},
}

// <<< state-map

// Do returns the marker for the state. An unknown state falls back to
// Stopped, which is the most harmless state: nothing happening.
func Do(s State) Marker {
	if m, ok := stateMarkers[s]; ok {
		return m
	}
	return stateMarkers[Stopped]
}

// Dimmed is the dim frame of the blinking blocking state. The screen is
// what drives it, on a 1.8s-on/200ms-off clock.
var Dimmed bool

// BlinkOn says whether the blocking state's bar should blink. With no
// color the blink doesn't happen: inverted video is the only signal left,
// and turning it off now and then would remove the filled area, which is
// exactly what tells "approve" apart from "answered".
func BlinkOn() bool {
	if Current == NoColor {
		return false
	}
	_, off := os.LookupEnv("TESSERACT_NO_BLINK")
	return !off
}

// Style is how the marker paints itself. Inverted becomes a solid bar:
// background in the state's color, text in the deepest background. With no
// color, it truly inverts — the terminal's reverse doesn't need a palette.
//
// In the dim frame the bar swaps background, but it's still a bar: the
// filled area is what says "this blocks the work", and it never disappears.
// What blinks is the intensity, not the presence.
func (m Marker) Style() lipgloss.Style {
	if m.Inverted {
		if Current == NoColor {
			return lipgloss.NewStyle().Reverse(true).Bold(true)
		}
		if Dimmed {
			return Paint(m.Color, BgRaised).Bold(true)
		}
		return Paint(BgVoid, m.Color).Bold(true)
	}
	e := Paint(m.Color, "")
	if m.Dim {
		e = e.Faint(true)
	}
	return e
}

// Line draws the marker. The blocking state fills the whole width, because
// urgency here is filled area, not hue; the others are just the glyph and
// the label. Width less than or equal to zero returns the short version.
func (m Marker) Line(width int) string {
	text := m.Glyph + " " + m.Label
	if m.Inverted && width > 0 {
		return m.Style().Width(width).Render(" " + text + " ")
	}
	return m.Style().Render(text)
}

// ---------------------------------------------------------------------------
// The two modes.
// ---------------------------------------------------------------------------

// Mode is who owns the keyboard. There's never two owners at once.
type Mode int

const (
	// Browse: every key belongs to the application. Simple border.
	Browse Mode = iota
	// Type: every key belongs to the cell. Double border, deeper
	// background and a badge.
	Type
)

// TypeBadge is the inverted badge that only shows up in TYPE mode.
const TypeBadge = "▓ TYPE ▓"

// Border is the mode's border. No rounded corner anywhere.
func (m Mode) Border() lipgloss.Border {
	if m == Type {
		return lipgloss.DoubleBorder()
	}
	return lipgloss.NormalBorder()
}

// Background is the mode's background. TYPE mode darkens the whole screen:
// it's the first sign that the application went silent.
func (m Mode) Background() string {
	if m == Type {
		return BgVoid
	}
	return BgBase
}

// Badge returns the mode's badge already painted. In BROWSE there's no
// badge.
func (m Mode) Badge() string {
	if m != Type {
		return ""
	}
	if Current == NoColor {
		return lipgloss.NewStyle().Reverse(true).Bold(true).Render(" " + TypeBadge + " ")
	}
	return Paint(BgVoid, BrandPhosphor).Bold(true).Render(" " + TypeBadge + " ")
}

// Cell returns the cell's border style. The phosphor green comes out of
// here and only here: it's the cell that holds the keyboard, and there's
// one per screen.
func Cell(m Mode, focused bool) lipgloss.Style {
	e := lipgloss.NewStyle().Border(m.Border())
	if Current == NoColor {
		// With no color, mode and focus stay legible: double border against
		// simple border, and bold on the cell that holds the keyboard.
		return e.Bold(focused)
	}
	switch {
	case focused && m == Type:
		return e.BorderForeground(resolve(BrandPhosphor)).Bold(true)
	case focused:
		return e.BorderForeground(resolve(BrandLive))
	default:
		return e.BorderForeground(resolve(LineDim))
	}
}

// Grid returns the project line's style: deep green when it's the focused
// project, dim otherwise.
func Grid(focused bool) lipgloss.Style {
	if focused {
		return Paint(LineActive, "")
	}
	return Paint(LineDim, "")
}

// Structure is everything that's frame and numbering: cyan, always.
func Structure() lipgloss.Style { return Paint(FluxCore, "") }

// Numbering is the grid's second dimension — project index, cell counter.
func Numbering() lipgloss.Style { return Paint(Flux, "") }

// resolve resolves a token for the current profile, for places that need a
// bare color instead of a full style — the border, for instance. Must not
// be called with the NoColor profile: the caller decides beforehand
// whether it paints at all.
func resolve(token string) color.Color {
	if Current == Color16 {
		return lipgloss.Color(ansi16[token])
	}
	return lipgloss.Color(token)
}
