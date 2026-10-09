package theme

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// spokenFor are the colors that must NOT become state: the greens, of which
// the phosphor is where the typing lands, the brand mark's cyans, and every
// project hue in both strengths.
func spokenFor() []string {
	colors := []string{
		BrandDeep, BrandCore, BrandLive, BrandPhosphor,
		FluxDeep, FluxCore, Flux,
	}
	for _, hue := range hues {
		colors = append(colors, hue.Vivid, hue.Dim)
	}
	return colors
}

func TestStateNeverUsesAProjectHueOrThePhosphor(t *testing.T) {
	for state, m := range stateMarkers {
		for _, forbidden := range spokenFor() {
			if m.Color == forbidden {
				t.Fatalf("state %q uses %s, which belongs to a project, the keyboard or the brand mark", state, forbidden)
			}
		}
	}
}

// TestProjectHuesStayApart — a hue is whose cell it is, so no two of them
// may come out the same, in any strength, on any terminal: in full color,
// once the renderer has rounded them to 256, and on the six plain slots of
// ANSI 16. And none of them may come out as a state's color.
func TestProjectHuesStayApart(t *testing.T) {
	if len(hues) != HueCount {
		t.Fatalf("HueCount says %d and there are %d hues", HueCount, len(hues))
	}
	states := map[string]bool{}
	// The neutral ladder counts too: a hue that rounds to the gray of the
	// text beside it is a hue nobody sees.
	for _, token := range []string{StateRead, StateBlock, StateBlockDim, StateDead, BrandPhosphor, FgFaint, FgMuted, FgDefault, FgBright, LineDim} {
		states[token] = true
		states[indexed256(token)] = true
	}
	seen := map[string]string{}
	seen16 := map[string]bool{}
	for _, hue := range hues {
		for _, token := range []string{hue.Vivid, hue.Dim} {
			for _, shade := range []string{token, indexed256(token)} {
				if other, repeated := seen[shade]; repeated && other != token {
					t.Errorf("%s and %s come out as the same color (%s)", token, other, shade)
				}
				if states[shade] {
					t.Errorf("%s comes out as a state's color or as a neutral (%s)", token, shade)
				}
				seen[shade] = token
			}
		}
		slot := ansi16[hue.Vivid]
		if slot == "" || slot != ansi16[hue.Dim] {
			t.Errorf("%s: both strengths land on one ANSI 16 slot, got %q and %q", hue.Vivid, slot, ansi16[hue.Dim])
		}
		if seen16[slot] {
			t.Errorf("two hues land on ANSI %s", slot)
		}
		seen16[slot] = true
		// The plain half of the table is the projects'; the bright half is
		// state and the phosphor.
		if n, _ := strconv.Atoi(slot); n < 1 || n > 6 {
			t.Errorf("%s lands on ANSI %s, outside the six plain slots", hue.Vivid, slot)
		}
	}
	if ProjectHue(HueCount) != ProjectHue(0) || ProjectHue(-1) != ProjectHue(HueCount-1) {
		t.Error("a color number past the last hue wraps around")
	}
}

// indexed256 is the color a 256-color terminal gets for a token, as the
// renderer rounds it.
func indexed256(token string) string {
	return fmt.Sprint(colorprofile.ANSI256.Convert(lipgloss.Color(token)))
}

// TestFocusIsWeightWhereThereIsNoShade — with sixteen colors both strengths
// of a hue are one index, and with none there is no hue: what tells the
// focused cell's line apart there is that it is heavy.
func TestFocusIsWeightWhereThereIsNoShade(t *testing.T) {
	previous := Current
	defer func() { Current = previous }()
	for _, profile := range []Profile{FullColor, Color16, NoColor} {
		Current = profile
		hue := ProjectHue(0)
		on, off := hue.Line(true).Render("x"), hue.Line(false).Render("x")
		if on == off {
			t.Errorf("profile %d: the focused line reads the same as the others", profile)
		}
		if !strings.Contains(on, "\x1b[1") {
			t.Errorf("profile %d: the focused line should be heavy: %q", profile, on)
		}
	}
}

func TestOnlyApproveFillsTheWholeLine(t *testing.T) {
	for state, m := range stateMarkers {
		if m.Inverted != (state == Approve) {
			t.Fatalf("state %q: inverted=%v — only approve fills the line", state, m.Inverted)
		}
	}
}

func TestEachStateHasItsOwnGlyph(t *testing.T) {
	seen := map[string]State{}
	for state, m := range stateMarkers {
		if other, repeated := seen[m.Glyph]; repeated {
			t.Fatalf("glyph %q in %q and %q — with no color the two become the same signal", m.Glyph, state, other)
		}
		seen[m.Glyph] = state
	}
}

func TestNoColorDoesNotEmitColorEscape(t *testing.T) {
	previous := Current
	Current = NoColor
	defer func() { Current = previous }()

	out := Do(Approve).Style().Render("⏵") + Do(Answered).Style().Render("⬤") +
		Typing().Render(TypeTag) + ProjectHue(0).Line(true).Render("─") + Paint(FgFaint, "").Render("x")
	if strings.Contains(out, "38;") || strings.Contains(out, "48;") {
		t.Fatalf("NO_COLOR: a color escape leaked out in %q", out)
	}
	if !strings.Contains(out, "⏵") || !strings.Contains(out, "⬤") {
		t.Fatalf("NO_COLOR: the glyphs must survive — %q", out)
	}
}

func TestSymbolMaskCoversTheDrawing(t *testing.T) {
	if len(symbolMask) != len(Symbol) {
		t.Fatalf("the mask has %d lines and the symbol has %d", len(symbolMask), len(Symbol))
	}
	for i, line := range Symbol {
		if len([]rune(symbolMask[i])) != len([]rune(line)) {
			t.Fatalf("line %d: mask %q doesn't cover %q", i, symbolMask[i], line)
		}
	}
}

func TestPaintedSymbolDoesNotTouchTheDrawing(t *testing.T) {
	previous := Current
	Current = FullColor
	defer func() { Current = previous }()

	for i, line := range PaintedSymbol() {
		if clean := stripEscape(line); clean != Symbol[i] {
			t.Fatalf("line %d became %q, should still be %q", i, clean, Symbol[i])
		}
	}
}

func TestBlinkDoesNotRemoveTheFilledArea(t *testing.T) {
	previous, previousDimmed := Current, Dimmed
	Current = FullColor
	defer func() { Current, Dimmed = previous, previousDimmed }()

	Dimmed = false
	lit := Do(Approve).Style().Render("x")
	Dimmed = true
	dim := Do(Approve).Style().Render("x")

	if lit == dim {
		t.Fatal("the dim frame should differ from the lit one")
	}
	for name, out := range map[string]string{"lit": lit, "dim": dim} {
		if !strings.Contains(out, "48;") {
			t.Fatalf("%s frame lost its background — the bar has to stay a bar: %q", name, out)
		}
	}

	// On sixteen colors the bar is yellow in both frames: bright, then plain.
	Current = Color16
	Dimmed = false
	if lit := Do(Approve).Style().Render("x"); !strings.Contains(lit, "103") {
		t.Fatalf("the lit frame on 16 colors should fill with ANSI 11: %q", lit)
	}
	Dimmed = true
	if dim := Do(Approve).Style().Render("x"); !strings.Contains(dim, "43") {
		t.Fatalf("the dim frame on 16 colors should still be filled, with ANSI 3: %q", dim)
	}
}

// stripEscape strips the color codes, so the test looks only at the
// drawing.
func stripEscape(text string) string {
	var out strings.Builder
	inside := false
	for _, r := range text {
		switch {
		case r == 0x1b:
			inside = true
		case inside:
			if r >= 0x40 && r <= 0x7e && r != '[' {
				inside = false
			}
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

func TestEveryTokenHasA16ColorTarget(t *testing.T) {
	for _, m := range stateMarkers {
		if _, ok := ansi16[m.Color]; !ok {
			t.Fatalf("state color %s has no ANSI 16 equivalent", m.Color)
		}
	}
}

// TestInkPaintsTheSameAsTheStyle pins the freeze: the ink has to write
// exactly the same codes the style would write, or the screen changes look
// without anyone asking for it.
func TestInkPaintsTheSameAsTheStyle(t *testing.T) {
	for _, style := range []lipgloss.Style{
		Paint(FgFaint, ""),
		Paint(FgBright, BgRaised).Bold(true),
		Paint(BrandPhosphor, ""),
	} {
		ink := Tint(style)
		for _, text := range []string{"tesseract", "  spaces  ", "accentuation is great", "▸ ⬤ ⏵"} {
			if ink.Render(text) != style.Render(text) {
				t.Fatalf("ink diverged from style at %q: %q != %q", text, ink.Render(text), style.Render(text))
			}
		}
	}
}

// TestTokensAreTheBrandSheets pins every token to the brand sheet's value
// (style/tesseract-brand-identity.html, :root and // 04). A token that drifts
// here drifts on every screen at once.
func TestTokensAreTheBrandSheets(t *testing.T) {
	sheet := [][2]string{
		{BgVoid, "#030507"}, {BgBase, "#070B0C"}, {BgSurface, "#0C1315"}, {BgRaised, "#121C1F"},
		{LineDim, "#16282A"}, {LineActive, "#205047"},
		{FgFaint, "#3E534E"}, {FgMuted, "#6C8076"}, {FgDefault, "#BFD1C6"}, {FgBright, "#E8F4EC"},
		{BrandDeep, "#0B3322"}, {BrandCore, "#1F7A4C"}, {BrandLive, "#35C27A"}, {BrandPhosphor, "#55FFA6"},
		{FluxDeep, "#082F31"}, {FluxCore, "#128C86"}, {Flux, "#22E0D0"},
		{StateWorking, "#6C8076"}, {StateRead, "#7DB7E8"}, {StateBlock, "#FFB454"},
		{StateDead, "#FF3B47"}, {StateOff, "#3E534E"},
		{AnsiRed, "#C22F38"}, {AnsiYellow, "#C9A227"}, {AnsiBlue, "#3E7FA8"}, {AnsiMagenta, "#8B4FC4"},
	}
	for _, pair := range sheet {
		if pair[0] != pair[1] {
			t.Errorf("token is %s and the brand sheet says %s", pair[0], pair[1])
		}
	}
}

// TestAnsi16FollowsTheBrandTable — on a 16-color terminal every token has to
// land on the index whose color, in the brand's table (// 10), is that very
// token. The neutral shades have no index of their own and are the only ones
// allowed to share one.
func TestAnsi16FollowsTheBrandTable(t *testing.T) {
	table := map[string]string{
		"0": BgBase, "1": AnsiRed, "2": BrandCore, "3": AnsiYellow,
		"4": AnsiBlue, "5": AnsiMagenta, "6": FluxCore, "7": FgDefault,
		"8": FgFaint, "9": StateDead, "10": BrandPhosphor, "11": StateBlock,
		"12": StateRead, "14": Flux, "15": FgBright,
	}
	for index, token := range table {
		if ansi16[token] != index {
			t.Errorf("%s should land on ANSI %s, and lands on %q", token, index, ansi16[token])
		}
	}
	// The symbol on sixteen colors: color14, color2 and color10.
	if ansi16[Flux] != "14" || ansi16[BrandCore] != "2" || ansi16[BrandPhosphor] != "10" {
		t.Error("the 16-color symbol is color14 + color2 + color10")
	}
	// Nothing is ever written on a background of its own index.
	if ansi16[BgRaised] == ansi16[FgFaint] || ansi16[BgRaised] == ansi16[FgMuted] {
		t.Error("a raised background on the faint text's index swallows the text")
	}
	// The phosphor is where the typing lands: no project's line lands on its
	// index.
	for _, hue := range hues {
		if ansi16[hue.Vivid] == ansi16[BrandPhosphor] {
			t.Errorf("%s lands on the phosphor's index", hue.Vivid)
		}
	}
}

// TestSymbolIsColoredAsTheBrandDrawsIt — the character version, stroke by
// stroke: back square in cyan from the top left, front one in green, the
// tessera in phosphor.
func TestSymbolIsColoredAsTheBrandDrawsIt(t *testing.T) {
	sheet := []string{
		"cccccc ",
		"cvvvvcv",
		"cv f vc",
		"cvvvvcv",
		" vvvvvv",
	}
	for i, line := range sheet {
		if symbolMask[i] != line {
			t.Errorf("line %d of the mask is %q, the brand sheet draws %q", i, symbolMask[i], line)
		}
	}
	previous := Current
	Current = Color16
	defer func() { Current = previous }()
	painted := strings.Join(PaintedSymbol(), "")
	for _, code := range []string{"\x1b[96m", "\x1b[32m", "\x1b[1;92m"} {
		if !strings.Contains(painted, code) {
			t.Errorf("on 16 colors the symbol should use %q: %q", code, painted)
		}
	}
}

// TestScreenBackgroundFollowsTheMode — bg.base while the application has the
// keyboard, bg.void once a cell takes it, and nothing imposed with no color.
func TestScreenBackgroundFollowsTheMode(t *testing.T) {
	previous := Current
	defer func() { Current = previous }()

	Current = FullColor
	if ScreenBackground(Browse) != lipgloss.Color(BgBase) {
		t.Error("in BROWSE the screen sits on bg.base")
	}
	if ScreenBackground(Type) != lipgloss.Color(BgVoid) {
		t.Error("in TYPE the screen sinks to bg.void")
	}
	if CursorColor() != lipgloss.Color(BrandPhosphor) {
		t.Error("the cursor of the cell that holds the keyboard is phosphor")
	}
	// Sixteen colors are indexes of the terminal's own palette: no shade is
	// imposed over it. No color imposes nothing at all.
	for _, profile := range []Profile{Color16, NoColor} {
		Current = profile
		if ScreenBackground(Type) != nil || ScreenForeground() != nil || CursorColor() != nil {
			t.Errorf("profile %d: the terminal keeps its own background, text and cursor", profile)
		}
	}
}

// TestOneFrameForEveryBox — a thin line with rounded corners, whoever holds
// the keyboard. The mode is said by a word, not by a second kind of line.
func TestOneFrameForEveryBox(t *testing.T) {
	if Frame != [6]string{"╭", "╮", "╰", "╯", "─", "│"} {
		t.Errorf("every box is a thin line with rounded corners, got %q", Frame)
	}
}

// TestNoColorLeavesApproveAsTheOnlySolidArea — with no color at all, the
// approve bar is the one thing in reverse video. The word of the cell that
// holds the keyboard keeps its weight, not a filled background.
func TestNoColorLeavesApproveAsTheOnlySolidArea(t *testing.T) {
	previous := Current
	Current = NoColor
	defer func() { Current = previous }()

	if bar := Do(Approve).Style().Render("x"); !strings.Contains(bar, "\x1b[1;7m") && !strings.Contains(bar, "\x1b[7;1m") {
		t.Errorf("the approve bar should be in reverse video: %q", bar)
	}
	for name, out := range map[string]string{
		"type":     Typing().Render(TypeTag),
		"answered": Do(Answered).Style().Render("x"),
		"down":     Do(Down).Style().Render("x"),
		"line":     ProjectHue(0).Line(true).Render("x"),
	} {
		if strings.Contains(out, "7m") {
			t.Errorf("%s is in reverse video, and only approve may be: %q", name, out)
		}
	}
	if !strings.Contains(Typing().Render(TypeTag), "\x1b[1m") {
		t.Error("the word of the cell that holds the keyboard stays heavy")
	}
	// What the palette writes in its darkest shade is what's off: with no
	// color the terminal's own faint says it.
	for _, token := range []string{FgFaint, FgMuted, LineDim, ProjectHue(3).Dim} {
		if !strings.Contains(Paint(token, "").Render("x"), "\x1b[2m") {
			t.Errorf("%s is what isn't the focus: with no color it stays faint", token)
		}
	}
	for _, token := range []string{FgDefault, FgBright, ProjectHue(3).Vivid, StateRead} {
		if strings.Contains(Paint(token, "").Render("x"), "\x1b[2m") {
			t.Errorf("%s is lit, and with no color it came out faint", token)
		}
	}
}

// TestInFullColorTheTypingMarkIsNotAnArea — the phosphor is a word, never a
// filled badge: the approve bar is the only solid thing on the screen.
func TestInFullColorTheTypingMarkIsNotAnArea(t *testing.T) {
	previous := Current
	Current = FullColor
	defer func() { Current = previous }()
	if out := Typing().Render(TypeTag); strings.Contains(out, "48;") {
		t.Errorf("the TYPE word has a background: %q", out)
	}
}

// TestUnderBringsTheBackgroundBackAfterAReset — a background laid under a
// line someone else painted has to survive that line's own resets, and yield
// to a background the line sets for itself.
func TestUnderBringsTheBackgroundBackAfterAReset(t *testing.T) {
	previous := Current
	Current = FullColor
	defer func() { Current = previous }()

	ink := Tint(Paint("", BgSurface))
	ground := strings.SplitN(Paint("", BgSurface).Render("x"), "x", 2)[0]

	for _, reset := range []string{"\x1b[0m", "\x1b[m", "\x1b[49m", "\x1b[0;31m", "\x1b[39;49m"} {
		out := ink.Under("a" + reset + "b")
		if !strings.HasSuffix(strings.SplitN(out, "b", 2)[0], reset+ground) {
			t.Errorf("after %q the background should come back: %q", reset, out)
		}
	}
	// A color's own numbers aren't a reset: 38;5;0 is black text, and
	// 48;2;0;0;0 is a background the line chose.
	for _, color := range []string{"\x1b[38;5;0m", "\x1b[48;2;0;0;0m", "\x1b[38;2;0;49;0m", "\x1b[1m"} {
		out := ink.Under("a" + color + "b")
		if strings.Count(out, ground) != 1 {
			t.Errorf("%q is not a reset, and the background was laid again: %q", color, out)
		}
	}
	if stripEscape(ink.Under("a\x1b[31mb\x1b[0mc")) != "abc" {
		t.Error("the background can't touch the text")
	}

	Current = NoColor
	if out := Tint(Paint("", BgSurface)).Under("a\x1b[0mb"); out != "a\x1b[0mb" {
		t.Errorf("with no color nothing is laid under the line: %q", out)
	}
}
