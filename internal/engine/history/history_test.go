package history

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiscardStartKeepsEnd writes well past the cap and checks that the
// file lost its start and kept its end.
func TestDiscardStartKeepsEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cell.log")
	const cap int64 = 4096
	h, err := Open(path, cap)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer h.Close()

	if _, err := h.Write([]byte("FIRST LINE OF ALL\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	filler := bytes.Repeat([]byte("x"), 512)
	for range 40 {
		if _, err := h.Write(append(filler, '\n')); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if _, err := h.Write([]byte("LAST LINE OF ALL\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if size := h.Size(); size > cap {
		t.Fatalf("history ended up with %d bytes, above the cap of %d", size, cap)
	}

	if matches, err := h.Search("FIRST"); err != nil {
		t.Fatalf("search: %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("the start should have been discarded, but it's still there: %#v", matches)
	}
	matches, err := h.Search("LAST LINE")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("the end had to be preserved, found %d matches", len(matches))
	}
}

// TestSearchReturnsCorrectLine checks the line number and the stripping of
// terminal codes.
func TestSearchReturnsCorrectLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cell.log")
	h, err := Open(path, DefaultCap)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer h.Close()

	if _, err := h.Write([]byte("one\ntwo\n\x1b[32mthree tesseract\x1b[0m\nfour\nfive TESSERACT\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	matches, err := h.Search("tesseract")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %#v", len(matches), matches)
	}
	if matches[0].Line != 3 || matches[0].Text != "three tesseract" {
		t.Fatalf("first match wrong: %#v", matches[0])
	}
	if matches[1].Line != 5 || matches[1].Text != "five TESSERACT" {
		t.Fatalf("second match wrong: %#v", matches[1])
	}
}

// TestTailReplaysTheEnd is what lets the cell come back to life with its
// previous screen.
func TestTailReplaysTheEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cell.log")
	h, err := Open(path, DefaultCap)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer h.Close()

	if _, err := h.Write([]byte("start\nend of screen\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	tail, err := h.Tail(14)
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if string(tail) != "end of screen\n" {
		t.Fatalf("tail came out %q", tail)
	}
}

// TestContinuesTheFileThatAlreadyExists ensures reopening doesn't erase the
// past.
func TestContinuesTheFileThatAlreadyExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cell.log")
	h, err := Open(path, DefaultCap)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := h.Write([]byte("before the crash\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	h.Close()

	h2, err := Open(path, DefaultCap)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer h2.Close()
	if _, err := h2.Write([]byte("after the crash\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	matches, err := h2.Search("crash")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected both lines, got %#v", matches)
	}
}

func TestStripCodes(t *testing.T) {
	cases := map[string]string{
		"\x1b[1;32m$ ls\x1b[0m":       "$ ls",
		"\x1b]0;title\x07prompt":      "prompt",
		"line\roverwritten":           "lineoverwritten",
		"Grüße naïve über":            "Grüße naïve über",
		"\x1b[2J\x1b[Hcleared screen": "cleared screen",
	}
	for raw, expected := range cases {
		if got := StripCodes(raw); got != expected {
			t.Errorf("StripCodes(%q) = %q, expected %q", raw, got, expected)
		}
	}
	if strings.Contains(StripCodes("\x1b[31mred"), "\x1b") {
		t.Error("terminal code left over in the cleaned text")
	}
}

// TestSearchInLargeFile covers the finishing-touch case: the history went
// past the cap, got cut, and search still finds what remains — with the
// correct line number in what's left.
func TestSearchInLargeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cell.log")
	const cap int64 = 256 << 10
	h, err := Open(path, cap)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer h.Close()

	// A lot of volume, with a needle at the start (which will be discarded)
	// and another at the end (which has to survive).
	if _, err := h.Write([]byte("NEEDLE-AT-THE-START\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	line := strings.Repeat("common build output ", 8) + "\n"
	for i := range 4000 {
		if i == 2000 {
			if _, err := h.Write([]byte("NEEDLE-IN-THE-MIDDLE of the volume\n")); err != nil {
				t.Fatalf("write: %v", err)
			}
		}
		if _, err := h.Write([]byte(line)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if _, err := h.Write([]byte("NEEDLE-AT-THE-END\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if h.Size() > cap {
		t.Fatalf("history went past the cap: %d", h.Size())
	}

	if matches, err := h.Search("NEEDLE-AT-THE-START"); err != nil {
		t.Fatalf("search: %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("the start was discarded, it couldn't show up: %#v", matches)
	}

	matches, err := h.Search("NEEDLE-AT-THE-END")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("the end needle had to be found once, got %d", len(matches))
	}

	// The line number matches the file's current line count.
	total, err := h.Lines()
	if err != nil {
		t.Fatalf("count lines: %v", err)
	}
	if matches[0].Line < 1 || matches[0].Line > total {
		t.Fatalf("line %d out of the %d-line file", matches[0].Line, total)
	}
	if matches[0].Line != total {
		t.Fatalf("the last line written had to be the last of the file: %d of %d", matches[0].Line, total)
	}
}

// TestSearchAcrossTheCut — the term that was on the discard boundary
// doesn't show up cut in half.
func TestSearchAcrossTheCut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cell.log")
	const cap int64 = 8 << 10
	h, err := Open(path, cap)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer h.Close()

	for range 20 {
		if _, err := h.Write([]byte(strings.Repeat("x", 500) + "CUT-WORD\n")); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	matches, err := h.Search("CUT-WORD")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("the matches that survived the cut had to be found")
	}
	for _, match := range matches {
		if !strings.Contains(match.Text, "CUT-WORD") {
			t.Fatalf("match without the term: %#v", match)
		}
	}
}
