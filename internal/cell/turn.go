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
//
// Working and Question match anywhere on the screen. WorkingLines and
// QuestionLines only match at the start of a line: they're for an interface
// that writes the same word in more than one place, where only the line that
// opens with it means the thing.
type Markers struct {
	Working       []string
	Question      []string
	WorkingLines  []string
	QuestionLines []string
	// Away is what a line of the agent's screen opens with while the
	// conversation isn't on it. No mod can say this one: the session it would
	// speak for is exactly what left.
	Away []string
}

// anyLineStarts says whether any line of the screen opens with one of the
// texts, indentation aside.
func anyLineStarts(screen string, texts []string) bool {
	if len(texts) == 0 {
		return false
	}
	for line := range strings.SplitSeq(screen, "\n") {
		line = strings.TrimLeft(line, " ")
		for _, text := range texts {
			if text != "" && strings.HasPrefix(line, text) {
				return true
			}
		}
	}
	return false
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
	// linked marks that a mod inside the agent's own session is attached, and
	// spoken that it has said something since. From its first word on the
	// screen isn't read at all: what the agent says about itself beats any
	// guess from outside. Until then the screen is all there is — a mod that
	// attaches while a question read off the screen is still up knows nothing
	// about it, and would leave it there until the next turn.
	linked bool
	spoken bool
	// signaledAt is the mod's clock on the last state it reported, so a
	// report that arrives late doesn't undo a newer one.
	signaledAt int64
}

// NewTurn prepares tracking for an agent with its markers.
func NewTurn(markers Markers) *Turn {
	return &Turn{markers: markers, state: Working}
}

// Observe takes the cell's screen and returns what state it's in.
func (t *Turn) Observe(screen string) State {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.linked && t.spoken {
		return t.state
	}

	changed := t.screenChanged(screen)
	question := anyPresent(screen, t.markers.Question) || anyLineStarts(screen, t.markers.QuestionLines)

	working := changed
	if len(t.markers.Working) > 0 || len(t.markers.WorkingLines) > 0 {
		working = anyPresent(screen, t.markers.Working) || anyLineStarts(screen, t.markers.WorkingLines)
	}
	// A line that asks beats a line that works: with a call waiting on the
	// user, the calls queued behind it are still drawn as work in progress,
	// and the turn is stopped all the same.
	if anyLineStarts(screen, t.markers.QuestionLines) {
		working = false
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

	// The question left the screen, so somebody answered it. Approve is a
	// question on screen and nothing else: without this, an agent that never
	// shows work afterwards — or shows it in words this cell doesn't know —
	// would be stuck asking for an approval already given.
	if t.state == Approve {
		t.state = Working
	}

	t.silence++
	if t.armed && t.silence >= readingsToEnd {
		t.armed = false
		t.state = Replied
	}
	return t.state
}

// Link says whether a mod is attached to speak for the turn. Either way the
// screen reading starts clean: turned off, the counters from before the mod
// took over describe a screen that's long gone; turned on, the screen keeps
// being read until the mod's first word.
func (t *Turn) Link(on bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.linked, t.spoken = on, false
	t.working, t.silence, t.armed = 0, 0, false
}

// Signal is the agent itself saying what state its turn is in. It only
// counts while a mod is linked, and returns whether it was taken: with the
// link down the screen reading is in charge, and a report older than the
// last one taken is a straggler.
func (t *Turn) Signal(state State, at int64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.linked || at < t.signaledAt {
		return false
	}
	t.signaledAt = at
	// Idle is the agent saying nothing is going on. An answer nobody read
	// yet is still waiting whatever the agent does at its prompt: only
	// someone looking at the cell takes that marker down.
	if state != Idle || t.state != Replied {
		t.state = state
	}
	t.spoken = true
	// A turn the agent reports is a turn someone asked for, even when the
	// request came from a place the cell never saw.
	t.interacted = true
	return true
}

// Answer marks that the user answered the question the agent was stuck on.
// The mod only speaks again when the approved tool finishes, which can take
// minutes — without this the cell would keep asking for an approval that
// was already given.
func (t *Turn) Answer() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.linked && t.state == Approve {
		t.state = Working
	}
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
		// A mod knows the agent is back at its prompt. The screen reading has
		// no word for that, and keeps the one it always used.
		t.state = Working
		if t.linked && t.spoken {
			t.state = Idle
		}
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
