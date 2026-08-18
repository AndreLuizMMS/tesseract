// Package engine owns everything: the processes, each cell's internal
// screen, the history and the desired state. The screen is a client — it
// draws what the engine tells it to and hands back keystrokes. No rule lives
// in the screen.
package engine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/docker"
	"github.com/andreluiz/tesseract/internal/engine/history"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// Engine holds the live grid.
type Engine struct {
	mu       sync.Mutex
	stateDir string
	config   Config
	projects []*project
	screen   string
	notice   string
	version  atomic.Uint64
	stop     chan struct{}
	once     sync.Once
}

type project struct {
	id    string
	path  string
	color int
	// compose is the file detected at the root; empty when the project has none.
	compose string
	// docker is the stack summary for the strip, refreshed from time to time.
	docker string
	cells  []*liveCell
}

type liveCell struct {
	id            string
	kind          string
	name          string
	target        string
	tab           string
	conversations map[string]string
	proc          cell.Cell
	record        *history.History
	// lastState is what the watcher saw last time, so it only notifies the
	// change and not the idle state.
	lastState cell.State
	// pendingRename marks that an automatic /rename was sent and the watcher
	// needs to adopt the name as soon as the turn ends.
	pendingRename bool
}

// New prepares an engine over a state directory.
func New(stateDir string, config Config) *Engine {
	if config.HistoryLimit <= 0 {
		config.HistoryLimit = history.DefaultCap
	}
	if config.Agents == nil {
		config.Agents = map[string]AgentProfile{}
	}
	return &Engine{
		stateDir: stateDir,
		config:   config,
		screen:   "grid",
		stop:     make(chan struct{}),
	}
}

// Start loads the desired state and reconstructs the grid. Reattaching never
// triggers work: an agent cell wakes up idle.
func (e *Engine) Start() error {
	state, loadErr := Load(StatePath(e.stateDir))

	e.mu.Lock()
	defer e.mu.Unlock()
	if loadErr != nil {
		e.notice = loadErr.Error()
	}
	if state.Screen != "" {
		e.screen = state.Screen
	}
	for _, savedProject := range state.Projects {
		p := &project{id: savedProject.ID, path: savedProject.Path, color: savedProject.Color, compose: docker.Detect(savedProject.Path)}
		if p.id == "" {
			p.id = newIdentity()
		}
		for _, savedCell := range savedProject.Cells {
			c := &liveCell{
				id:            savedCell.ID,
				kind:          savedCell.Kind,
				name:          savedCell.Name,
				target:        savedCell.Target,
				tab:           savedCell.Tab,
				conversations: map[string]string{},
			}
			for tab, conversation := range savedCell.Conversations {
				c.conversations[tab] = conversation
			}
			// State written before tabs kept only one conversation.
			if savedCell.Conversation != "" && len(c.conversations) == 0 {
				c.conversations[savedCell.Kind] = savedCell.Conversation
			}
			if c.id == "" {
				c.id = newIdentity()
			}
			if err := e.wake(p, c); err != nil {
				e.notice = err.Error()
			}
			p.cells = append(p.cells, c)
		}
		e.projects = append(e.projects, p)
	}
	e.markDirty()
	return loadErr
}

// wake starts a reconstructed cell's process. Called with the lock held.
func (e *Engine) wake(p *project, c *liveCell) error {
	proc, err := cell.New(c.kind)
	if err != nil {
		return err
	}
	record, err := history.Open(HistoryPath(e.stateDir, c.id), e.config.HistoryLimit)
	if err != nil {
		return err
	}
	c.record = record
	c.proc = proc
	c.lastState = cell.Working

	cellID := c.id
	conversations := map[string]string{}
	for tab, conversation := range c.conversations {
		conversations[tab] = conversation
	}
	return proc.Spawn(cell.Config{
		ID:        c.id,
		Directory: p.path,
		Name:      c.name,
		Target:    c.target,
		History:   record,
		Columns:   80,
		Lines:     24,
		Notify:    e.markDirty,

		Tab:           c.tab,
		Conversation:  conversations[c.kind],
		Conversations: conversations,
		// Off the lock: whoever calls this may be the cell's own birth, which
		// is already holding the engine's lock.
		OnConversationDiscovered: func(tab, id string) { go e.saveConversation(cellID, tab, id) },

		Profiles: e.profiles(),
		OpenHistory: func(suffix string) (*history.History, error) {
			return history.Open(HistoryPath(e.stateDir, cellID+"-"+suffix), e.config.HistoryLimit)
		},
	})
}

// profiles translates the user's config into what cells understand.
func (e *Engine) profiles() map[string]cell.Profile {
	profiles := map[string]cell.Profile{}
	for kind, profile := range e.config.Agents {
		profiles[kind] = cell.Profile{
			Program:       profile.Program,
			Args:          profile.Args,
			RenameCommand: profile.RenameCommand,
			Markers: cell.Markers{
				Working:  profile.WorkMarkers,
				Question: profile.QuestionMarkers,
			},
		}
	}
	return profiles
}

// saveConversation records the identity of the conversation the agent
// opened, so it can be reattached after a crash. On a cell with tabs, each
// tab has its own.
func (e *Engine) saveConversation(cellID, tab, conversation string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, p := range e.projects {
		for _, c := range p.cells {
			if c.id != cellID {
				continue
			}
			if tab == "" {
				tab = c.kind
			}
			if c.conversations == nil {
				c.conversations = map[string]string{}
			}
			if c.conversations[tab] == conversation {
				return
			}
			c.conversations[tab] = conversation
			e.save()
			return
		}
	}
}

// Create spawns a cell, creating the project along with it when its path
// isn't on screen yet.
func (e *Engine) Create(request protocol.Create) (string, error) {
	path, err := validatePath(request.Path)
	if err != nil {
		return "", err
	}
	request.Type = requestKind(request)

	e.mu.Lock()
	defer e.mu.Unlock()

	p := e.findProjectByPath(path)
	newProject := p == nil
	if newProject {
		p = &project{id: newIdentity(), path: path, color: e.nextColor(), compose: docker.Detect(path)}
	}

	c := &liveCell{
		id:            newIdentity(),
		kind:          request.Type,
		name:          strings.TrimSpace(request.Name),
		target:        request.Target,
		conversations: map[string]string{},
	}
	if c.name == "" {
		c.name = autoName(p, request.Type)
	}

	if err := e.wake(p, c); err != nil {
		if c.record != nil {
			c.record.Close()
		}
		return "", err
	}

	p.cells = append(p.cells, c)
	if newProject {
		e.projects = append(e.projects, p)
	}
	e.save()
	e.markDirty()
	return c.id, nil
}

// requestKind decides what gets born when the request doesn't say the kind: a
// markdown file becomes a read cell, and everything else becomes a session —
// which already brings agents along, without forcing anyone to choose at
// creation time.
func requestKind(request protocol.Create) string {
	if request.Type != "" {
		return request.Type
	}
	target := strings.ToLower(strings.TrimSpace(request.Target))
	if strings.HasSuffix(target, ".md") || strings.HasSuffix(target, ".markdown") {
		return "md"
	}
	return "session"
}

// Kill removes the cell. If it's the project's last one, the project leaves
// the screen — and disk stays exactly as it was.
func (e *Engine) Kill(cellID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for iProject, p := range e.projects {
		for iCell, c := range p.cells {
			if c.id != cellID {
				continue
			}
			if c.proc != nil {
				_ = c.proc.Kill()
			}
			if c.record != nil {
				_ = c.record.Close()
			}
			_ = os.Remove(HistoryPath(e.stateDir, c.id))
			p.cells = append(p.cells[:iCell], p.cells[iCell+1:]...)
			if len(p.cells) == 0 {
				e.projects = append(e.projects[:iProject], e.projects[iProject+1:]...)
			}
			e.save()
			e.markDirty()
			return nil
		}
	}
	return fmt.Errorf("cell %s does not exist", cellID)
}

// IsLastCell reports whether killing this cell takes the project off screen,
// so the confirmation can warn about it.
func (e *Engine) IsLastCell(cellID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, p := range e.projects {
		for _, c := range p.cells {
			if c.id == cellID {
				return len(p.cells) == 1
			}
		}
	}
	return false
}

// Key delivers the keystroke to the focused cell. This is TYPE mode.
func (e *Engine) Key(request protocol.Key) error {
	cellID := request.Cell
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", cellID)
	}
	tap := cell.Keystroke{
		Code:  request.Code,
		Text:  request.Text,
		Mod:   request.Mod,
		Paste: request.Paste,
	}
	if err := c.proc.Key(tap); err != nil {
		return err
	}
	e.markDirty()
	return nil
}

// Resize fits the cell's area to what the screen reserved for it.
func (e *Engine) Resize(cellID string, columns, rows int) error {
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", cellID)
	}
	return c.proc.Resize(columns, rows)
}

// Scroll moves the cell's history reading position.
func (e *Engine) Scroll(cellID string, delta int, live bool) error {
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", cellID)
	}
	c.proc.Scroll(delta, live)
	e.markDirty()
	return nil
}

// Search looks for a term in the cell's history.
func (e *Engine) Search(cellID, term string) ([]history.Match, error) {
	c := e.findCell(cellID)
	if c == nil {
		return nil, fmt.Errorf("cell %s does not exist", cellID)
	}
	return e.historyOf(c).Search(term)
}

// historyOf is the history that search and scroll should look at: on a cell
// with tabs, the one belonging to the tab currently showing.
func (e *Engine) historyOf(c *liveCell) *history.History {
	if withHistory, ok := c.proc.(cell.WithHistory); ok {
		if record := withHistory.ActiveHistory(); record != nil {
			return record
		}
	}
	return c.record
}

// Rename changes the cell's label. Name is a label: it doesn't touch the
// process, the directory, or the history.
func (e *Engine) Rename(cellID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("the name cannot be empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, p := range e.projects {
		for _, c := range p.cells {
			if c.id == cellID {
				c.name = name
				e.save()
				e.markDirty()
				return nil
			}
		}
	}
	return fmt.Errorf("cell %s does not exist", cellID)
}

// Snapshot is the whole state, ready to draw.
func (e *Engine) Snapshot() protocol.State {
	e.mu.Lock()
	defer e.mu.Unlock()

	snapshot := protocol.State{Notice: e.notice, Screen: e.screen, Types: typeCards(), Quota: ReadQuota()}
	for _, p := range e.projects {
		drawing := protocol.Project{
			ID:         p.id,
			Path:       p.path,
			Name:       filepath.Base(p.path),
			Color:      p.color,
			HasCompose: p.compose != "",
			Docker:     p.docker,
		}
		for _, c := range p.cells {
			cellSnapshot := protocol.Cell{
				ID:    c.id,
				Type:  c.kind,
				Name:  c.name,
				State: string(cell.Stopped),
				Live:  true,
			}
			if withTabs, ok := c.proc.(cell.WithTabs); ok {
				cellSnapshot.Tabs = withTabs.Tabs()
				cellSnapshot.Tab = withTabs.ActiveTab()
			}
			if c.proc != nil {
				frame := c.proc.Draw()
				cellSnapshot.State = string(c.proc.State())
				cellSnapshot.Lines = frame.Lines
				cellSnapshot.CursorX = frame.CursorX
				cellSnapshot.CursorY = frame.CursorY
				cellSnapshot.Scroll = frame.Scroll
				cellSnapshot.Live = frame.Live
			}
			drawing.Cells = append(drawing.Cells, cellSnapshot)
		}
		snapshot.Projects = append(snapshot.Projects, drawing)
	}
	return snapshot
}

// nextColor picks the new project's column color: the first one nobody is
// using, so that two neighboring projects never look the same.
func (e *Engine) nextColor() int {
	used := map[int]bool{}
	for _, p := range e.projects {
		used[p.color] = true
	}
	for color := 0; ; color++ {
		if !used[color] {
			return color
		}
	}
}

// typeCards gives the screen what it needs to build the form without knowing
// any type internally. The type list is fixed from startup, and goes into
// every snapshot: building it once is enough.
var typeCards = sync.OnceValue(func() []protocol.CellType {
	var cards []protocol.CellType
	for _, descriptor := range cell.Descriptors() {
		cards = append(cards, protocol.CellType{
			Type:          descriptor.Type,
			TargetLabel:   descriptor.TargetLabel,
			CompletesPath: descriptor.TargetIsPath,
			AcceptsPrompt: descriptor.AcceptsPrompt,
			Chat:          descriptor.HasConversation,
		})
	}
	return cards
})

// Summary is what `tess status` shows.
func (e *Engine) Summary() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.projects) == 0 {
		return "engine running, no project on screen"
	}
	var text strings.Builder
	cells := 0
	for _, p := range e.projects {
		cells += len(p.cells)
	}
	fmt.Fprintf(&text, "engine running · %d project(s) · %d cell(s)\n", len(e.projects), cells)
	for _, p := range e.projects {
		fmt.Fprintf(&text, "\n%s  %s\n", filepath.Base(p.path), p.path)
		for _, c := range p.cells {
			state := cell.Stopped
			if c.proc != nil {
				state = c.proc.State()
			}
			fmt.Fprintf(&text, "  %s %-7s %s\n", state.Marker(), c.kind, c.name)
		}
	}
	return text.String()
}

// Shutdown kills every cell and releases the files. The desired state stays
// on disk for the next startup.
func (e *Engine) Shutdown() {
	e.stopWatcher()
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, p := range e.projects {
		for _, c := range p.cells {
			if c.proc != nil {
				_ = c.proc.Kill()
			}
			if c.record != nil {
				_ = c.record.Close()
			}
		}
	}
}

// Version goes up on every change. Each connected screen compares it against
// what it already drew, so more than one screen can follow the same engine.
func (e *Engine) Version() uint64 { return e.version.Load() }

func (e *Engine) markDirty() { e.version.Add(1) }

// SwitchScreen remembers the chosen screen across runs.
func (e *Engine) SwitchScreen(screen string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.screen = screen
	e.save()
}

// Screen is the last chosen screen.
func (e *Engine) Screen() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.screen
}

// save writes the desired state. Always called with the lock held.
func (e *Engine) save() {
	state := DesiredState{Screen: e.screen}
	for _, p := range e.projects {
		saved := SavedProject{ID: p.id, Path: p.path, Color: p.color}
		for _, c := range p.cells {
			savedCell := SavedCell{ID: c.id, Kind: c.kind, Name: c.name, Target: c.target, Tab: c.tab}
			if len(c.conversations) > 0 {
				savedCell.Conversations = c.conversations
			}
			saved.Cells = append(saved.Cells, savedCell)
		}
		state.Projects = append(state.Projects, saved)
	}
	if err := Save(StatePath(e.stateDir), state); err != nil {
		e.notice = "could not write the state: " + err.Error()
	}
}

func (e *Engine) findCell(id string) *liveCell {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, p := range e.projects {
		for _, c := range p.cells {
			if c.id == id {
				return c
			}
		}
	}
	return nil
}

func (e *Engine) findProjectByPath(path string) *project {
	for _, p := range e.projects {
		if p.path == path {
			return p
		}
	}
	return nil
}

// autoName gives the next free name of the type within the project.
func autoName(p *project, kind string) string {
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("%s %d", kind, n)
		free := true
		for _, c := range p.cells {
			if c.name == candidate {
				free = false
				break
			}
		}
		if free {
			return candidate
		}
	}
}

// validatePath requires a directory that exists and is writable, right now.
func validatePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("the project needs a path")
	}
	if strings.HasPrefix(raw, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			raw = filepath.Join(home, strings.TrimPrefix(raw, "~"))
		}
	}
	path, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("invalid path: %s", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("directory does not exist: %s", path)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", path)
	}
	if err := syscall.Access(path, 2 /* W_OK */); err != nil {
		return "", fmt.Errorf("no write permission on: %s", path)
	}
	return path, nil
}

// newIdentity is the stable identity of a cell or project — it is not the
// process, and it survives it.
func newIdentity() string {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", os.Getpid())
	}
	return hex.EncodeToString(raw)
}
