package cell

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/charmbracelet/x/vt"
)

// WithLink is a cell whose agent can speak for itself: a mod running inside
// its session reports the turn's state and the conversation's identity, and
// takes prompts on the way back. The tab says which agent of a cell with
// tabs is talking; a cell without tabs ignores it.
type WithLink interface {
	// Attach hands the cell the way to reach its mod. From here until the
	// returned function is called, the mod's word about the turn beats the
	// screen reading.
	Attach(tab string, deliver func(Delivery) error) (detach func(), err error)
	// Delivered is the mod answering a delivery: it took it and is
	// submitting it, or its session had nowhere to put it.
	Delivered(tab, id string, taken bool) error
	// Signal is the mod reporting the turn's state, with its own clock.
	Signal(tab string, state State, at int64) error
	// Rebind is the mod reporting that the conversation changed under the
	// cell — a /clear, a /resume, a /branch.
	Rebind(tab, conversation string) error
}

// Delivery is one piece of work on its way to the mod. The mod answers with
// the ID when it takes it, and drops it once Expires (the clock in
// milliseconds) has passed: by then the cell has typed it in instead.
type Delivery struct {
	ID      string
	Text    string
	Expires int64
}

// ackWait is how long the mod has to say it took a delivery. The answer is
// one short process talking to a local socket; past this the mod is gone,
// hung, or not the session on screen.
var ackWait = 2 * time.Second

// selfPath is the running binary, read once at startup: after an update the
// file is swapped by rename, and asking again would return a path that no
// longer exists.
var selfPath, _ = os.Executable()

// linkEnvironment is what the mod needs to find its way back to this cell:
// which cell and tab it's in, and which binary reports to the engine.
func linkEnvironment(cfg Config) []string {
	return []string{
		"TESSERACT_CELL_ID=" + cfg.ID,
		"TESSERACT_TAB=" + cfg.Tab,
		"TESSERACT_BIN=" + selfPath,
	}
}

// Attach makes this connection the cell's way to its mod. Several can be
// attached at once — a mod that reloads attaches again before the old
// connection notices it's gone — and the rule is that the newest one owns the
// cell: deliveries go to it alone, and when it drops the one before it takes
// over. The mod's word stops beating the screen only when none is left.
func (a *Agent) Attach(_ string, deliver func(Delivery) error) (func(), error) {
	// The pointer is the identity: detaching removes this connection and no
	// other.
	mine := &deliver
	a.linkMu.Lock()
	a.links = append(a.links, mine)
	a.linkMu.Unlock()
	a.turn.Link(true)
	return func() {
		a.linkMu.Lock()
		a.links = slices.DeleteFunc(a.links, func(link *func(Delivery) error) bool { return link == mine })
		last := len(a.links) == 0
		a.linkMu.Unlock()
		if last {
			a.turn.Link(false)
			a.notify()
		}
	}, nil
}

func (a *Agent) Delivered(_, id string, taken bool) error {
	a.linkMu.Lock()
	answer := a.acks[id]
	delete(a.acks, id)
	a.linkMu.Unlock()
	if answer != nil {
		answer <- taken // buffered: the one answer never waits for a reader
	}
	return nil
}

func (a *Agent) Signal(_ string, state State, at int64) error {
	if a.turn.Signal(state, at) {
		a.notify()
	}
	return nil
}

func (a *Agent) Rebind(_, conversation string) error {
	if conversation == "" {
		return fmt.Errorf("the conversation has no identity")
	}
	a.linkMu.Lock()
	changed := a.conversation != conversation
	a.conversation = conversation
	a.linkMu.Unlock()
	if changed && a.config.OnConversationDiscovered != nil {
		a.config.OnConversationDiscovered(a.config.Tab, conversation)
	}
	return nil
}

// errNotOnScreen is a prompt with nowhere to go: the conversation was sent to
// the background, and the cell shows Claude Code's list of agents instead.
// Typing the text there would start something else. The cell reads it off
// the screen, and a mod whose session can't take work says the same.
var errNotOnScreen = fmt.Errorf("The conversation isn't on screen in this cell. ↵ enters it; esc there brings the conversation back.")

// viaLink hands the text to the mod that owns the cell and waits for its
// answer. Taken means the mod is submitting it. Not taken with no error means
// the text still has to go in through the terminal: no mod, a connection
// that fails, or one that never answers — a write that reached the socket
// says nothing about who is reading it.
func (a *Agent) viaLink(text string) (taken bool, err error) {
	a.linkMu.Lock()
	if len(a.links) == 0 {
		a.linkMu.Unlock()
		return false, nil
	}
	owner := a.links[len(a.links)-1]
	a.deliveries++
	id := strconv.FormatInt(a.deliveries, 10)
	answer := make(chan bool, 1)
	if a.acks == nil {
		a.acks = map[string]chan bool{}
	}
	a.acks[id] = answer
	a.linkMu.Unlock()
	defer func() {
		a.linkMu.Lock()
		delete(a.acks, id)
		a.linkMu.Unlock()
	}()

	deadline := time.Now().Add(ackWait)
	if (*owner)(Delivery{ID: id, Text: text, Expires: deadline.UnixMilli()}) != nil {
		return false, nil
	}
	select {
	case taken := <-answer:
		if !taken {
			return false, errNotOnScreen
		}
		return true, nil
	case <-time.After(time.Until(deadline)):
		return false, nil
	}
}

// answers says whether the key is one that settles a question the agent
// put on screen: enter and esc in any dialog, a digit in the ones that list
// numbered options.
func answers(tap Keystroke) bool {
	if tap.Code == vt.KeyEnter || tap.Code == vt.KeyEscape {
		return true
	}
	return len(tap.Text) == 1 && tap.Text[0] >= '1' && tap.Text[0] <= '9'
}

func (s *Session) linked(tab string) (WithLink, error) {
	s.mu.Lock()
	item := s.open[tab]
	s.mu.Unlock()
	withLink, ok := item.(WithLink)
	if !ok {
		return nil, fmt.Errorf("the %s tab has no agent to link", tab)
	}
	return withLink, nil
}

func (s *Session) Attach(tab string, deliver func(Delivery) error) (func(), error) {
	withLink, err := s.linked(tab)
	if err != nil {
		return nil, err
	}
	return withLink.Attach(tab, deliver)
}

func (s *Session) Delivered(tab, id string, taken bool) error {
	withLink, err := s.linked(tab)
	if err != nil {
		return err
	}
	return withLink.Delivered(tab, id, taken)
}

func (s *Session) Signal(tab string, state State, at int64) error {
	withLink, err := s.linked(tab)
	if err != nil {
		return err
	}
	return withLink.Signal(tab, state, at)
}

func (s *Session) Rebind(tab, conversation string) error {
	withLink, err := s.linked(tab)
	if err != nil {
		return err
	}
	return withLink.Rebind(tab, conversation)
}
