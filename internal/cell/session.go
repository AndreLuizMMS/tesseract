package cell

import (
	"fmt"
	"sync"

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
}

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

	s.active = 0
	for i, tab := range s.tabs {
		if tab == cfg.Tab {
			s.active = i
		}
	}
	return s.openTab(s.tabs[s.active])
}

// openTab brings up a tab's agent, if it isn't up already.
func (s *Session) openTab(tab string) error {
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
	config.Conversation = conversation
	config.History = record
	config.Columns, config.Lines = columns, lines
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
	if record != nil {
		s.histories[tab] = record
	}
	s.mu.Unlock()
	return nil
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

	if err := s.openTab(tab); err != nil {
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

func (s *Session) Draw() Frame {
	if current := s.current(); current != nil {
		return current.Draw()
	}
	return Frame{Live: true, CursorX: -1, CursorY: -1}
}

func (s *Session) Key(tap Keystroke) error {
	current := s.current()
	if current == nil {
		return fmt.Errorf("the tab hasn't come up yet")
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
	return []State{Working, Replied, Approve, Crashed, Stopped, Orphaned}
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
		return fmt.Errorf("the tab hasn't come up yet")
	}
	withPrompt, accepts := current.(WithPrompt)
	if !accepts {
		return fmt.Errorf("the %s tab doesn't take a prompt", s.ActiveTab())
	}
	return withPrompt.Prompt(text)
}

// RequestAutoName tells the active tab's agent to choose its own name.
func (s *Session) RequestAutoName() error {
	withConversation, ok := s.current().(WithConversation)
	if !ok {
		return fmt.Errorf("the %s tab doesn't have a named conversation", s.ActiveTab())
	}
	return withConversation.RequestAutoName()
}

// ConversationName is the name the active tab's agent gave the conversation.
func (s *Session) ConversationName() (string, error) {
	withConversation, ok := s.current().(WithConversation)
	if !ok {
		return "", fmt.Errorf("the %s tab doesn't have a named conversation", s.ActiveTab())
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
