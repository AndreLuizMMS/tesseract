package theme

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// greensAndCyans are the colors that must NOT become state. Green is
// keyboard ownership, cyan is structure.
var greensAndCyans = []string{
	BrandDeep, BrandCore, BrandLive, BrandPhosphor,
	FluxDeep, FluxCore, Flux,
}

func TestStateNeverUsesGreenOrCyan(t *testing.T) {
	for state, m := range stateMarkers {
		for _, forbidden := range greensAndCyans {
			if m.Color == forbidden {
				t.Fatalf("state %q uses %s, which belongs to the keyboard or structure", state, forbidden)
			}
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

	out := Do(Approve).Line(20) + Do(Answered).Line(0) + Mode(Type).Badge()
	if strings.Contains(out, "38;2;") || strings.Contains(out, "48;2;") {
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
