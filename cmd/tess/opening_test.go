package main

import "testing"

// TestOpeningOnlyAnimatesOverAnEmptyGrid — coming back to cells that were
// already there never waits on the drawing, whatever the terminal is.
func TestOpeningOnlyAnimatesOverAnEmptyGrid(t *testing.T) {
	for _, width := range []int{0, 200} {
		if animateOpening(width, 1) {
			t.Fatalf("a grid with a project to restore animated the opening at width %d", width)
		}
	}
}
