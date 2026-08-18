package screen

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// linesPerScroll is how much the mouse wheel moves at a time.
const linesPerScroll = 3

// spinCadence is how often the work-in-progress drawing advances a frame.
const spinCadence = 120 * time.Millisecond

// spinsPerCheck is how many frames pass between the screen asking the engine
// how the stack is doing, while Docker works. That way services turn green
// one at a time on screen, instead of everything changing at once at the
// end.
const spinsPerCheck = 10

// The blocking state's status bar blinks every 2 seconds: 1.8s on, 200ms
// off. It's the fourth signal for "approve", alongside the shape, the area
// and the reverse video — and it's the only motion in the app. The clock
// only runs while a cell is locked; with none, it stops and the screen goes
// still again.
const (
	warnOn  = 1800 * time.Millisecond
	warnOff = 200 * time.Millisecond
)

const (
	viewGrid = "grid"
	viewList = "list"
)

type stateArrived protocol.State

type completionArrived protocol.Completed

type resultsArrived protocol.Matches

type servicesArrived protocol.Services

type errorArrived string

type engineDied struct{}

// panelTick moves the Docker panel's work-in-progress drawing.
type panelTick struct{ count int }

// warnTick flips the blocking status bar's frame.
type warnTick struct{}

// Model is the screen. It holds what the engine sent to draw, the keyboard
// mode and where the focus is — no business rule lives here.
type Model struct {
	client *Client
	state  protocol.State
	focus  Focus
	view   string
	width  int
	height int
	err    string

	typing    bool
	sizes     map[string]Dims
	selection *Selection

	panel        *DockerPanel
	form         *Form
	question     *Question
	confirmation *Confirmation
	help         bool
	results      *protocol.Matches
	resultChosen int
	// blinking prevents two blocking-status-bar clocks from running at once.
	blinking bool
}

// blinkWarning schedules the next frame of the blocking status bar, and
// returns nothing when there's nothing to blink — that's how the clock
// stops on its own.
func (m *Model) blinkWarning() tea.Cmd {
	if !theme.BlinkOn() || !m.hasBlockedCell() {
		m.blinking, theme.Dimmed = false, false
		return nil
	}
	m.blinking = true
	wait := warnOn
	if theme.Dimmed {
		wait = warnOff
	}
	return tea.Tick(wait, func(time.Time) tea.Msg { return warnTick{} })
}

// hasBlockedCell says whether any cell in the grid is waiting on you.
func (m *Model) hasBlockedCell() bool {
	for _, project := range m.state.Projects {
		for _, item := range project.Cells {
			if item.State == "approve" && item.Live {
				return true
			}
		}
	}
	return false
}

// NewModel prepares the screen connected to an already-connected engine,
// starting from the first snapshot it sent.
func NewModel(client *Client, initial protocol.State) *Model {
	view := initial.Screen
	if view != viewList {
		view = viewGrid
	}
	return &Model{
		client: client,
		state:  initial,
		view:   view,
		sizes:  map[string]Dims{},
	}
}

// Listen passes the engine's messages into the screen's program.
func (m *Model) Listen(program *tea.Program) {
	go func() {
		for {
			envelope, err := m.client.Receive()
			if err != nil {
				program.Send(engineDied{})
				return
			}
			switch envelope.Type {
			case protocol.TypeState:
				if state, err := protocol.Unpack[protocol.State](envelope); err == nil {
					program.Send(stateArrived(state))
				}
			case protocol.TypeCompleted:
				if response, err := protocol.Unpack[protocol.Completed](envelope); err == nil {
					program.Send(completionArrived(response))
				}
			case protocol.TypeServices:
				if services, err := protocol.Unpack[protocol.Services](envelope); err == nil {
					program.Send(servicesArrived(services))
				}
			case protocol.TypeMatches:
				if results, err := protocol.Unpack[protocol.Matches](envelope); err == nil {
					program.Send(resultsArrived(results))
				}
			case protocol.TypeError:
				if e, err := protocol.Unpack[protocol.Error](envelope); err == nil {
					program.Send(errorArrived(e.Message))
				}
			}
		}
	}()
}

// Init starts the warning bar's clock when the grid already opens with a
// locked cell — the case of someone who closed the screen and came back
// later.
func (m *Model) Init() tea.Cmd { return m.blinkWarning() }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.reportSizes()

	case stateArrived:
		m.state = protocol.State(msg)
		m.focus = Adjust(m.state, m.focus)
		m.reportSizes()
		if !m.blinking {
			return m, m.blinkWarning()
		}

	case completionArrived:
		if m.form != nil {
			m.form.Completed(protocol.Completed(msg))
		}

	case servicesArrived:
		if m.panel != nil {
			m.panel.Received(protocol.Services(msg))
		}

	case resultsArrived:
		results := protocol.Matches(msg)
		m.results, m.resultChosen = &results, 0

	case errorArrived:
		m.err = string(msg)

	case engineDied:
		return m, tea.Quit

	case panelTick:
		return m, m.spinPanel(msg)

	case warnTick:
		theme.Dimmed = !theme.Dimmed
		return m, m.blinkWarning()

	case tea.MouseWheelMsg:
		return m, m.scroll(msg)

	case tea.MouseClickMsg:
		m.startSelection(msg.Button, msg.X, msg.Y)

	case tea.MouseMotionMsg:
		m.dragSelection(msg.X, msg.Y)

	case tea.MouseReleaseMsg:
		return m, m.copySelection()

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.PasteMsg:
		return m, m.handlePaste(msg.Content)
	}
	return m, nil
}

// Mode says who owns the keyboard right now — what the title bar shows.
func (m *Model) Mode() keyboard.Mode {
	switch {
	case m.panel != nil:
		return keyboard.DockerPanel
	case m.form != nil || m.question != nil || m.confirmation != nil:
		return keyboard.Form
	case m.typing:
		return keyboard.Type
	}
	return keyboard.Browse
}

// handleKey hands the key to whoever owns it right now. There are never two
// owners at once, so a collision is structurally impossible.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	erased := msg.Code == tea.KeyBackspace

	switch {
	case m.panel != nil:
		return m, m.handlePanel(key)
	case m.form != nil:
		return m, m.handleForm(key, msg.Text, erased)
	case m.question != nil:
		return m, m.handleQuestion(key, msg.Text, erased)
	case m.confirmation != nil:
		return m, m.handleConfirmation(key)
	case m.help:
		if action := keyboard.Lookup(keyboard.Browse, key); action == keyboard.Back || action == keyboard.Help {
			m.help = false
		}
		return m, nil
	case m.results != nil:
		return m, m.handleResults(key)
	case m.typing:
		return m, m.handleTyping(key, msg)
	}
	return m.handleNavigate(key)
}

// handlePaste hands the pasted text to whoever owns the keyboard now. Paste
// doesn't arrive as a key — the terminal sends the whole content at once —
// so it skips handleKey; but it obeys the same single owner.
func (m *Model) handlePaste(text string) tea.Cmd {
	if text == "" {
		return nil
	}
	switch {
	case m.form != nil:
		return m.handleForm("", oneLine(text), false)
	case m.question != nil:
		return m.handleQuestion("", oneLine(text), false)
	case m.typing:
		focus := m.focusedCell()
		if focus == nil {
			return nil
		}
		m.send(protocol.TypeKey, protocol.Key{Cell: focus.ID, Paste: text})
	}
	return nil
}

// oneLine flattens pasted text to fit a single-line field.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func (m *Model) handleTyping(key string, msg tea.KeyPressMsg) tea.Cmd {
	if keyboard.Lookup(keyboard.Type, key) == keyboard.ExitType {
		m.typing = false
		return nil
	}
	// Everything else goes to the cell, no exceptions.
	focus := m.focusedCell()
	if focus == nil {
		return nil
	}
	m.send(protocol.TypeKey, protocol.Key{
		Cell: focus.ID,
		Code: msg.Code,
		Text: msg.Text,
		Mod:  int(msg.Mod),
	})
	return nil
}

func (m *Model) handleNavigate(key string) (tea.Model, tea.Cmd) {
	m.err = ""
	action := keyboard.Lookup(keyboard.Browse, key)
	project, item := m.currentFocus()

	switch action {
	case keyboard.CellPrevious:
		m.moveDirection(-1, 0)
	case keyboard.CellNext:
		m.moveDirection(1, 0)
	case keyboard.CellUp:
		m.moveDirection(0, -1)
	case keyboard.CellDown:
		m.moveDirection(0, 1)
	case keyboard.GoToProject:
		if n, err := strconv.Atoi(key); err == nil {
			m.focus.Project, m.focus.Cell = n-1, 0
		}
	case keyboard.SkipToAlert:
		m.jumpToCaller()
	case keyboard.NextTab:
		if item != nil {
			m.send(protocol.TypeTab, protocol.Tab{Cell: item.ID, Step: 1})
		}
	case keyboard.PreviousTab:
		if item != nil {
			m.send(protocol.TypeTab, protocol.Tab{Cell: item.ID, Step: -1})
		}

	case keyboard.EnterType:
		if item != nil {
			m.typing = true
		}
	case keyboard.Fullscreen:
		m.focus.Full = !m.focus.Full
	case keyboard.SwitchScreen:
		m.view = viewList
		if !m.showingGrid() {
			m.view = viewGrid
		}
		m.send(protocol.TypeScreen, protocol.Screen{Screen: m.view})

	case keyboard.Create:
		path := ""
		if project != nil {
			path = project.Path
		}
		m.form = NewForm(m.state.Types, path)
	case keyboard.Resume:
		if item != nil {
			m.send(protocol.TypeResume, protocol.Resume{Cell: item.ID})
		}
	case keyboard.Kill:
		if item != nil {
			m.confirmation = killConfirmation(*project, *item)
		}

	case keyboard.Rename:
		if item != nil {
			m.send(protocol.TypeRename, protocol.Rename{Cell: item.ID})
		}

	case keyboard.Prompt:
		if item != nil {
			m.question = &Question{
				Title: "PROMPT · " + item.Name, Label: "PROMPT",
				Hint: "goes to the cell without you entering it", Action: keyboard.Prompt,
			}
		}
	case keyboard.OpenDocker:
		m.openDocker()
	case keyboard.OpenEditor:
		if project != nil {
			m.send(protocol.TypeEditor, protocol.Editor{Project: project.ID})
		}

	case keyboard.SearchTerm:
		if item != nil {
			m.question = &Question{
				Title: "SEARCH", Label: "TERM",
				Hint: "searches the focused cell's history", Action: keyboard.SearchTerm,
			}
		}
	case keyboard.Back:
		if m.selection != nil {
			m.selection = nil
			break
		}
		if m.focus.Full {
			m.focus.Full = false
			break
		}
		if item != nil {
			m.send(protocol.TypeScroll, protocol.Scroll{Cell: item.ID, Live: true})
		}
	case keyboard.Help:
		m.help = true
	case keyboard.Close:
		return m, tea.Quit
	}

	m.focus = Adjust(m.state, m.focus)
	m.reportSizes()
	return m, nil
}

// handlePanel hands the key to the Docker panel, which owns the keyboard
// while it's open.
func (m *Model) handlePanel(key string) tea.Cmd {
	request, open := m.panel.Key(key)
	if request == nil {
		if !open {
			m.panel = nil
		}
		return nil
	}

	m.send(protocol.TypeDocker, *request)
	if !open {
		m.panel = nil
		return nil
	}
	// A real action: the panel starts saying it's working, and the screen
	// keeps drawing itself while that happens.
	if request.Action != "list" {
		m.panel.Started(request.Action, request.Service)
		return tickFrom(0)
	}
	return nil
}

// spinPanel advances the drawing and, every so often, asks the engine how
// the stack is doing.
func (m *Model) spinPanel(tick panelTick) tea.Cmd {
	if m.panel == nil || !m.panel.IsWorking() {
		return nil
	}
	m.panel.Spin()
	if tick.count%spinsPerCheck == spinsPerCheck-1 {
		m.send(protocol.TypeDocker, protocol.Docker{Project: m.panel.Project, Action: "list"})
	}
	return tickFrom(tick.count + 1)
}

func tickFrom(count int) tea.Cmd {
	return tea.Tick(spinCadence, func(time.Time) tea.Msg { return panelTick{count: count} })
}

func (m *Model) handleForm(key, text string, erased bool) tea.Cmd {
	action := keyboard.Lookup(keyboard.Form, key)
	if request, wants := m.form.WantsComplete(action); wants {
		m.send(protocol.TypeComplete, request)
		return nil
	}
	create, open := m.form.Key(action, text, erased)
	if create != nil {
		m.send(protocol.TypeCreate, *create)
	}
	if !open {
		m.form = nil
	}
	return nil
}

func (m *Model) handleQuestion(key, text string, erased bool) tea.Cmd {
	action := keyboard.Lookup(keyboard.Form, key)
	confirmed, open := m.question.Key(action, text, erased)
	if open {
		return nil
	}
	question := m.question
	m.question = nil
	if !confirmed {
		return nil
	}
	item := m.focusedCell()
	if item == nil {
		return nil
	}
	switch question.Action {
	case keyboard.Prompt:
		m.send(protocol.TypePrompt, protocol.Prompt{Cell: item.ID, Text: question.Text})
	case keyboard.SearchTerm:
		m.send(protocol.TypeSearch, protocol.Search{Cell: item.ID, Term: question.Text})
	}
	return nil
}

func (m *Model) handleConfirmation(key string) tea.Cmd {
	action := keyboard.Lookup(keyboard.Form, key)
	if action != keyboard.Confirm && action != keyboard.Cancel {
		return nil
	}
	confirmation := m.confirmation
	m.confirmation = nil
	if action != keyboard.Confirm {
		return nil
	}
	if confirmation.Action == keyboard.Kill {
		m.send(protocol.TypeKill, protocol.Kill{Cell: confirmation.Target})
	}
	return nil
}

func (m *Model) handleResults(key string) tea.Cmd {
	switch keyboard.Lookup(keyboard.Browse, key) {
	case keyboard.CellPrevious:
		m.resultChosen = max(m.resultChosen-1, 0)
	case keyboard.CellNext:
		m.resultChosen = min(m.resultChosen+1, max(len(m.results.Lines)-1, 0))
	case keyboard.Back:
		m.results = nil
	case keyboard.EnterType:
		if len(m.results.Lines) > 0 {
			m.send(protocol.TypeGoToLine, protocol.GoToLine{
				Cell: m.results.Cell,
				Line: m.results.Lines[m.resultChosen].Line,
			})
		}
		m.results = nil
	}
	return nil
}

// moveDirection follows the arrow literally: on the grid, the focus goes to
// the cell that's literally on that side in the drawing. When there's none
// — the edge of the screen, or a row that didn't fit — the focus takes one
// step in project order, so no cell is ever out of reach. On the list,
// which is a single column, the up and down arrows keep switching project.
func (m *Model) moveDirection(dx, dy int) {
	if m.showingGrid() {
		if neighbor, has := Neighbor(m.state, m.focus, m.width, m.height, dx, dy); has {
			m.focus.Project, m.focus.Cell = neighbor.Project, neighbor.Cell
			return
		}
		m.moveThroughCells(dx + dy)
		return
	}
	if dy != 0 {
		m.focus.Project, m.focus.Cell = m.focus.Project+dy, 0
		return
	}
	m.moveThroughCells(dx)
}

// moveThroughCells walks the whole grid, crossing project boundaries: on the
// grid every cell is in view, so moving between them doesn't stop at a
// project's edge.
func (m *Model) moveThroughCells(step int) {
	total := m.totalCells()
	if total == 0 {
		return
	}
	m.focus.Project, m.focus.Cell = m.positionOf((m.linearPosition() + step + total) % total)
}

func (m *Model) totalCells() int {
	total := 0
	for _, project := range m.state.Projects {
		total += len(project.Cells)
	}
	return total
}

// jumpToCaller goes to the next cell asking for attention, crossing project
// boundaries.
func (m *Model) jumpToCaller() {
	calls := func(state string) bool { return state == "answered" || state == "approve" }
	total := m.totalCells()
	if total == 0 {
		return
	}
	current := m.linearPosition()
	for step := 1; step <= total; step++ {
		p, c := m.positionOf((current + step) % total)
		if calls(m.state.Projects[p].Cells[c].State) {
			m.focus.Project, m.focus.Cell = p, c
			return
		}
	}
}

func (m *Model) linearPosition() int {
	linear := 0
	for i, project := range m.state.Projects {
		if i == m.focus.Project {
			return linear + m.focus.Cell
		}
		linear += len(project.Cells)
	}
	return 0
}

func (m *Model) positionOf(linear int) (int, int) {
	for i, project := range m.state.Projects {
		if linear < len(project.Cells) {
			return i, linear
		}
		linear -= len(project.Cells)
	}
	return 0, 0
}

// scroll moves the history reading. The mouse wheel works in both views,
// since it isn't a key.
func (m *Model) scroll(msg tea.MouseWheelMsg) tea.Cmd {
	focus := m.focusedCell()
	if focus == nil {
		return nil
	}
	if m.selection != nil {
		// Scrolling mid-drag would move the text under the mark. After
		// copying, scrolling clears the mark: it would stay lit over
		// different text.
		if m.selection.Dragging {
			return nil
		}
		m.selection = nil
	}
	delta := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = linesPerScroll
	case tea.MouseWheelDown:
		delta = -linesPerScroll
	default:
		return nil
	}
	m.send(protocol.TypeScroll, protocol.Scroll{Cell: focus.ID, Delta: delta})
	return nil
}

// startSelection anchors the mark where the button went down. Clicking a
// cell also selects it — except in TYPE, where switching keyboard owners
// without asking would be a surprise.
func (m *Model) startSelection(button tea.MouseButton, x, y int) {
	if button != tea.MouseLeft {
		return
	}
	m.selection = nil
	id, cx, cy, found := m.cellUnderPoint(x, y)
	if !found {
		return
	}
	if !m.typing {
		m.focusCell(id)
	} else if focus := m.focusedCell(); focus == nil || focus.ID != id {
		return
	}
	m.selection = &Selection{Cell: id, AnchorX: cx, AnchorY: cy, CurrentX: cx, CurrentY: cy, Dragging: true}
}

// dragSelection stretches the mark to wherever the mouse is, without letting
// it leave the cell where it started.
func (m *Model) dragSelection(x, y int) {
	if m.selection == nil || !m.selection.Dragging {
		return
	}
	area, has := m.visibleAreas()[m.selection.Cell]
	if !has {
		return
	}
	m.selection.CurrentX = min(max(x-area[0], 0), area[2]-1)
	m.selection.CurrentY = min(max(y-area[1], 0), area[3]-1)
}

// copySelection ends the drag and sends the stretch to the terminal's
// clipboard. The mark stays lit: it's the receipt for what got copied.
func (m *Model) copySelection() tea.Cmd {
	if m.selection == nil {
		return nil
	}
	m.selection.Dragging = false
	if m.selection.Empty() {
		// Going down and up in the same spot is a click, not a mark:
		// selecting the cell can't overwrite what the user had copied
		// before.
		m.selection = nil
		return nil
	}
	text := m.selectedText()
	if strings.TrimSpace(text) == "" {
		m.selection = nil
		return nil
	}
	return tea.SetClipboard(text)
}

// selectedText pulls out of the snapshot whatever is under the mark.
func (m *Model) selectedText() string {
	if m.selection == nil {
		return ""
	}
	inner, has := m.visibleInners()[m.selection.Cell]
	if !has {
		return ""
	}
	item := m.cellByID(m.selection.Cell)
	if item == nil {
		return ""
	}
	return m.selection.Text(item.Lines, inner.Cols)
}

// visibleAreas is where each visible cell sits on screen — origin and size
// of its inner content. It's what the mouse needs to know what it touched.
func (m *Model) visibleAreas() map[string][4]int {
	areas := map[string][4]int{}
	for id, inner := range m.visibleInners() {
		if x, y, has := m.cellOrigin(id); has {
			areas[id] = [4]int{x, y, inner.Cols, inner.Rows}
		}
	}
	return areas
}

// cellUnderPoint says which cell is under the screen point, and where the
// point lands inside its content.
func (m *Model) cellUnderPoint(x, y int) (id string, cx, cy int, found bool) {
	for item, area := range m.visibleAreas() {
		if x >= area[0] && x < area[0]+area[2] && y >= area[1] && y < area[1]+area[3] {
			return item, x - area[0], y - area[1], true
		}
	}
	return "", 0, 0, false
}

func (m *Model) cellByID(id string) *protocol.Cell {
	for i, project := range m.state.Projects {
		for j, item := range project.Cells {
			if item.ID == id {
				return &m.state.Projects[i].Cells[j]
			}
		}
	}
	return nil
}

func (m *Model) focusCell(id string) {
	for i, project := range m.state.Projects {
		for j, item := range project.Cells {
			if item.ID == id {
				m.focus.Project, m.focus.Cell = i, j
				return
			}
		}
	}
}

func (m *Model) View() tea.View {
	view := tea.NewView("")
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	// Tesseract owns the terminal's background while it's open, and gives it
	// back on exit. Without this the grid would inherit the caller's
	// background, and the theme's black would only show up under what the
	// app itself paints — everything else, including agent output, would
	// keep the outside background.
	view.BackgroundColor = theme.ScreenBackground()
	view.ForegroundColor = theme.ScreenForeground()
	view.WindowTitle = m.windowTitle()
	if m.width == 0 || m.height == 0 {
		return view
	}

	err := m.err
	if err == "" {
		err = m.state.Notice
	}

	mode := m.Mode()
	state := m.state
	if m.selection != nil {
		if inner, has := m.visibleInners()[m.selection.Cell]; has {
			state = withSelection(state, *m.selection, inner.Cols)
		}
	}

	var background string
	if m.showingGrid() {
		background = Draw(state, m.focus, mode, m.width, m.height, err)
	} else {
		background = DrawList(state, m.focus, mode, m.width, m.height, err)
	}

	switch {
	case m.panel != nil:
		background = overlay(background, m.panel.Draw(m.width), m.width, m.height)
	case m.form != nil:
		background = overlay(background, m.form.Draw(m.width), m.width, m.height)
	case m.question != nil:
		background = overlay(background, m.question.Draw(m.width), m.width, m.height)
	case m.confirmation != nil:
		background = overlay(background, m.confirmation.Draw(m.width), m.width, m.height)
	case m.help:
		background = overlay(background, helpBox(m.width, m.height), m.width, m.height)
	case m.results != nil:
		background = overlay(background, resultsBox(*m.results, m.resultChosen, m.width), m.width, m.height)
	}
	view.SetContent(background)

	// The cursor only shows up when the keyboard belongs to the cell and the
	// reading is live — a blinking cursor over scrolled-back history would
	// be a lie.
	if item := m.focusedCell(); m.typing && item != nil && item.Live && item.CursorX >= 0 {
		if x, y, has := m.cellOrigin(item.ID); has {
			view.Cursor = tea.NewCursor(x+item.CursorX, y+item.CursorY)
		}
	}
	return view
}

// windowTitle puts the brand on the terminal tab and says where the keyboard
// is. Whoever spends the day with the window minimized still sees Tesseract.
func (m *Model) windowTitle() string {
	project, item := m.currentFocus()
	if project == nil || item == nil {
		return theme.Glyph + " ts"
	}
	return theme.Glyph + " ts — " + project.Name + "/" + item.Name
}

func (m *Model) showingGrid() bool { return m.view != viewList }

// reportSizes tells the engine the area reserved for each visible cell, so
// the processes inside them see a terminal of the right size.
func (m *Model) reportSizes() {
	if m.width == 0 || m.height == 0 {
		return
	}
	for id, inner := range m.visibleInners() {
		if previous, alreadyReported := m.sizes[id]; alreadyReported && previous == inner {
			continue
		}
		m.sizes[id] = inner
		m.send(protocol.TypeSize, protocol.Size{Cell: id, Cols: inner.Cols, Rows: inner.Rows})
	}
}

func (m *Model) visibleInners() map[string]Dims {
	if m.showingGrid() {
		return Arrange(m.state, m.focus, m.width, m.height).Inners()
	}
	item := m.focusedCell()
	if item == nil {
		return nil
	}
	return map[string]Dims{item.ID: PreviewInner(m.width, m.height)}
}

// cellOrigin is where a cell's content begins on screen, so the cursor lands
// in the right spot.
func (m *Model) cellOrigin(id string) (int, int, bool) {
	if !m.showingGrid() {
		return indexWidth + 2, 4, true
	}
	if m.focus.Full {
		return 1, 2, true
	}
	return OriginInGrid(m.state, m.focus, m.width, m.height, id)
}

func (m *Model) send(kind string, data any) {
	if m.client == nil {
		return // a screen with no engine on the other side: only the drawing gets exercised
	}
	if err := m.client.Send(kind, data); err != nil {
		m.err = "the engine didn't respond: " + err.Error()
	}
}

func (m *Model) currentFocus() (*protocol.Project, *protocol.Cell) {
	if m.focus.Project >= len(m.state.Projects) {
		return nil, nil
	}
	project := &m.state.Projects[m.focus.Project]
	if m.focus.Cell >= len(project.Cells) {
		return project, nil
	}
	return project, &project.Cells[m.focus.Cell]
}

func (m *Model) focusedCell() *protocol.Cell {
	_, item := m.currentFocus()
	return item
}

// killConfirmation builds the kill warning, saying whether the whole project
// leaves the screen along with it.
func killConfirmation(project protocol.Project, item protocol.Cell) *Confirmation {
	lines := []string{"kill " + item.Type + " · " + item.Name + "?"}
	if len(project.Cells) == 1 {
		lines = append(lines, "",
			scrollColor.Render("it's the last cell of "+strings.ToUpper(project.Name)+","),
			scrollColor.Render("so the project leaves the screen."),
			faintColor.Render("the directory on disk is left untouched."))
	}
	return &Confirmation{Title: "KILL", Lines: lines, Action: keyboard.Kill, Target: item.ID}
}

// openDocker opens the focused project's panel. A project without compose
// has no panel, and saying so beats opening an empty box.
func (m *Model) openDocker() {
	project, _ := m.currentFocus()
	if project == nil {
		return
	}
	if !project.HasCompose {
		m.err = "the project " + project.Name + " has no compose file at its root"
		return
	}
	m.panel = NewDockerPanel(project.ID, project.Name)
	m.send(protocol.TypeDocker, protocol.Docker{Project: project.ID, Action: "list"})
}
