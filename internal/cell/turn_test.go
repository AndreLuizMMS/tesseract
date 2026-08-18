package cell

import "testing"

// testMarkers mimics an agent that announces when it's working and when
// it's waiting for an answer.
var testMarkers = Markers{
	Working:  []string{"esc to interrupt", "tokens)"},
	Question: []string{"No, and tell Claude what to do differently"},
}

// screenWithSpinner returns the screen of an idle agent, with the spinner at
// a different frame and the cursor somewhere else — the screen changes, but
// nothing is actually happening.
func screenWithSpinner(frame int) string {
	spin := []string{"⠋", "⠙", "⠹", "⠸"}[frame%4]
	cursor := []string{"█", " "}[frame%2]
	return "old conversation\n" + spin + " ready for the next request\n> " + cursor
}

// TestSpinnerDoesNotTriggerReply is the false-alarm rule: the screen
// changing on its own isn't work, so there's no turn to end.
func TestSpinnerDoesNotTriggerReply(t *testing.T) {
	turn := NewTurn(testMarkers)
	for frame := range 40 {
		if state := turn.Observe(screenWithSpinner(frame)); state == Replied {
			t.Fatalf("a blinking spinner triggered a reply at frame %d", frame)
		}
	}
}

// TestWorkFollowedBySilenceTriggersReply is the happy path.
func TestWorkFollowedBySilenceTriggersReply(t *testing.T) {
	turn := NewTurn(testMarkers)
	turn.Interact()

	for i := range readingsToArm {
		state := turn.Observe("writing the file…\nesc to interrupt")
		if state != Working {
			t.Fatalf("work reading %d should be working, got %q", i, state)
		}
	}

	for i := range readingsToEnd - 1 {
		if state := turn.Observe("done. should I cover the mobile menu too?"); state == Replied {
			t.Fatalf("declared the turn ended too early, at silence %d", i)
		}
	}
	if state := turn.Observe("done. should I cover the mobile menu too?"); state != Replied {
		t.Fatalf("after the full silence it should have replied, got %q", state)
	}
}

// TestShortWorkDoesNotArm — a blip of work doesn't count as a turn.
func TestShortWorkDoesNotArm(t *testing.T) {
	turn := NewTurn(testMarkers)
	turn.Interact()
	turn.Observe("esc to interrupt")
	for range 20 {
		if state := turn.Observe("nothing happening here"); state == Replied {
			t.Fatal("a blip of work can't turn into a reply")
		}
	}
}

// TestQuestionBecomesApproveNotReplied — an agent stuck on a question blocks
// work; an agent that finished the turn only has something to read.
func TestQuestionBecomesApproveNotReplied(t *testing.T) {
	turn := NewTurn(testMarkers)
	turn.Interact()
	for range readingsToArm {
		turn.Observe("editing…\nesc to interrupt")
	}

	question := "Do you want to make this edit?\n1. Yes\n2. No, and tell Claude what to do differently"
	for i := range readingsToEnd * 3 {
		state := turn.Observe(question)
		if state != Approve {
			t.Fatalf("reading %d: expected approve, got %q", i, state)
		}
	}

	// With the question answered, the agent goes back to working and the
	// turn continues.
	for range readingsToArm {
		turn.Observe("applying…\nesc to interrupt")
	}
	for range readingsToEnd {
		turn.Observe("done, applied.")
	}
	if state := turn.State(); state != Replied {
		t.Fatalf("after the question was answered and the work finished, expected replied, got %q", state)
	}
}

// TestSeenClearsTheCall — whoever gets read stops calling.
func TestSeenClearsTheCall(t *testing.T) {
	turn := NewTurn(testMarkers)
	turn.Interact()
	for range readingsToArm {
		turn.Observe("esc to interrupt")
	}
	for range readingsToEnd {
		turn.Observe("done")
	}
	if turn.State() != Replied {
		t.Fatalf("expected replied, got %q", turn.State())
	}
	turn.Seen()
	if state := turn.State(); state == Replied {
		t.Fatal("after looking at the cell, it can't keep calling")
	}
}

// TestAnyWorkMarkerWorks — the agent changes the text between versions, and
// the cell recognizes both.
func TestAnyWorkMarkerWorks(t *testing.T) {
	for _, marker := range []string{"esc to interrupt", "Cogitating… (4s · ↓ 18 tokens)"} {
		turn := NewTurn(testMarkers)
		turn.Interact()
		for range readingsToArm {
			if state := turn.Observe("working\n" + marker); state != Working {
				t.Fatalf("marker %q wasn't recognized", marker)
			}
		}
		for range readingsToEnd {
			turn.Observe("done")
		}
		if turn.State() != Replied {
			t.Fatalf("with marker %q the turn didn't end", marker)
		}
	}
}

// TestNoMarkerFallsBackToScreenChanging covers the agent that says nothing
// about itself.
func TestNoMarkerFallsBackToScreenChanging(t *testing.T) {
	turn := NewTurn(Markers{})
	turn.Interact()
	for i := range readingsToArm {
		if state := turn.Observe("line " + string(rune('a'+i))); state != Working {
			t.Fatalf("a changing screen is the only signal left: got %q", state)
		}
	}
	// The first "still screen" reading is still a screen change; the
	// silence starts counting from the next one.
	for range readingsToEnd + 1 {
		turn.Observe("still screen")
	}
	if state := turn.State(); state != Replied {
		t.Fatalf("a screen that stopped changing ends the turn, got %q", state)
	}
}

// TestAgentComingUpDoesNotBecomeAReply — the agent opening its own interface
// makes the screen change a lot, and that's not a reply to anyone: without a
// request, there's no turn.
func TestAgentComingUpDoesNotBecomeAReply(t *testing.T) {
	turn := NewTurn(Markers{})
	for frame := range 30 {
		turn.Observe("drawing the interface " + string(rune('a'+frame%26)))
	}
	for range readingsToEnd * 3 {
		turn.Observe("interface ready, waiting")
	}
	if state := turn.State(); state == Replied {
		t.Fatal("an agent that only came up can't appear as having replied")
	}

	// After a request, the next turn counts normally.
	turn.Interact()
	for range readingsToArm {
		turn.Observe("working " + string(rune('a'+turn.working)))
	}
	for range readingsToEnd + 1 {
		turn.Observe("done")
	}
	if state := turn.State(); state != Replied {
		t.Fatalf("after a request, the turn ends normally: %q", state)
	}
}
