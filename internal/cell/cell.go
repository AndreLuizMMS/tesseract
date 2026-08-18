// Package cell holds the contract for a cell and the registry of the types
// that exist. A new type is a file in here plus one line in the registry —
// the screen, the keyboard and the engine are never touched.
package cell

import (
	"fmt"
	"sort"
	"sync"

	"github.com/andreluiz/tesseract/internal/engine/history"
)

// State is the cell's situation, as it appears in the marker.
type State string

const (
	Working  State = "working"  // live process producing
	Replied  State = "answered" // gave back the turn, has an answer waiting
	Approve  State = "approve"  // stuck on a question and won't move without an answer
	Crashed  State = "down"     // the process died on its own
	Stopped  State = "stopped"  // no process, cell preserved
	Orphaned State = "orphan"   // the project's directory vanished from disk
)

// Marker is the symbol for the state on the cell's bar.
func (e State) Marker() string {
	switch e {
	case Working:
		return "▸"
	case Replied:
		return "⬤"
	case Approve:
		return "⏵"
	case Crashed:
		return "✖"
	case Stopped:
		return "○"
	case Orphaned:
		return "⚠"
	}
	return "·"
}

// Config is everything a cell needs to be born.
type Config struct {
	ID        string           // stable identity, survives the process
	Directory string           // project root
	Name      string           // label chosen by the user
	Target    string           // md file, logs service
	Prompt    string           // initial work, agent cells only
	History   *history.History // where everything the cell produces is recorded
	Columns   int              // width of the area the screen reserved
	Lines     int              // height of the area the screen reserved
	Notify    func()           // called when the cell has something new to show

	// Conversation is the conversation identity to reattach to. Empty, the
	// agent starts a new one and reports which one through
	// OnConversationDiscovered. A cell with tabs has one per tab.
	Conversation             string
	Conversations            map[string]string
	OnConversationDiscovered func(tab, id string)

	// Tab is which tab is born active in a cell that has more than one.
	Tab string

	// Profiles is how each agent type comes up on this machine. The cell
	// looks up the profile of its own type; the one with tabs looks up each
	// tab's.
	Profiles map[string]Profile

	// OpenHistory serves cells with tabs: each tab gets its own file.
	OpenHistory func(suffix string) (*history.History, error)
}

// Profile is how one agent type comes up on this machine.
type Profile struct {
	Program       string
	Args          []string
	RenameCommand string
	Markers       Markers
}

// Keystroke is one key reaching the cell. It carries the key, not the raw
// bytes: the internal screen does the translating, since it knows the
// terminal modes in there.
type Keystroke struct {
	Code  rune   // the key; specials use the emulator's codes
	Text  string // what the key prints, when it prints anything
	Mod   int    // ctrl, alt, shift summed
	Paste string // text pasted all at once, not key by key
}

// Frame is what the screen draws: the cell's portrait right now.
type Frame struct {
	Lines   []string
	CursorX int
	CursorY int
	Scroll  int  // how many lines above live the reading is
	Live    bool // false when the user scrolled back
}

// WithTabs is a cell that holds more than one agent inside and switches
// between them.
type WithTabs interface {
	Tabs() []string
	ActiveTab() string
	SwitchTab(step int) error
}

// WithRefresh is a cell that has something to recheck when it comes back into
// view.
type WithRefresh interface {
	Refresh()
}

// WithHistory is a cell that keeps more than one history inside and knows
// which one search should look at.
type WithHistory interface {
	ActiveHistory() *history.History
}

// Cell is the contract. A type declares four things: how it's born, how it
// draws itself, what it does with a keystroke, and which states it has.
type Cell interface {
	Spawn(Config) error
	Draw() Frame
	Key(Keystroke) error
	State() State
	States() []State

	// Service: the screen reports size and scroll; the engine kills.
	Resize(columns, lines int) error
	Scroll(delta int, live bool)
	Kill() error
}

// Descriptor is the type's sheet: what the creation form needs to ask for a
// cell of that type to be born. Without this the form would need to know the
// types internally, and a new type would touch the screen.
type Descriptor struct {
	Type string
	// Order places the type in the form's queue. Lower goes first.
	Order int
	// TargetLabel is the name of the fourth field when the type needs a
	// target (md file, logs service). Empty when the type asks for nothing.
	TargetLabel string
	// TargetIsPath says the target is a path on disk and accepts tab
	// completion.
	TargetIsPath bool
	// AcceptsPrompt marks the types that come up already working.
	AcceptsPrompt bool
	// HasConversation marks the types that have a conversation with its own
	// name — the ones that respond to renaming from inside and adopt a name
	// from outside.
	HasConversation bool
}

type entry struct {
	descriptor Descriptor
	factory    func() Cell
}

var (
	mu       sync.RWMutex
	registry = map[string]entry{}
)

// Register declares a cell type. Called from the type's file init, and it's
// the only line a new type adds outside its own file.
func Register(descriptor Descriptor, factory func() Cell) {
	mu.Lock()
	defer mu.Unlock()
	if _, repeated := registry[descriptor.Type]; repeated {
		panic("cell type registered twice: " + descriptor.Type)
	}
	registry[descriptor.Type] = entry{descriptor: descriptor, factory: factory}
}

// New manufactures a cell of the requested type.
func New(kind string) (Cell, error) {
	mu.RLock()
	defer mu.RUnlock()
	found, exists := registry[kind]
	if !exists {
		return nil, fmt.Errorf("unknown cell type: %q", kind)
	}
	return found.factory(), nil
}

// Descriptors lists the sheets of the registered types, in the order the
// form should show them.
func Descriptors() []Descriptor {
	mu.RLock()
	defer mu.RUnlock()
	sheets := make([]Descriptor, 0, len(registry))
	for _, found := range registry {
		sheets = append(sheets, found.descriptor)
	}
	sort.Slice(sheets, func(i, j int) bool {
		if sheets[i].Order != sheets[j].Order {
			return sheets[i].Order < sheets[j].Order
		}
		return sheets[i].Type < sheets[j].Type
	})
	return sheets
}

// Describe returns the sheet for a type.
func Describe(kind string) (Descriptor, bool) {
	mu.RLock()
	defer mu.RUnlock()
	found, exists := registry[kind]
	return found.descriptor, exists
}

// Types lists the registered types, in the form's order.
func Types() []string {
	kinds := make([]string, 0)
	for _, sheet := range Descriptors() {
		kinds = append(kinds, sheet.Type)
	}
	return kinds
}
