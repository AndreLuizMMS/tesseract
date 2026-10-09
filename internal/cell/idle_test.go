package cell

import "testing"

// TestIdleNeverTakesDownAnUnreadAnswer — "idle" is the agent saying nothing
// is going on at its prompt. An answer nobody read is still waiting; only
// someone looking at the cell takes that marker down, and what's left then
// is an idle agent, not a working one.
func TestIdleNeverTakesDownAnUnreadAnswer(t *testing.T) {
	turn := NewTurn(claudeMarkers)
	turn.Link(true)

	turn.Signal(Idle, 1)
	if state := turn.State(); state != Idle {
		t.Fatalf("a session at its prompt should be idle, got %q", state)
	}
	turn.Signal(Replied, 2)
	turn.Signal(Idle, 3)
	if state := turn.State(); state != Replied {
		t.Fatalf("the unread answer was taken down by an idle report: %q", state)
	}
	turn.Seen()
	if state := turn.State(); state != Idle {
		t.Fatalf("a read answer leaves an idle agent, got %q", state)
	}

	// A turn the user stopped stays said until the next one starts.
	turn.Signal(Interrupted, 4)
	turn.Seen()
	if state := turn.State(); state != Interrupted {
		t.Fatalf("looking at the cell shouldn't erase the interruption: %q", state)
	}
	turn.Signal(Working, 5)
	if state := turn.State(); state != Working {
		t.Fatalf("the next turn takes over, got %q", state)
	}

	// With no mod, reading an answer goes back to the word the screen
	// reading always used.
	scraped := NewTurn(claudeMarkers)
	scraped.state = Replied
	scraped.Seen()
	if state := scraped.State(); state != Working {
		t.Fatalf("without a mod the read answer is %q", state)
	}
}
