package cell

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
	turn          *Turn
	renameCommand string
	conversation  string
	// account is the config folder of the Claude account the agent runs on;
	// empty for an agent that has no accounts to choose from.
	account string
	bornAt  time.Time
	stop    chan struct{}
	// away says the screen shows something other than the conversation (see
	// Markers.Away). It's read off the screen on every watch.
	away             atomic.Bool
	readName         func(directory, conversation string) (string, error)
	findConversation func(directory string, since time.Time) string

	// linkMu guards what the mod changes from outside the watch: the
	// conversation's identity and the way to reach the mod itself. links are
	// the mod connections attached right now, oldest first; acks are the
	// deliveries still waiting for the mod to say it took them.
	linkMu     sync.Mutex
	links      []*func(Delivery) error
	acks       map[string]chan bool
	deliveries int64
}

// spawn brings the agent up and starts following its turn.
func (a *Agent) spawn(cfg Config, profile Profile, program string, args []string, markers Markers, environment ...string) error {
	// The user's config wins: it exists for the day the agent changes the
	// text it writes on its own screen.
	// It replaces what matches anywhere on the screen; the line markers
	// stay, since they're Tesseract's own skin and change with it.
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
	if err := a.start(cfg, program, args, append(append(terminalEnvironment(), linkEnvironment(cfg)...), environment...)); err != nil {
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
			screen := a.screenAsText()
			state := a.turn.Observe(screen)
			wasAway := a.away.Swap(anyLineStarts(screen, a.turn.markers.Away))
			if state != previous || wasAway != a.away.Load() {
				previous = state
				a.notify()
			}
			if a.Conversation() == "" && a.findConversation != nil {
				if found := a.findConversation(a.config.Directory, a.bornAt); found != "" {
					_ = a.Rebind("", found)
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
	// Whatever the turn last said was about a conversation that isn't on
	// this screen any more.
	if a.away.Load() {
		return Background
	}
	return a.turn.State()
}

func (a *Agent) States() []State {
	return []State{Working, Idle, Replied, Approve, Interrupted, Background, Crashed, Stopped, Orphaned}
}

// Key marks the cell as read: whoever entered it already saw what there was
// to see.
func (a *Agent) Key(tap Keystroke) error {
	a.turn.Interact()
	a.turn.Seen()
	if answers(tap) {
		a.turn.Answer()
	}
	return a.Process.Key(tap)
}

// Prompt sends work to the agent without the user entering the cell.
func (a *Agent) Prompt(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("Empty prompt. p opens the box again.")
	}
	a.turn.Interact()
	// With a mod inside the session the text goes in as the user's own
	// words, queued behind whatever is running. Pasting is what's left for
	// an agent with no mod, or one whose mod didn't answer: it types into
	// whatever has the keyboard, and the enter has to wait for the agent to
	// digest the paste.
	// The list of agents is on screen: the conversation's own session would
	// take the prompt and run it where nobody is looking.
	if a.away.Load() {
		return errNotOnScreen
	}
	if taken, err := a.viaLink(text); taken || err != nil {
		return err
	}
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
		return fmt.Errorf("This agent can't rename its own conversation. Name it from inside: ↵ enters the cell.")
	}
	return a.Prompt(a.renameCommand)
}

// ConversationName is the name the agent itself gave the conversation. A
// nameless conversation reports an error instead of renaming the cell to
// empty.
func (a *Agent) ConversationName() (string, error) {
	if a.readName == nil {
		return "", fmt.Errorf("This agent keeps no conversation name. Name it from inside: ↵ enters the cell.")
	}
	name, err := a.readName(a.config.Directory, a.Conversation())
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("The conversation has no name yet. ctrl+r asks the agent for one.")
	}
	return name, nil
}

// Account is the config folder of the account the agent runs on. The tab is
// the session's to use: a single agent is its own only tab.
func (a *Agent) Account(string) string { return a.account }

// Conversation is the conversation's identity, which the engine keeps to
// reattach later.
func (a *Agent) Conversation() string {
	a.linkMu.Lock()
	defer a.linkMu.Unlock()
	return a.conversation
}

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
