package cell

import (
	"fmt"
	"sync"
	"time"

	"github.com/andreluiz/tesseract/internal/engine/history"
)

func init() {
	Register(Descriptor{
		Type:            "session",
		Order:           0,
		AcceptsPrompt:   true,
		HasConversation: true,
	}, func() Cell { return &Session{} })
}

// sessionTabs are the agents every session carries inside. Creating a
// session doesn't force a choice of which one gets used: the tab key
// switches between them any time.
var sessionTabs = []string{"claude", "cursor", "bash", "md"}

// Session is a cell with several tabs — one per agent. Only the tab in use
// has a process: the others are born when someone switches to them, so a
// session doesn't cost three programs at once.
type Session struct {
	mu     sync.Mutex
	config Config
	tabs   []string
	active int
	// open is the cell for each tab already opened, keyed by tab.
	open          map[string]Cell
	histories     map[string]*history.History
	columns       int
	lines         int
	conversations map[string]string
	closing       bool
	// asleep marks the tabs whose process was released for being silent out
	// of view. Nothing was lost: the conversation, the history and the size
	// are all still here, and switching back brings the agent up again where
	// it left off.
	asleep map[string]bool
	// sleepAfter is how much silence a tab out of view can take before it's
	// released. Zero keeps everyone awake.
	sleepAfter time.Duration
	stop       chan struct{}
}

// sleepCheckInterval is how often the session looks for a tab to release.
// A minute is fine: what's being decided here is measured in minutes.
const sleepCheckInterval = time.Minute

func (s *Session) Spawn(cfg Config) error {
	s.config = cfg
	s.tabs = append([]string{}, sessionTabs...)
	s.open = map[string]Cell{}
	s.histories = map[string]*history.History{}
	s.conversations = map[string]string{}
	for tab, conversation := range cfg.Conversations {
		s.conversations[tab] = conversation
	}
	s.columns, s.lines = max(cfg.Columns, 20), max(cfg.Lines, 5)
	s.asleep = map[string]bool{}
	s.sleepAfter = cfg.SleepAfter
	s.stop = make(chan struct{})

	s.active = 0
	for i, tab := range s.tabs {
		if tab == cfg.Tab {
			s.active = i
		}
	}
	if err := s.openTab(s.tabs[s.active], ""); err != nil {
		return err
	}
	if s.sleepAfter > 0 {
		go s.watchIdleTabs()
	}
	return nil
}

// watchIdleTabs releases the process of a tab that nobody is looking at and
// that has been silent long enough. An agent that opened and was never used
// costs the same as one that's working — a whole CLI plus everything it
// brings with it — and that's what this gets back.
func (s *Session) watchIdleTabs() {
	clock := time.NewTicker(sleepCheckInterval)
	defer clock.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-clock.C:
			s.sleepIdleTabs()
		}
	}
}

// sleepIdleTabs is one pass of the sweep. It only touches a tab that can be
// woken up where it left off — an agent with its own conversation. A shell
// is never released: what's running in there is the user's, not a turn, and
// silence in a shell doesn't mean idleness.
func (s *Session) sleepIdleTabs() {
	s.mu.Lock()
	if s.closing || len(s.tabs) == 0 {
		s.mu.Unlock()
		return
	}
	active := s.tabs[s.active]
	type candidate struct {
		tab  string
		cell Cell
	}
	var candidates []candidate
	for tab, item := range s.open {
		if tab == active || s.asleep[tab] {
			continue
		}
		candidates = append(candidates, candidate{tab: tab, cell: item})
	}
	after := s.sleepAfter
	s.mu.Unlock()

	for _, found := range candidates {
		if !sleepable(found.cell, after) {
			continue
		}
		s.sleepTab(found.tab, found.cell)
	}
}

// sleepable decides whether a tab can be released right now.
func sleepable(item Cell, after time.Duration) bool {
	// Only what knows how to come back: an agent reattaches to its own
	// conversation, a shell has nowhere to return to.
	if _, resumable := item.(WithConversation); !resumable {
		return false
	}
	idle, knows := item.(WithIdle)
	if !knows || idle.Idle() < after {
		return false
	}
	// A tab stuck on a question is holding a decision that only the user can
	// make. Releasing it would throw the question away.
	return item.State() != Approve
}

// sleepTab releases one tab's process, keeping everything that lets it come
// back: the conversation, the recorded history and the reserved size.
func (s *Session) sleepTab(tab string, item Cell) {
	_ = item.Kill()

	s.mu.Lock()
	delete(s.open, tab)
	record := s.histories[tab]
	delete(s.histories, tab)
	s.asleep[tab] = true
	s.mu.Unlock()

	if record != nil && record != s.config.History {
		_ = record.Close()
	}
	s.notify()
}

// AsleepTabs is which tabs are sleeping, so the screen can say so instead of
// letting a process vanish without a word.
func (s *Session) AsleepTabs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var sleeping []string
	for _, tab := range s.tabs {
		if s.asleep[tab] {
			sleeping = append(sleeping, tab)
		}
	}
	return sleeping
}

// openTab brings up a tab's agent, if it isn't up already, with prompt as
// the first thing it hears.
func (s *Session) openTab(tab, prompt string) error {
	s.mu.Lock()
	if _, alreadyOpen := s.open[tab]; alreadyOpen {
		s.mu.Unlock()
		return nil
	}
	config := s.config
	conversation := s.conversations[tab]
	columns, lines := s.columns, s.lines
	s.mu.Unlock()

	created, err := New(tab)
	if err != nil {
		return err
	}

	record := config.History
	if config.OpenHistory != nil {
		opened, err := config.OpenHistory(tab)
		if err != nil {
			return err
		}
		record = opened
	}

	config.Tab = tab
	config.Prompt = prompt
	config.Conversation = conversation
	config.History = record
	config.Columns, config.Lines = columns, lines
	// Only the tab in view wakes the mosaic. A background agent writes to
	// its own screen the whole time, and letting that through would rebuild
	// and push the whole grid dozens of times a second for something nobody
	// is looking at.
	config.Notify = func() { s.notifyFrom(tab) }
	config.OnConversationDiscovered = func(_ string, id string) {
		s.mu.Lock()
		s.conversations[tab] = id
		s.mu.Unlock()
		if s.config.OnConversationDiscovered != nil {
			s.config.OnConversationDiscovered(tab, id)
		}
	}

	if err := created.Spawn(config); err != nil {
		if record != nil && record != s.config.History {
			record.Close()
		}
		return err
	}

	s.mu.Lock()
	s.open[tab] = created
	delete(s.asleep, tab)
	if record != nil {
		s.histories[tab] = record
	}
	s.mu.Unlock()
	return nil
}

// RespawnTab takes one tab's agent down and brings it back up on the same
// conversation, with prompt as the first thing it hears.
func (s *Session) RespawnTab(tab, prompt string) error {
	s.mu.Lock()
	item := s.open[tab]
	s.mu.Unlock()
	if item != nil {
		s.sleepTab(tab, item)
	}
	return s.openTab(tab, prompt)
}

// Account is the account the tab's agent runs on, while it's up.
func (s *Session) Account(tab string) string {
	s.mu.Lock()
	item := s.open[tab]
	s.mu.Unlock()
	if withAccount, ok := item.(WithAccount); ok {
		return withAccount.Account(tab)
	}
	return ""
}

// current is the active tab's cell, or nil while it hasn't come up yet.
func (s *Session) current() Cell {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tabs) == 0 {
		return nil
	}
	return s.open[s.tabs[s.active]]
}

// Tabs lists the session's tabs, in the order the screen should show them.
func (s *Session) Tabs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.tabs...)
}

// ActiveTab is the tab showing right now.
func (s *Session) ActiveTab() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tabs) == 0 {
		return ""
	}
	return s.tabs[s.active]
}

// SwitchTab moves to the neighboring tab, bringing up its agent if it's the
// first time. Positive step goes right.
func (s *Session) SwitchTab(step int) error {
	s.mu.Lock()
	if len(s.tabs) == 0 {
		s.mu.Unlock()
		return fmt.Errorf("the session has no tabs")
	}
	if step == 0 {
		step = 1
	}
	s.active = (s.active + step%len(s.tabs) + len(s.tabs)) % len(s.tabs)
	tab := s.tabs[s.active]
	s.mu.Unlock()

	if err := s.openTab(tab, ""); err != nil {
		return err
	}
	// The tab that comes back into view rechecks whatever it needs — the
	// markdown one, for example, looks for a new file.
	if withRefresh, ok := s.current().(WithRefresh); ok {
		go withRefresh.Refresh()
	}
	s.notify()
	return nil
}

func (s *Session) notify() {
	if s.config.Notify != nil {
		s.config.Notify()
	}
}

// notifyFrom passes the news on only when it comes from the tab in view.
func (s *Session) notifyFrom(tab string) {
	if s.ActiveTab() != tab {
		return
	}
	s.notify()
}

func (s *Session) Draw() Frame {
	if current := s.current(); current != nil {
		return current.Draw()
	}
	return Frame{Live: true, CursorX: -1, CursorY: -1}
}

func (s *Session) Key(tap Keystroke) error {
	current := s.current()
	if current == nil {
		return fmt.Errorf("The tab is still coming up. Try again in a moment.")
	}
	return current.Key(tap)
}

func (s *Session) State() State {
	if current := s.current(); current != nil {
		return current.State()
	}
	return Stopped
}

func (s *Session) States() []State {
	return []State{Working, Idle, Replied, Approve, Interrupted, Background, Crashed, Stopped, Orphaned}
}

// Resize stores the size and passes it to every tab already opened:
// switching tabs can't show a terminal of the wrong size.
func (s *Session) Resize(columns, lines int) error {
	s.mu.Lock()
	if s.columns == columns && s.lines == lines {
		s.mu.Unlock()
		return nil
	}
	s.columns, s.lines = columns, lines
	opened := make([]Cell, 0, len(s.open))
	for _, item := range s.open {
		opened = append(opened, item)
	}
	s.mu.Unlock()

	for _, item := range opened {
		if err := item.Resize(columns, lines); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) Scroll(delta int, live bool) {
	if current := s.current(); current != nil {
		current.Scroll(delta, live)
	}
}

func (s *Session) Kill() error {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return nil
	}
	s.closing = true
	if s.stop != nil {
		close(s.stop)
	}
	opened := make([]Cell, 0, len(s.open))
	for _, item := range s.open {
		opened = append(opened, item)
	}
	records := make([]*history.History, 0, len(s.histories))
	for _, record := range s.histories {
		records = append(records, record)
	}
	s.mu.Unlock()

	for _, item := range opened {
		_ = item.Kill()
	}
	for _, record := range records {
		_ = record.Close()
	}
	return nil
}

// ActiveHistory is the file for the tab showing right now — it's the one
// search looks at.
func (s *Session) ActiveHistory() *history.History {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tabs) == 0 {
		return s.config.History
	}
	if record, ok := s.histories[s.tabs[s.active]]; ok {
		return record
	}
	return s.config.History
}

// Prompt sends work to the active tab, when it accepts it.
func (s *Session) Prompt(text string) error {
	current := s.current()
	if current == nil {
		return fmt.Errorf("The tab is still coming up. Try again in a moment.")
	}
	withPrompt, accepts := current.(WithPrompt)
	if !accepts {
		return fmt.Errorf("The %s tab takes no prompt. tab switches to an agent.", s.ActiveTab())
	}
	return withPrompt.Prompt(text)
}

// RequestAutoName tells the active tab's agent to choose its own name.
func (s *Session) RequestAutoName() error {
	withConversation, ok := s.current().(WithConversation)
	if !ok {
		return fmt.Errorf("The %s tab has no conversation to name. tab switches to an agent.", s.ActiveTab())
	}
	return withConversation.RequestAutoName()
}

// ConversationName is the name the active tab's agent gave the conversation.
func (s *Session) ConversationName() (string, error) {
	withConversation, ok := s.current().(WithConversation)
	if !ok {
		return "", fmt.Errorf("The %s tab has no conversation to name. tab switches to an agent.", s.ActiveTab())
	}
	return withConversation.ConversationName()
}

// Conversation is the active tab's conversation identity.
func (s *Session) Conversation() string {
	if withConversation, ok := s.current().(WithConversation); ok {
		return withConversation.Conversation()
	}
	return ""
}

// Conversations is what the engine keeps to reattach each tab after a crash.
func (s *Session) Conversations() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make(map[string]string, len(s.conversations))
	for tab, conversation := range s.conversations {
		copied[tab] = conversation
	}
	return copied
}
