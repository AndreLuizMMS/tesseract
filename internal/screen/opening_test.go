package screen

import (
	"io"
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/theme"
)

// symbol returns just one frame's drawing, with no color and no signature
// beside it — it's what the eye sees of the brand at that instant.
func symbol(a *Opening, tip *[2]int) []string {
	// The block is three spaces of indent, the symbol's seven characters and
	// then the signature. The cut is by width, not by separator: the
	// drawing's last line starts with a space and would fool a search for
	// spaces.
	width := len([]rune(theme.Symbol[0]))
	lines := a.blockLines(tip)[:len(theme.Symbol)]
	for i, line := range lines {
		drawing := []rune(strings.TrimPrefix(noStyle(line), "   "))
		if len(drawing) > width {
			drawing = drawing[:width]
		}
		lines[i] = strings.TrimRight(string(drawing), " ")
	}
	return lines
}

// TestOpeningStartsEmptyAndEndsWhole — the brand doesn't appear ready-made:
// it draws itself. The first frame has nothing, the last has the canonical
// symbol.
func TestOpeningStartsEmptyAndEndsWhole(t *testing.T) {
	a := NewOpening(io.Discard, false)

	for i, line := range symbol(a, nil) {
		if strings.TrimSpace(line) != "" {
			t.Fatalf("the starting frame should be empty, and line %d has %q", i, line)
		}
	}

	a.lightAll()
	for i, line := range symbol(a, nil) {
		if expected := strings.TrimRight(theme.Symbol[i], " "); line != expected {
			t.Fatalf("line %d of the final frame is %q, should be %q", i, line, expected)
		}
	}
}

// TestStrokePathCoversTheWholeSymbol — the pen's path can't leave a hole in
// the drawing or run through where there's no stroke.
func TestStrokePathCoversTheWholeSymbol(t *testing.T) {
	a := NewOpening(io.Discard, false)
	for _, point := range strokePath {
		if _, has := theme.SymbolColor(point[0], point[1]); !has {
			t.Fatalf("the stroke passes through %v, where there's no drawing", point)
		}
		a.lit[point] = true
	}
	a.lit[tesseraPosition] = true

	for i, line := range theme.Symbol {
		for j := range []rune(line) {
			if _, has := theme.SymbolColor(i, j); has && !a.lit[[2]int{i, j}] {
				t.Fatalf("position %v is never drawn by the stroke", [2]int{i, j})
			}
		}
	}
}

// TestPenTipOnlyLightsWhereItIs — it's the lit tip that gives the sense of a
// running pen; if it painted the whole drawing, there would be no motion.
func TestPenTipOnlyLightsWhereItIs(t *testing.T) {
	previous := theme.Current
	theme.Current = theme.FullColor
	defer func() { theme.Current = previous }()

	a := NewOpening(io.Discard, false)
	a.lightAll()

	tip := [2]int{0, 0}
	withTip := a.blockLines(&tip)[0]
	withoutTip := a.blockLines(nil)[0]
	if withTip == withoutTip {
		t.Fatal("the tip should paint differently from an already-settled stroke")
	}
	if noStyle(withTip) != noStyle(withoutTip) {
		t.Fatal("the tip is only color: the drawing can't change")
	}
}
