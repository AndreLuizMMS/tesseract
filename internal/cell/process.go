package cell

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// replayedTail is how much of the history gets replayed into the internal
// screen when the cell is reborn. Enough for the last screen to reappear
// whole, little enough to cost nothing.
const replayedTail = 256 << 10

// Process is the base for cells that run something: it holds the pseudo
// terminal, the internal screen and the history recording. The concrete
// types only say which command comes up.
type Process struct {
	mu       sync.Mutex
	brush    sync.Mutex // guards the internal screen between whoever writes and whoever draws
	emulator *vt.SafeEmulator
	terminal *os.File
	cmd      *exec.Cmd
	config   Config
	state    State
	scroll   int
	dying    bool // true when we're the ones who killed it
	finished bool

	// frame is the last portrait drawn, held under the brush. Drawing a
	// still internal screen costs the same as drawing one that changed, and
	// the engine draws the whole grid 25 times a second — a still cell hands
	// back the same portrait instead of redrawing it.
	frame       Frame
	frameValid  bool
	frameScroll int
}

// start brings the command up inside a pseudo terminal of the requested
// size, and replays the previous history into the internal screen.
func (p *Process) start(cfg Config, program string, args []string, environment []string) error {
	if cfg.Columns <= 0 {
		cfg.Columns = 80
	}
	if cfg.Lines <= 0 {
		cfg.Lines = 24
	}
	if info, err := os.Stat(cfg.Directory); err != nil || !info.IsDir() {
		return fmt.Errorf("project directory doesn't exist: %s", cfg.Directory)
	}

	p.config = cfg
	p.emulator = vt.NewSafeEmulator(cfg.Columns, cfg.Lines)
	p.state = Working

	// The internal screen answers on its own the questions the program asks
	// the terminal. Whoever listens for those answers needs to be up before
	// any byte comes in — including before replaying the history, which
	// brings old questions along and would hang here if nobody collected
	// them.
	go p.respondToProcess()

	if cfg.History != nil {
		if tail, err := cfg.History.Tail(replayedTail); err == nil && len(tail) > 0 {
			// Only the internal screen receives it: replaying isn't
			// producing, and none of it goes back to the file or reaches
			// the process. The answers the replay triggers respond to old
			// questions and die here, because there's no process yet to
			// receive them.
			p.brush.Lock()
			_, _ = p.emulator.Write(tail)
			p.frameValid = false
			p.brush.Unlock()
		}
	}

	return p.reconnect(program, args, environment)
}

// reconnect brings the command up on the same internal screen, which is what
// lets a log cell go back to following the service when it comes back up,
// without losing what was already written.
func (p *Process) reconnect(program string, args, environment []string) error {
	p.mu.Lock()
	columns, lines := p.config.Columns, p.config.Lines
	p.mu.Unlock()

	cmd := exec.Command(program, args...)
	cmd.Dir = p.config.Directory
	cmd.Env = environment

	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(columns),
		Rows: uint16(lines),
	})
	if err != nil {
		return fmt.Errorf("bringing up %s in %s: %w", program, p.config.Directory, err)
	}

	p.mu.Lock()
	p.cmd, p.terminal = cmd, terminal
	p.finished, p.dying = false, false
	p.state = Working
	p.mu.Unlock()

	go p.readFromProcess(terminal, cmd)
	return nil
}

// readFromProcess carries what the process wrote to the internal screen and
// to the history, and notices when it dies.
func (p *Process) readFromProcess(terminal *os.File, cmd *exec.Cmd) {
	buf := make([]byte, 32<<10)
	for {
		n, err := terminal.Read(buf)
		if n > 0 {
			p.brush.Lock()
			_, _ = p.emulator.Write(buf[:n])
			p.frameValid = false
			p.brush.Unlock()
			if p.config.History != nil {
				_, _ = p.config.History.Write(buf[:n])
			}
			p.notify()
		}
		if err != nil {
			break
		}
	}

	_ = cmd.Wait()

	p.mu.Lock()
	if p.dying {
		p.state = Stopped
	} else {
		p.state = Crashed
	}
	p.finished = true
	p.mu.Unlock()
	p.notify()
}

// respondToProcess hands the program the answers the internal screen
// produces — cursor position queries, terminal capability queries and the
// like. There's only one, alive for the cell's whole life: the process can
// die and come back, the internal screen stays the same.
func (p *Process) respondToProcess() {
	buf := make([]byte, 4<<10)
	for {
		n, err := p.emulator.Read(buf)
		if n > 0 {
			p.mu.Lock()
			terminal := p.terminal
			p.mu.Unlock()
			if terminal != nil {
				_, _ = terminal.Write(buf[:n])
			}
		}
		if err != nil {
			return
		}
	}
}

func (p *Process) notify() {
	if p.config.Notify != nil {
		p.config.Notify()
	}
}

// Draw assembles the cell's portrait: the internal screen when the reading
// is live, or the history window when the user scrolled back. Either way the
// text comes out styled — scrolling can't erase anyone's color.
func (p *Process) Draw() Frame {
	p.mu.Lock()
	scroll := p.scroll
	p.mu.Unlock()

	if p.emulator == nil {
		return Frame{Live: true}
	}

	p.brush.Lock()
	defer p.brush.Unlock()

	// Nothing entered the internal screen and the reading is in the same
	// place: the portrait is the same as before.
	if p.frameValid && p.frameScroll == scroll {
		return p.frame
	}

	// The requested scroll is the key to the stored portrait; the clamped
	// one is what draws. They only diverge when there isn't enough history
	// yet, and what grows the history is writing to the internal screen —
	// which already invalidates it.
	requested := scroll
	height := p.emulator.Height()
	above := p.emulator.ScrollbackLen()
	scroll = min(scroll, above)
	onScreen := strings.Split(p.emulator.Render(), "\n")

	if scroll == 0 {
		position := p.emulator.CursorPosition()
		return p.storeFrame(requested, Frame{Lines: onScreen, CursorX: position.X, CursorY: position.Y, Live: true})
	}

	// The scrolled window mixes the lines that already scrolled off above
	// with the ones still on screen below.
	history := p.emulator.Scrollback()
	lines := make([]string, 0, height)
	top := above - scroll
	for i := range height {
		line := top + i
		switch {
		case line < above:
			lines = append(lines, history.Line(line).Render())
		case line-above < len(onScreen):
			lines = append(lines, onScreen[line-above])
		default:
			lines = append(lines, "")
		}
	}
	return p.storeFrame(requested, Frame{Lines: lines, Scroll: scroll, CursorX: -1, CursorY: -1})
}

// storeFrame records the just-drawn portrait. Called with the brush held,
// the same lock whoever writes to the internal screen uses — the stored
// portrait is never one of a screen that already changed.
func (p *Process) storeFrame(scroll int, frame Frame) Frame {
	p.frame, p.frameScroll, p.frameValid = frame, scroll, true
	return frame
}

// Key delivers the keystroke to the process. In TYPE mode every key passes
// through here, no exceptions. The internal screen translates the key into
// the bytes that program expects, respecting whatever modes it turned on.
func (p *Process) Key(tap Keystroke) error {
	p.mu.Lock()
	finished := p.finished
	p.scroll = 0 // typing brings the reading back to live
	p.mu.Unlock()

	if finished || p.emulator == nil {
		return fmt.Errorf("the cell isn't running")
	}
	if tap.Paste != "" {
		// Paste goes in as a paste, not as typing: when the program in
		// there asked for bracketed paste, the text arrives marked as
		// pasted and line breaks don't become enter — a multi-line prompt
		// goes in whole instead of being sent line by line.
		p.emulator.Paste(tap.Paste)
		return nil
	}
	// A key that prints something goes in as text: what it writes is what
	// matters, so an uppercase letter with shift doesn't get lost along the
	// way.
	if tap.Text != "" && tap.Mod&^int(vt.ModShift) == 0 {
		p.emulator.SendText(tap.Text)
		return nil
	}
	p.emulator.SendKey(vt.KeyPressEvent{
		Code: tap.Code,
		Text: tap.Text,
		Mod:  vt.KeyMod(tap.Mod),
	})
	return nil
}

func (p *Process) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

func (p *Process) States() []State {
	return []State{Working, Crashed, Stopped, Orphaned}
}

// Resize adjusts the internal screen and the pseudo terminal to the space
// the screen reserved for the cell.
func (p *Process) Resize(columns, lines int) error {
	if columns <= 0 || lines <= 0 {
		return nil
	}
	p.mu.Lock()
	// Resizing again to the same size clears the internal screen, and the
	// program in there only redraws when it has something to say — that's
	// why a freshly opened tab used to show up blank.
	if p.config.Columns == columns && p.config.Lines == lines {
		p.mu.Unlock()
		return nil
	}
	terminal := p.terminal
	p.config.Columns, p.config.Lines = columns, lines
	p.mu.Unlock()

	if p.emulator != nil {
		p.brush.Lock()
		p.emulator.Resize(columns, lines)
		p.frameValid = false
		p.brush.Unlock()
	}
	if terminal != nil {
		_ = pty.Setsize(terminal, &pty.Winsize{Cols: uint16(columns), Rows: uint16(lines)})
	}
	p.notify()
	return nil
}

// Scroll moves the reading through the history. Positive delta goes up.
func (p *Process) Scroll(delta int, live bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if live {
		p.scroll = 0
		return
	}
	ceiling := 0
	if p.emulator != nil {
		ceiling = p.emulator.ScrollbackLen()
	}
	p.scroll = min(max(p.scroll+delta, 0), ceiling)
}

// Kill ends the process. The project's disk is never touched.
func (p *Process) Kill() error {
	p.mu.Lock()
	if p.finished || p.cmd == nil || p.cmd.Process == nil {
		p.mu.Unlock()
		return nil
	}
	p.dying = true
	pid := p.cmd.Process.Pid
	terminal := p.terminal
	p.mu.Unlock()

	// The pseudo terminal creates its own process group; killing the group
	// takes along whatever the shell had opened.
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	if terminal != nil {
		_ = terminal.Close()
	}
	return nil
}
