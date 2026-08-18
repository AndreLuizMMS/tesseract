package screen

import (
	"os"
	"testing"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// TestShowcase isn't a test: it dumps painted screens to become images for
// the repository's showcase. Only runs with SHOWCASE pointing at a folder.
// Temporary.
func TestShowcase(t *testing.T) {
	dir := os.Getenv("SHOWCASE")
	if dir == "" {
		t.Skip("no SHOWCASE")
	}

	state := testGrid()
	state.Quota = &protocol.Quota{Percent: 21, Rollover: "3:12"}

	screens := map[string]string{
		"grid":    Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Browse, 118, 24, ""),
		"digitar": Draw(state, Focus{Project: 0, Cell: 0}, keyboard.Type, 118, 24, ""),
	}
	for name, drawing := range screens {
		if err := os.WriteFile(dir+"/"+name+".ansi", []byte(drawing), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
