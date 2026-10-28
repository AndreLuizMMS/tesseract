// Package engine owns everything: the processes, each cell's internal
// screen, the history and the desired state. The screen is a client — it
// draws what the engine tells it to and hands back keystrokes. No rule lives
// in the screen.
package engine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/docker"
	"github.com/andreluiz/tesseract/internal/engine/history"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// Engine holds the live grid.
type Engine struct {
	mu       sync.Mutex
	stateDir string
	config   Config
	projects []*project
	recents  []string
	screen   string
	notice   string
	version  atomic.Uint64
	// discoveries numbers each conversation an agent reports, in the order
	// they were reported (see saveConversation).
	discoveries atomic.Uint64
	stop        chan struct{}
	once        sync.Once
	// down says Shutdown already ran (see save).
	down bool
	// accounts says which Claude account a cell born now runs on.
	accounts *accounts
}

type project struct {
	id    string
	path  string
	color int
	// compose is the file detected at the root; empty when the project has none.
	compose string
	// docker is the stack summary for the strip, refreshed from time to time.
	docker string
	// dockerDown says a service of the stack fell on its own, as of the same
	// reading.
	dockerDown bool
	// orphan marks that the directory vanished from disk. The processes may
	// well be alive in there: it's the project that's gone, and every cell
	// of it shows that instead of its own state.
	orphan bool
	cells  []*liveCell
}

type liveCell struct {
	id            string
	kind          string
	name          string
	target        string
	tab           string
	conversations map[string]string
	// discovered is the ticket of the last conversation saved for each tab.
	discovered map[string]uint64
	proc       cell.Cell
	record     *history.History
	// lastState is what the watcher saw last time, so it only notifies the
	// change and not the idle state.
	lastState cell.State
	// lastTab is the tab that state was read from, on a cell with tabs.
	lastTab string
	// pendingRename marks that an automatic /rename was sent and the watcher
	// needs to adopt the name as soon as it reaches the transcript.
	// renameFrom is the title the conversation had before — the new one is
	// whatever differs — and renameUntil is when the wait gives up.
	pendingRename bool
	renameFrom    string
	renameUntil   time.Time
	// detail is what the cell's mod said about the state it last reported —
	// the command waiting for approval, the opening of the answer — and
	// detailTab is the tab it spoke for. The watcher puts it in the notice.
	detail, detailTab string
	// written is what the cell's tool mods said its agent wrote last.
	written writtenFiles
	// said is the same word kept for the screen, and context how full each
	// tab's conversation window is (see said.go).
	said    said
	context map[string]int
}

// New prepares an engine over a state directory.
func New(stateDir string, config Config) *Engine {
	if config.HistoryLimit <= 0 {
		config.HistoryLimit = history.DefaultCap
	}
	if config.ScrollbackLines <= 0 {
		config.ScrollbackLines = DefaultScrollback
	}
	if config.Agents == nil {
		config.Agents = map[string]AgentProfile{}
	}
	return &Engine{
		stateDir: stateDir,
		config:   config,
		screen:   "grid",
		stop:     make(chan struct{}),
		accounts: newAccounts(config.ClaudeAccounts),
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
	e.recents = state.Recents
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
			if err := e.wake(p, c, ""); err != nil {
				e.notice = err.Error()
			}
			p.cells = append(p.cells, c)
		}
		e.projects = append(e.projects, p)
	}
	// State written before recents existed still knows which directories were
	// in use — the projects it reconstructed.
	if len(e.recents) == 0 {
		for _, p := range e.projects {
			e.recents = append(e.recents, p.path)
		}
	}
	// Only over a state that was read whole: sweeping what a half-read file
	// doesn't mention would throw away live conversations.
	if loadErr == nil {
		e.sweepHistories()
	}
	e.markDirty()
	return loadErr
}

// sweepHistories deletes the recordings of cells that no longer exist. A
// cell used to take only its own file when it died, and every tab it had
// open left one behind — after a few months that's hundreds of megabytes of
// conversations nobody can reach any more. Called with the lock held.
func (e *Engine) sweepHistories() {
	alive := map[string]bool{}
	for _, p := range e.projects {
		for _, c := range p.cells {
			alive[c.id] = true
		}
	}
	found, err := filepath.Glob(filepath.Join(HistoryDir(e.stateDir), "*.log"))
	if err != nil {
		return
	}
	for _, file := range found {
		name := strings.TrimSuffix(filepath.Base(file), ".log")
		id, _, _ := strings.Cut(name, "-")
		if !alive[id] {
			_ = os.Remove(file)
		}
	}
}

// wake starts a reconstructed cell's process, with prompt as the first thing
// its agent hears. Called with the lock held.
func (e *Engine) wake(p *project, c *liveCell, prompt string) error {
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
		ID:         c.id,
		Directory:  p.path,
		Name:       c.name,
		Target:     c.target,
		Prompt:     prompt,
		History:    record,
		Columns:    80,
		Lines:      24,
		Notify:     e.markDirty,
		Scrollback: e.config.ScrollbackLines,
		SleepAfter: time.Duration(e.config.SleepAfterMinutes) * time.Minute,

		Tab:           c.tab,
		Conversation:  conversations[c.kind],
		Conversations: conversations,
		// Off the lock: whoever calls this may be the cell's own birth, which
		// is already holding the engine's lock.
		// Each discovery is saved by its own goroutine, and they don't run in
		// the order they were made: the ticket, taken here, is what keeps the
		// id the agent was born with from landing after the one its mod
		// reported and putting the old conversation back.
		OnConversationDiscovered: func(tab, id string) {
			go e.saveConversation(cellID, tab, id, e.discoveries.Add(1))
		},

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
	claude := profiles["claude"]
	claude.Args = append(append([]string{}, claude.Args...), e.claudeArgs(claude.Args)...)
	if folders := e.accounts.list(); len(folders) > 0 {
		claude.Accounts = folders
		claude.Account = e.accounts.current
	}
	profiles["claude"] = claude
	return profiles
}

// saveConversation records the identity of the conversation the agent
// opened, so it can be reattached after a crash. On a cell with tabs, each
// tab has its own.
func (e *Engine) saveConversation(cellID, tab, conversation string, ticket uint64) {
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
			if c.discovered == nil {
				c.discovered = map[string]uint64{}
			}
			if ticket < c.discovered[tab] {
				return // a newer discovery got here first
			}
			c.discovered[tab] = ticket
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

	if err := e.wake(p, c, ""); err != nil {
		if c.record != nil {
			c.record.Close()
		}
		return "", err
	}

	p.cells = append(p.cells, c)
	if newProject {
		e.projects = append(e.projects, p)
	}
	e.remember(path)
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
			for _, file := range HistoryFiles(e.stateDir, c.id) {
				_ = os.Remove(file)
			}
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
		return fmt.Errorf("Name can't be empty. ctrl+r asks the agent for one.")
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
//
// The identity of the grid is read under the lock; each cell's portrait is
// taken outside it. Drawing waits on the same latch the process holds while
// writing to its own screen, so doing it under the engine's lock would put
// every key, every creation and every kill in line behind the whole grid's
// portrait — with a dozen busy cells that was a tenth of a second per frame,
// fifty times a second.
func (e *Engine) Snapshot() protocol.State {
	type portrait struct {
		project, cell int
		proc          cell.Cell
		orphan        bool
		said          said
		context       map[string]int
	}

	e.mu.Lock()
	snapshot := protocol.State{Notice: e.notice, Screen: e.screen, Types: typeCards(), Recents: append([]string(nil), e.recents...)}
	var pending []portrait
	for _, p := range e.projects {
		drawing := protocol.Project{
			ID:         p.id,
			Path:       p.path,
			Name:       filepath.Base(p.path),
			Color:      p.color,
			HasCompose: p.compose != "",
			Docker:     p.docker,
			DockerDown: p.dockerDown,
		}
		for _, c := range p.cells {
			if c.proc != nil {
				pending = append(pending, portrait{
					project: len(snapshot.Projects),
					cell:    len(drawing.Cells),
					proc:    c.proc,
					orphan:  p.orphan,
					said:    c.said,
					context: maps.Clone(c.context),
				})
			}
			drawing.Cells = append(drawing.Cells, protocol.Cell{
				ID:     c.id,
				Type:   c.kind,
				Name:   c.name,
				State:  string(p.restState()),
				Live:   true,
				Doc:    inProject(p.path, c.written.doc),
				Edited: c.written.edited(p.path),
			})
		}
		snapshot.Projects = append(snapshot.Projects, drawing)
	}
	e.mu.Unlock()

	snapshot.Quota = ReadQuota()
	for _, next := range pending {
		cellSnapshot := &snapshot.Projects[next.project].Cells[next.cell]
		if withTabs, ok := next.proc.(cell.WithTabs); ok {
			cellSnapshot.Tabs = withTabs.Tabs()
			cellSnapshot.Tab = withTabs.ActiveTab()
		}
		if withSleep, ok := next.proc.(cell.WithSleep); ok {
			cellSnapshot.Asleep = withSleep.AsleepTabs()
		}
		frame := next.proc.Draw()
		if !next.orphan {
			cellSnapshot.State = string(next.proc.State())
		}
		cellSnapshot.Detail, cellSnapshot.Ask = next.said.saidFor(cellSnapshot.State, cellSnapshot.Tab, cellSnapshot.Tabs != nil)
		cellSnapshot.Context = next.context[cellSnapshot.Tab]
		cellSnapshot.Lines = frame.Lines
		cellSnapshot.CursorX = frame.CursorX
		cellSnapshot.CursorY = frame.CursorY
		cellSnapshot.Scroll = frame.Scroll
		cellSnapshot.Live = frame.Live
	}
	return snapshot
}

// nextColor picks the new project's hue: the one the fewest projects are
// wearing, the first of them in the theme's order. While there's a hue
// nobody has, no two projects look the same; once they're all taken, the new
// project — which lands at the end of the mosaic — at least never repeats the
// one right before it.
func (e *Engine) nextColor() int {
	var worn [theme.HueCount]int
	last := -1
	for _, p := range e.projects {
		last = ((p.color % theme.HueCount) + theme.HueCount) % theme.HueCount
		worn[last]++
	}
	best := -1
	for hue, count := range worn {
		if hue != last && (best < 0 || count < worn[best]) {
			best = hue
		}
	}
	return best
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
	fmt.Fprintf(&text, "engine running · %s · %s\n",
		plural(len(e.projects), "project", "projects"), plural(cells, "cell", "cells"))
	for _, p := range e.projects {
		fmt.Fprintf(&text, "\n%s  %s\n", filepath.Base(p.path), p.path)
		for _, c := range p.cells {
			state := p.restState()
			if c.proc != nil && !p.orphan {
				state = c.proc.State()
			}
			// The glyph and its word: a terminal that prints this has no
			// legend to look the glyph up in. The kind is only said when
			// the name doesn't already ("session 1" is a session).
			label := c.name
			if !strings.HasPrefix(c.name, c.kind) {
				label += "  · " + c.kind
			}
			fmt.Fprintf(&text, "  %s %-11s  %s\n", state.Marker(), state.Word(), label)
		}
	}
	return text.String()
}

// restState is a cell's state before its process is asked: orphaned when
// the project's directory is gone, stopped otherwise.
func (p *project) restState() cell.State {
	if p.orphan {
		return cell.Orphaned
	}
	return cell.Stopped
}

// plural writes the number next to the noun in the right form.
func plural(count int, one, many string) string {
	if count == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", count, many)
}

// Shutdown kills every cell and releases the files. The desired state stays
// on disk for the next startup.
func (e *Engine) Shutdown() {
	e.stopWatcher()
	e.mu.Lock()
	defer e.mu.Unlock()
	e.down = true
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
//
// An engine that was shut down writes nothing more: what's on disk is the
// grid as it stood. A conversation an agent reports at birth is saved by a
// goroutine of its own, and one that lost the race against Shutdown used to
// write into a state directory whoever stopped the engine was already done
// with.
func (e *Engine) save() {
	if e.down {
		return
	}
	state := DesiredState{Screen: e.screen, Recents: e.recents}
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

// recentsCap is how many directories the form remembers. Long enough to
// cover what someone actually rotates between, short enough to stay a list
// and not an archive.
const recentsCap = 12

// remember puts the directory at the top of the recents, without repeating
// it. Called with the lock held.
func (e *Engine) remember(path string) {
	recents := []string{path}
	for _, previous := range e.recents {
		if previous != path && len(recents) < recentsCap {
			recents = append(recents, previous)
		}
	}
	e.recents = recents
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
// Its errors land on the screen's footer after the form has closed, so each
// one is a whole sentence that ends on the key that fixes it.
func validatePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("Project needs a path. n opens the form again.")
	}
	if strings.HasPrefix(raw, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			raw = filepath.Join(home, strings.TrimPrefix(raw, "~"))
		}
	}
	path, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("Invalid path: %s. n picks a folder from the list.", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("No directory at %s. n picks another folder.", path)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is a file, not a directory. n picks a folder.", path)
	}
	if err := syscall.Access(path, 2 /* W_OK */); err != nil {
		return "", fmt.Errorf("No write permission on %s. n picks another folder.", path)
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
