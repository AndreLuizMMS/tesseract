package screen

import (
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// TestDocHintJoinsTheStatusLine — a markdown the agent wrote shows up as one
// more key on the last line, beside the help's, and only then.
func TestDocHintJoinsTheStatusLine(t *testing.T) {
	const width = 60
	var none protocol.State
	item := protocol.Cell{ID: "c1", State: "idle", Live: true}

	usual := footer(none, keyboard.Browse, width, "")
	if got := footerFor(none, &item, keyboard.Browse, width, "", 0); got != usual {
		t.Fatal("with no file, the status line is left alone")
	}
	item.Doc = "docs/plan.md"
	line := footerFor(none, &item, keyboard.Browse, width, "", 0)
	if got := noStyle(line); got != " ?   m reads plan.md" {
		t.Fatalf("the status line should name the key that reads the file: %q", got)
	}
	if widthOf(line) != width {
		t.Fatalf("the status line has to keep the screen's width, got %d", widthOf(line))
	}
	// While something else is being said — an error, a cell in TYPE — the
	// file waits.
	if got := noStyle(footerFor(none, &item, keyboard.Type, width, "", 0)); strings.Contains(got, "plan.md") {
		t.Fatalf("in TYPE the key isn't the application's: %q", got)
	}
	if got := noStyle(footerFor(none, &item, keyboard.Browse, width, "boom", 0)); strings.Contains(got, "plan.md") {
		t.Fatalf("an error has the line to itself: %q", got)
	}
}

// TestPickerOffersTheLastEditOnlyWhenThereIsOne — the file's key and its
// line in the picker exist with an edit to open, and ask for that cell's.
func TestPickerOffersTheLastEditOnlyWhenThereIsOne(t *testing.T) {
	found := protocol.IDEs{List: []protocol.IDE{{ID: "code", Label: "VS Code", Location: "wsl"}}}

	without := NewIDEPicker("p1", "api", &protocol.Cell{ID: "c1"})
	without.Received(found)
	if strings.Contains(plain(strings.Join(without.Draw(100), "\n")), "last edit") {
		t.Fatal("with no edit, the picker shouldn't offer one")
	}
	if request, open := without.Key("f"); request != nil || !open {
		t.Fatal("with no edit, the file key does nothing and the picker stays")
	}

	with := NewIDEPicker("p1", "api", &protocol.Cell{ID: "c1", Edited: "main.go:42"})
	with.Received(found)
	if drawn := plain(strings.Join(with.Draw(100), "\n")); !strings.Contains(drawn, "opens main.go:42") {
		t.Fatalf("the picker should name the file its key opens:\n%s", drawn)
	}
	request, open := with.Key("f")
	if request == nil || open || request.Cell != "c1" || request.ID != "code" {
		t.Fatalf("the file key should ask for the cell's last edit in the picked IDE: %+v", request)
	}
	if request, _ := with.Key("enter"); request == nil || request.Cell != "" {
		t.Fatalf("enter still opens the project: %+v", request)
	}
}

// TestPickerKeepsArrowsPressedWhileSearching — finding the IDEs takes a few
// seconds the first time. Arrows pressed meanwhile aren't thrown away: the
// list lands with the pick already moved, and never past its end.
func TestPickerKeepsArrowsPressedWhileSearching(t *testing.T) {
	list := protocol.IDEs{List: []protocol.IDE{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}, {ID: "c", Label: "C"}}}
	picker := NewIDEPicker("p1", "doxar-api", nil)
	picker.Key("down")
	picker.Received(list)
	if picker.Selected != 1 {
		t.Fatalf("one arrow pressed during the search should land on the second IDE, got %d", picker.Selected)
	}
	eager := NewIDEPicker("p1", "doxar-api", nil)
	for range 9 {
		eager.Key("down")
	}
	eager.Key("up")
	eager.Received(list)
	if eager.Selected != 2 {
		t.Fatalf("arrows past the end stop on the last IDE, got %d", eager.Selected)
	}
}
