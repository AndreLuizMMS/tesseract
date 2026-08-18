package engine

import (
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/protocol"
)

// TestStillGridProducesNoFrame pins down the engine's silence: with the
// watcher up and no cell producing anything, the version doesn't climb — and
// with no new version, no screen gets a snapshot to redraw.
func TestStillGridProducesNoFrame(t *testing.T) {
	m := testEngine(t)
	if _, err := m.Create(protocol.Create{Path: t.TempDir(), Type: "bash"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	go m.Watch()

	// Wait for the shell to finish writing its prompt: while it's talking,
	// climbing the version is correct. Three quiet windows in a row is
	// enough to know it stopped, not just paused mid-output.
	quiet := 0
	for range 40 {
		before := m.Version()
		time.Sleep(200 * time.Millisecond)
		if before == m.Version() {
			quiet++
		} else {
			quiet = 0
		}
		if quiet == 3 {
			break
		}
	}
	if quiet < 3 {
		t.Fatal("the shell never settled down")
	}

	before := m.Version()
	time.Sleep(1500 * time.Millisecond)
	if after := m.Version(); after != before {
		t.Fatalf("still grid generated %d frame(s) for nothing", after-before)
	}
}
