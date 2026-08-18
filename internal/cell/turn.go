package cell

import (
	"crypto/sha256"
	"strings"
	"sync"
)

// An agent that says when it's working is taken at its word. Only the ones
// that say nothing get judged by the screen changing — and then a blinking
// spinner or a moving cursor would count as work, which is where the false
// alarm comes from.
const (
	// readingsToArm is how many consecutive readings of work the cell needs
	// before "done" counts as a reply.
	readingsToArm = 3
	// readingsToEnd is how many consecutive readings of silence the cell
	// needs before the turn counts as ended. At half a second per reading,
	// that's three seconds — enough to cover the pause an agent takes in
	// the middle of an answer.
	readingsToEnd = 6
)

// Markers is what an agent writes on its own interface: Working while the
// turn is in progress, Question while it waits for a yes or no.
//
// They're lists because agents change this text between versions, and
// because the same cell needs to recognize the old and the new one without
// choosing.
type Markers struct {
	Working  []string
	Question []string
}

// anyPresent says whether any of the texts shows up on the screen, ignoring
// case.
func anyPresent(screen string, texts []string) bool {
	if len(texts) == 0 {
		return false
	}
	lower := strings.ToLower(screen)
	for _, text := range texts {
		if text == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(text)) {
			return true
		}
	}
	return false
}

// Turn follows the state of whoever's conversing. Turn state only makes
// sense for an agent: a shell doesn't "reply".
type Turn struct {
	mu         sync.Mutex
	markers    Markers
	screenHash [sha256.Size]byte
	working    int
	silence    int
	armed      bool
	// interacted marks that someone has already asked this agent for
	// something. Before that there's no turn to end: what the screen shows
	// is the agent opening its own interface, and that's not a reply to
	// anyone.
	interacted bool
	state      State
}

// NewTurn prepares tracking for an agent with its markers.
func NewTurn(markers Markers) *Turn {
	return &Turn{markers: markers, state: Working}
}

// Observe takes the cell's screen and returns what state it's in.
func (t *Turn) Observe(screen string) State {
	t.mu.Lock()
	defer t.mu.Unlock()

	changed := t.screenChanged(screen)
	question := anyPresent(screen, t.markers.Question)

	working := changed
	if len(t.markers.Working) > 0 {
		working = anyPresent(screen, t.markers.Working)
	}

	if working && !t.interacted {
		// The agent coming up and drawing itself isn't a turn. Without
		// this, opening a tab would already light up the replied marker.
		t.state = Working
		return t.state
	}

	if working {
		t.working++
		t.silence = 0
		if t.working >= readingsToArm {
			t.armed = true
		}
		// Work started again: whatever was unread lost its validity, and
		// nothing is stuck.
		t.state = Working
		return t.state
	}

	t.working = 0

	// A question on screen is the agent stopped, not the agent working: it
	// stays like that until someone answers. This isn't an ended turn.
	if question {
		t.silence = 0
		t.state = Approve
		return t.state
	}

	t.silence++
	if t.armed && t.silence >= readingsToEnd {
		t.armed = false
		t.state = Replied
	}
	return t.state
}

// Interact marks that someone asked the agent for something. From here on
// there's a turn to track.
func (t *Turn) Interact() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.interacted = true
}

// Seen marks that someone looked at the cell: whatever was waiting to be
// read has been read.
func (t *Turn) Seen() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == Replied {
		t.state = Working
	}
}

// State is the turn's state right now.
func (t *Turn) State() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// screenChanged compares the screen with the previous reading. It only keeps
// the hash because the screen is large and this runs twice a second for
// every cell.
func (t *Turn) screenChanged(screen string) bool {
	hash := sha256.Sum256([]byte(screen))
	if hash == t.screenHash {
		return false
	}
	t.screenHash = hash
	return true
}
