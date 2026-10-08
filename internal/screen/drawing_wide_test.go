package screen

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestTruncateCountsColumnsNotLetters — an emoji or a CJK letter takes two
// columns on screen. Cutting by letter count let the line run past its width.
func TestTruncateCountsColumnsNotLetters(t *testing.T) {
	for text, want := range map[string]string{
		"📁 src │ 💾 6/12G":                 "📁 src │ ",
		"日本語のテキスト":                        "日本語の",
		"ab日本":                            "ab日本",
		"\x1b[31m📁 red folder\x1b[0m end": "\x1b[31m📁 red fo\x1b[0m\x1b[0m",
	} {
		got := truncate(text, 9)
		if width := lipgloss.Width(got); width > 9 {
			t.Errorf("%q cut at 9 columns came out %d wide: %q", text, width, got)
		}
		if got != want && lipgloss.Width(text) > 9 {
			t.Errorf("%q cut at 9 columns: got %q, want %q", text, got, want)
		}
	}
	// A double-wide letter straddling the cut is left out whole, and pad
	// gives the column back.
	if got := pad("日本語", 5); got != "日本 " {
		t.Errorf("pad should fill the column a wide letter left: %q", got)
	}
}

// TestOverlayKeepsWideLinesAligned — a dialog over a line with wide letters
// keeps its border in the same column as on every other line, and the right
// side of the line stays where it was.
func TestOverlayKeepsWideLinesAligned(t *testing.T) {
	const width = 40
	row := func(text string) string { return "│" + pad(text, width-2) + "│" }
	background := strings.Join([]string{
		row("plain ascii line under the dialog"),
		row("📁 /tmp/w │ 💾 6/12G │ 🤖 Haiku 5.5 ✦"),
		row("日本語のテキストが背景にある行です"),
		row("x📁📁📁📁📁📁📁📁📁📁📁📁📁📁📁📁📁"),
	}, "\n")
	box := []string{"┌──────────┐", "│  dialog  │", "│  dialog  │", "└──────────┘"}

	for i, line := range strings.Split(overlay(background, box, width, 4), "\n") {
		if got := lipgloss.Width(line); got != width {
			t.Errorf("line %d came out %d columns wide, want %d: %q", i, got, width, line)
		}
		flat := plain(line)
		if !strings.HasSuffix(flat, "│") {
			t.Errorf("line %d lost its right border: %q", i, flat)
		}
		left := flat[:strings.Index(flat, box[i])]
		if got := lipgloss.Width(left); got != (width-12)/2 {
			t.Errorf("line %d: the dialog starts at column %d, want %d: %q", i, got, (width-12)/2, flat)
		}
	}
}
