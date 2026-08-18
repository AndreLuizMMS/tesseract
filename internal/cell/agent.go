package cell

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/vt"

	"github.com/andreluiz/tesseract/internal/engine/history"
)

// observationInterval is how often the cell looks at its own screen to
// decide what turn state it's in.
const observationInterval = 500 * time.Millisecond

// enterDelay gives the agent time to digest the pasted text before receiving
// the enter that tells the turn to begin.
const enterDelay = 120 * time.Millisecond

// WithPrompt is a cell that accepts work without the user entering it.
type WithPrompt interface {
	Prompt(text string) error
}

// WithConversation is a cell that has a conversation with its own name: you
// can push the name into it and pull the name it chose.
type WithConversation interface {
	// RequestAutoName tells the agent to choose and write the conversation's
	// own name, without the user typing anything.
	RequestAutoName() error
	// ConversationName is the name the agent itself gave the conversation.
	ConversationName() (string, error)
	// Conversation is the conversation's identity, to reattach after a crash.
	Conversation() string
}

// Agent is the base for claude and cursor: an interactive process with turn
// state, a named conversation, and prompting without entering.
type Agent struct {
	Process
	turn             *Turn
	renameCommand    string
	conversation     string
	bornAt           time.Time
	stop             chan struct{}
	readName         func(directory, conversation string) (string, error)
	findConversation func(directory string, since time.Time) string
}

// spawn brings the agent up and starts following its turn.
func (a *Agent) spawn(cfg Config, profile Profile, program string, args []string, markers Markers) error {
	// The user's config wins: it exists for the day the agent changes the
	// text it writes on its own screen.
	if len(profile.Markers.Working) > 0 {
		markers.Working = profile.Markers.Working
	}
	if len(profile.Markers.Question) > 0 {
		markers.Question = profile.Markers.Question
	}
	a.turn = NewTurn(markers)
	a.conversation = cfg.Conversation
	a.bornAt = time.Now()
	a.stop = make(chan struct{})
	if err := a.start(cfg, program, args, terminalEnvironment()); err != nil {
		return err
	}
	go a.watchTurn()
	return nil
}

// watchTurn reads the screen at regular intervals and updates the state.
// This is where the false-alarm heuristic runs.
func (a *Agent) watchTurn() {
	clock := time.NewTicker(observationInterval)
	defer clock.Stop()
	previous := a.turn.State()
	for {
		select {
		case <-a.stop:
			return
		case <-clock.C:
			if a.Process.State() != Working {
				return // the process died; it's in charge of the state now
			}
			state := a.turn.Observe(a.screenAsText())
			if state != previous {
				previous = state
				a.notify()
			}
			if a.conversation == "" && a.findConversation != nil {
				if found := a.findConversation(a.config.Directory, a.bornAt); found != "" {
					a.conversation = found
					if a.config.OnConversationDiscovered != nil {
						a.config.OnConversationDiscovered(a.config.Tab, found)
					}
				}
			}
		}
	}
}

// screenAsText is the cell's screen without color codes, which is what the
// agent's markers are compared against.
func (a *Agent) screenAsText() string {
	lines := a.Draw().Lines
	cleaned := make([]string, len(lines))
	for i, line := range lines {
		cleaned[i] = history.StripCodes(line)
	}
	return strings.Join(cleaned, "\n")
}

// State is the process's state when it died, and the turn's state while it's
// alive.
func (a *Agent) State() State {
	if base := a.Process.State(); base != Working {
		return base
	}
	return a.turn.State()
}

func (a *Agent) States() []State {
	return []State{Working, Replied, Approve, Crashed, Stopped, Orphaned}
}

// Key marks the cell as read: whoever entered it already saw what there was
// to see.
func (a *Agent) Key(tap Keystroke) error {
	a.turn.Interact()
	a.turn.Seen()
	return a.Process.Key(tap)
}

// Prompt sends work to the agent without the user entering the cell.
func (a *Agent) Prompt(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("the prompt is empty")
	}
	a.turn.Interact()
	if err := a.Process.Key(Keystroke{Paste: text}); err != nil {
		return err
	}
	time.Sleep(enterDelay)
	return a.Process.Key(Keystroke{Code: vt.KeyEnter})
}

// RequestAutoName sends the plain rename command, without a name — the agent
// itself chooses and writes the conversation's title.
func (a *Agent) RequestAutoName() error {
	if a.renameCommand == "" {
		return fmt.Errorf("this agent doesn't know how to rename its own conversation")
	}
	return a.Prompt(a.renameCommand)
}

// ConversationName is the name the agent itself gave the conversation. A
// nameless conversation reports an error instead of renaming the cell to
// empty.
func (a *Agent) ConversationName() (string, error) {
	if a.readName == nil {
		return "", fmt.Errorf("this agent doesn't keep a conversation name")
	}
	name, err := a.readName(a.config.Directory, a.conversation)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("the conversation doesn't have a name yet")
	}
	return name, nil
}

// Conversation is the conversation's identity, which the engine keeps to
// reattach later.
func (a *Agent) Conversation() string { return a.conversation }

// Kill ends the watch along with the process.
func (a *Agent) Kill() error {
	select {
	case <-a.stop:
	default:
		close(a.stop)
	}
	return a.Process.Kill()
}

// newConversationID generates the identifier the agent gets so it can be
// reattached after a crash.
func newConversationID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return ""
	}
	// UUID version 4 format, which is what the agents expect.
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	text := hex.EncodeToString(raw)
	return text[0:8] + "-" + text[8:12] + "-" + text[12:16] + "-" + text[16:20] + "-" + text[20:]
}
