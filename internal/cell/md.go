package cell

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/fsnotify/fsnotify"
)

func init() {
	Register(Descriptor{
		Type:         "md",
		Order:        50,
		TargetLabel:  "MD",
		TargetIsPath: true,
	}, func() Cell { return &Md{} })
}

const (
	// diskDebounce merges several changes to the file in a row into a
	// single reload — an editor tends to write in two or three steps.
	diskDebounce = 80 * time.Millisecond
	// scanInterval is how often the tab looks for new files in the project.
	// Deliberately spaced out: the scan also happens every time the tab
	// comes back into view, which is when it matters.
	scanInterval = 60 * time.Second
	// maxDepth is how deep the search goes from the project's root.
	maxDepth = 6
	// maxFiles keeps a giant project from stalling the scan.
	maxFiles = 2000
)

// skipDirs don't hold project documentation and are expensive to scan.
var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true, "dist": true,
	"build": true, "target": true, ".next": true, ".venv": true,
	"venv": true, "coverage": true, ".cache": true, ".idea": true,
}

// Md is the markdown tab: a list of the project's files with a search box on
// top, and the chosen file rendered. Read-only — the cell never edits
// anything.
type Md struct {
	mu        sync.Mutex
	directory string
	files     []string // paths relative to the project's root
	filter    string
	selected  int
	// open is the file being read; empty means the list is on screen.
	open     string
	content  []string
	scroll   int
	columns  int
	lines    int
	state    State
	notify   func()
	watcher  *fsnotify.Watcher
	watching string
	stop     chan struct{}
}

func (m *Md) Spawn(cfg Config) error {
	m.directory = cfg.Directory
	m.columns, m.lines = max(cfg.Columns, 40), max(cfg.Lines, 10)
	m.notify = cfg.Notify
	m.stop = make(chan struct{})
	m.state = Stopped

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	m.watcher = watcher

	m.scan()
	// Created pointing at a file, the tab opens straight into it; without a
	// target, it starts on the list.
	if cfg.Target != "" {
		m.openFile(cfg.Target)
	}

	go m.watchDisk()
	go m.scanPeriodically()
	return nil
}

// scan looks for the project's markdown files.
func (m *Md) scan() {
	m.mu.Lock()
	root := m.directory
	m.mu.Unlock()

	var found []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relative, relativeErr := filepath.Rel(root, path)
		if relativeErr != nil {
			return nil
		}
		if entry.IsDir() {
			name := entry.Name()
			if path == root {
				return nil
			}
			if skipDirs[name] || (strings.HasPrefix(name, ".") && name != ".github") {
				return filepath.SkipDir
			}
			if strings.Count(relative, string(os.PathSeparator)) >= maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMarkdown(entry.Name()) {
			return nil
		}
		found = append(found, relative)
		if len(found) >= maxFiles {
			return filepath.SkipAll
		}
		return nil
	})

	// Shallower first: the README at the root matters more than one four
	// levels down.
	sort.Slice(found, func(i, j int) bool {
		depthI := strings.Count(found[i], string(os.PathSeparator))
		depthJ := strings.Count(found[j], string(os.PathSeparator))
		if depthI != depthJ {
			return depthI < depthJ
		}
		return found[i] < found[j]
	})

	m.mu.Lock()
	m.files = found
	m.selected = min(m.selected, max(len(m.filtered())-1, 0))
	m.mu.Unlock()
}

func isMarkdown(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

// filtered are the files that match the search. Called with the lock held.
func (m *Md) filtered() []string {
	if strings.TrimSpace(m.filter) == "" {
		return m.files
	}
	target := strings.ToLower(m.filter)
	var matched []string
	for _, file := range m.files {
		if strings.Contains(strings.ToLower(filepath.Base(file)), target) {
			matched = append(matched, file)
			continue
		}
		// The path counts too: whoever types "docs/spec" knows what they
		// want.
		if strings.Contains(strings.ToLower(file), target) {
			matched = append(matched, file)
		}
	}
	return matched
}

// openFile puts a file into reading mode.
func (m *Md) openFile(path string) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.directory, path)
	}
	m.mu.Lock()
	m.open = path
	m.scroll = 0
	m.mu.Unlock()

	m.watchFile(path)
	m.reload()
}

// closeFile sends the tab back to the list.
func (m *Md) closeFile() {
	m.mu.Lock()
	m.open, m.content, m.scroll, m.state = "", nil, 0, Stopped
	m.mu.Unlock()
}

// watchFile starts watching the open file's directory. It's the directory
// and not the file because an editor that saves by swapping the file for
// another makes a file watcher lose track.
func (m *Md) watchFile(path string) {
	folder := filepath.Dir(path)
	m.mu.Lock()
	previous, watcher := m.watching, m.watcher
	m.watching = folder
	m.mu.Unlock()

	if watcher == nil || previous == folder {
		return
	}
	if previous != "" {
		_ = watcher.Remove(previous)
	}
	_ = watcher.Add(folder)
}

func (m *Md) watchDisk() {
	var wait <-chan time.Time
	for {
		select {
		case <-m.stop:
			return
		case event, open := <-m.watcher.Events:
			if !open {
				return
			}
			m.mu.Lock()
			relevant := m.open != "" && filepath.Clean(event.Name) == filepath.Clean(m.open)
			m.mu.Unlock()
			if !relevant {
				continue
			}
			wait = time.After(diskDebounce)
		case <-m.watcher.Errors:
			continue
		case <-wait:
			wait = nil
			m.reload()
			if m.notify != nil {
				m.notify()
			}
		}
	}
}

func (m *Md) scanPeriodically() {
	clock := time.NewTicker(scanInterval)
	defer clock.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-clock.C:
			m.scan()
		}
	}
}

// reload loads the open file and renders it at the current width.
func (m *Md) reload() {
	m.mu.Lock()
	path, columns := m.open, m.columns
	m.mu.Unlock()
	if path == "" {
		return
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		m.mu.Lock()
		m.state = Crashed
		m.content = []string{"", "  couldn't read " + path, "", "  " + readableError(err)}
		m.scroll = 0
		m.mu.Unlock()
		return
	}

	page := renderPage(string(raw), columns)
	m.mu.Lock()
	m.state = Stopped
	m.content = page
	if m.scroll > len(m.content) {
		m.scroll = 0
	}
	m.mu.Unlock()
}

func readableError(err error) string {
	if os.IsNotExist(err) {
		return "the file is gone from disk"
	}
	if os.IsPermission(err) {
		return "no read permission"
	}
	return err.Error()
}

func (m *Md) Draw() Frame {
	m.mu.Lock()
	defer m.mu.Unlock()

	frame := Frame{CursorX: -1, CursorY: -1, Scroll: m.scroll, Live: m.scroll == 0}
	if m.open == "" {
		frame.Lines = m.drawList()
		return frame
	}

	frame.Lines = append(frame.Lines, m.pageHeader())
	for i := range max(m.lines-1, 1) {
		line := m.scroll + i
		if line >= len(m.content) {
			break
		}
		frame.Lines = append(frame.Lines, m.content[line])
	}
	return frame
}

// pageHeader is the document's spine: the file, where the reading is and how
// to go back. Called with the lock held.
func (m *Md) pageHeader() string {
	file := relativeTo(m.directory, m.open)
	from := m.scroll + 1
	to := min(m.scroll+max(m.lines-1, 1), len(m.content))
	position := strconv.Itoa(from) + "–" + strconv.Itoa(to) + " of " + strconv.Itoa(len(m.content))
	back := "esc back to list"

	left := " " + file + " "
	right := " " + position + " · " + back + " "
	filler := max(m.columns-len([]rune(left))-len([]rune(right)), 1)
	return pathColor + left + mdDimColor + strings.Repeat("─", filler) + right + colorReset
}

// drawList assembles the search box on top and the files below. Called with
// the lock held.
func (m *Md) drawList() []string {
	matched := m.filtered()
	lines := []string{
		searchColor + " search: " + colorReset + m.filter + "▏",
		mdDimColor + " " + strings.Repeat("─", max(m.columns-2, 1)) + colorReset,
		mdDimColor + " " + strconv.Itoa(len(matched)) + " of " + strconv.Itoa(len(m.files)) +
			" documents · ↑↓ pick · ↵ open" + colorReset,
		"",
	}

	fit := max(m.lines-len(lines), 1)
	first := min(max(m.selected-fit/2, 0), max(len(matched)-fit, 0))
	for i := first; i < len(matched) && i-first < fit; i++ {
		lines = append(lines, m.listLine(matched[i], i == m.selected))
	}
	if len(matched) == 0 {
		lines = append(lines, mdDimColor+"  no document matches the search"+colorReset)
	}
	return lines
}

// listLine writes a file: the dimmed folder, the highlighted name.
func (m *Md) listLine(file string, selected bool) string {
	folder, name := filepath.Split(file)
	mark, nameColor := "   ", mdWhiteColor
	if selected {
		mark, nameColor = " ▸ ", selectedColor
	}
	return mark + mdDimColor + folder + nameColor + name + colorReset
}

// Colors for the markdown tab. Written here instead of in the screen
// because the cell draws its own content — the screen only receives lines
// already made.
const (
	searchColor   = "\x1b[1;38;5;252m"
	mdWhiteColor  = "\x1b[38;5;252m"
	selectedColor = "\x1b[1;38;5;42m"
	pathColor     = "\x1b[38;5;39m"
	mdDimColor    = "\x1b[38;5;240m"
	colorReset    = "\x1b[0m"
)

func relativeTo(root, path string) string {
	if relative, err := filepath.Rel(root, path); err == nil {
		return relative
	}
	return path
}

// Key is the list's search and navigation: typing filters, the arrows pick,
// and enter opens. While reading, esc goes back to the list.
func (m *Md) Key(tap Keystroke) error {
	m.mu.Lock()
	reading := m.open != ""
	m.mu.Unlock()

	if reading {
		switch tap.Code {
		case vt.KeyEscape:
			m.closeFile()
		case vt.KeyUp:
			m.Scroll(1, false)
		case vt.KeyDown:
			m.Scroll(-1, false)
		case vt.KeyPgUp:
			m.Scroll(m.height(), false)
		case vt.KeyPgDown:
			m.Scroll(-m.height(), false)
		}
		m.signal()
		return nil
	}

	switch tap.Code {
	case vt.KeyEnter:
		m.mu.Lock()
		matched := m.filtered()
		selected := ""
		if m.selected < len(matched) {
			selected = matched[m.selected]
		}
		m.mu.Unlock()
		if selected != "" {
			m.openFile(selected)
		}
	case vt.KeyUp:
		m.moveInList(-1)
	case vt.KeyDown:
		m.moveInList(1)
	case vt.KeyBackspace:
		m.mu.Lock()
		if runes := []rune(m.filter); len(runes) > 0 {
			m.filter = string(runes[:len(runes)-1])
		}
		m.selected = 0
		m.mu.Unlock()
	case vt.KeyEscape:
		m.mu.Lock()
		m.filter, m.selected = "", 0
		m.mu.Unlock()
	default:
		if tap.Paste != "" {
			m.mu.Lock()
			m.filter += tap.Paste
			m.selected = 0
			m.mu.Unlock()
			break
		}
		if tap.Text != "" {
			m.mu.Lock()
			m.filter += tap.Text
			m.selected = 0
			m.mu.Unlock()
		}
	}
	m.signal()
	return nil
}

func (m *Md) moveInList(step int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	matched := len(m.filtered())
	if matched == 0 {
		m.selected = 0
		return
	}
	m.selected = min(max(m.selected+step, 0), matched-1)
}

func (m *Md) height() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return max(m.lines-2, 1)
}

func (m *Md) signal() {
	if m.notify != nil {
		m.notify()
	}
}

// Refresh is called when the tab comes back into view: that's the right
// time to look for a new file, not once a minute in the dark.
func (m *Md) Refresh() {
	m.scan()
	m.reload()
	m.signal()
}

func (m *Md) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Md) States() []State {
	return []State{Stopped, Crashed, Orphaned}
}

func (m *Md) Resize(columns, lines int) error {
	m.mu.Lock()
	widthChanged := columns != m.columns
	m.columns, m.lines = columns, lines
	m.mu.Unlock()
	if widthChanged {
		m.reload()
	}
	m.signal()
	return nil
}

// Scroll moves through the open file's text. Here, scrolling up goes back to
// the start.
func (m *Md) Scroll(delta int, live bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if live {
		m.scroll = 0
		return
	}
	if m.open == "" {
		return
	}
	ceiling := max(len(m.content)-m.lines, 0)
	m.scroll = min(max(m.scroll-delta, 0), ceiling)
}

func (m *Md) Kill() error {
	select {
	case <-m.stop:
	default:
		close(m.stop)
	}
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}
