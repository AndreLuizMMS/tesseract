package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"

	"github.com/andreluiz/tesseract/internal/theme"
)

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

// command compiles `ts` once and returns a ready command, pointing at a fake
// state — nothing in the test touches the real Tesseract.
func command(t *testing.T, home string, args ...string) *exec.Cmd {
	t.Helper()
	buildOnce.Do(func() {
		dest, err := os.MkdirTemp("/tmp", "ts-bin")
		if err != nil {
			buildErr = err
			return
		}
		binPath = filepath.Join(dest, "ts")
		out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
		if err != nil {
			buildErr = err
			t.Logf("build: %s", out)
		}
	})
	if buildErr != nil {
		t.Fatalf("build: %v", buildErr)
	}

	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(),
		"XDG_STATE_HOME="+filepath.Join(home, "state"),
		"XDG_RUNTIME_DIR="+filepath.Join(home, "run"),
		"XDG_CONFIG_HOME="+filepath.Join(home, "config"),
		// No systemd in the test: the engine starts as a bare process.
		"PATH="+filepath.Join(home, "shortcuts")+string(filepath.ListSeparator)+os.Getenv("PATH"),
	)
	return cmd
}

// prepareShortcuts puts a systemctl that refuses everything ahead on the
// path, so the test never touches the machine's real service — without
// taking out of the path the tools the cells need.
func prepareShortcuts(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, "shortcuts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("prepare shortcuts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("prepare fake systemctl: %v", err)
	}
}

// testHome is a short home: the unix socket doesn't take a long path. The
// configuration points the agents to a fake program, so the test doesn't
// depend on claude or cursor being installed.
func testHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "ts")
	if err != nil {
		t.Fatalf("prepare home: %v", err)
	}
	prepareShortcuts(t, home)
	fakeAgent := filepath.Join(home, "fake-agent")
	if err := os.WriteFile(fakeAgent, []byte("#!/bin/sh\nwhile true; do sleep 1; done\n"), 0o755); err != nil {
		t.Fatalf("prepare agent: %v", err)
	}
	config := filepath.Join(home, "config", "tesseract", "config.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatalf("prepare configuration: %v", err)
	}
	body := `{"sound":false,"notify":false,"agents":{"claude":{"program":"` + fakeAgent + `"},"cursor":{"program":"` + fakeAgent + `"}}}`
	if err := os.WriteFile(config, []byte(body), 0o644); err != nil {
		t.Fatalf("prepare configuration: %v", err)
	}
	t.Cleanup(func() {
		stop := command(t, home, "stop")
		_ = stop.Run()
		os.RemoveAll(home)
	})
	return home
}

func run(t *testing.T, home string, args ...string) (string, int) {
	t.Helper()
	cmd := command(t, home, args...)
	out, err := cmd.CombinedOutput()
	code := 0
	var exitErr *exec.ExitError
	if err != nil {
		if ok := asExitError(err, &exitErr); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("run %v: %v", args, err)
		}
	}
	return string(out), code
}

func asExitError(err error, dest **exec.ExitError) bool {
	exitErr, ok := err.(*exec.ExitError)
	if ok {
		*dest = exitErr
	}
	return ok
}

// TestStatusWithNoEngine — with no engine up, status says so and exits
// clean.
func TestStatusWithNoEngine(t *testing.T) {
	home := testHome(t)
	out, code := run(t, home, "status")
	if code != 0 {
		t.Fatalf("status exited with %d: %s", code, out)
	}
	if !strings.Contains(out, "not running") {
		t.Fatalf("status should say the engine isn't running: %q", out)
	}
}

// TestNewStartsEngineAndCreatesProject — `ts new` works without opening the
// screen, and starts the engine if it isn't up.
func TestNewStartsEngineAndCreatesProject(t *testing.T) {
	home := testHome(t)
	project := filepath.Join(home, "my-project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	out, code := run(t, home, "new", project)
	if code != 0 {
		t.Fatalf("new exited with %d: %s", code, out)
	}
	if !strings.Contains(out, "my-project") {
		t.Fatalf("new should confirm the project: %q", out)
	}

	out, code = run(t, home, "status")
	if code != 0 {
		t.Fatalf("status exited with %d: %s", code, out)
	}
	if !strings.Contains(out, "1 project") || !strings.Contains(out, "my-project") {
		t.Fatalf("status should show the new project: %q", out)
	}
}

// TestNewOnMissingPathFails — a clear error and an error exit code.
func TestNewOnMissingPathFails(t *testing.T) {
	home := testHome(t)
	out, code := run(t, home, "new", filepath.Join(home, "does-not-exist"))
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d: %s", code, out)
	}
	if !strings.Contains(out, "does not exist") {
		t.Fatalf("unclear message: %q", out)
	}
}

// TestStopShutsDownEngine.
func TestStopShutsDownEngine(t *testing.T) {
	home := testHome(t)
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if _, code := run(t, home, "new", project); code != 0 {
		t.Fatal("couldn't create the project")
	}

	out, code := run(t, home, "stop")
	if code != 0 {
		t.Fatalf("stop exited with %d: %s", code, out)
	}
	waitUntil(t, 5*time.Second, func() bool {
		text, _ := run(t, home, "status")
		return strings.Contains(text, "not running")
	})
}

// TestResetClearsStateAndKeepsConfig.
func TestResetClearsStateAndKeepsConfig(t *testing.T) {
	home := testHome(t)
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	config := filepath.Join(home, "config", "tesseract", "config.json")

	if _, code := run(t, home, "new", project); code != 0 {
		t.Fatal("couldn't create the project")
	}
	stateFile := filepath.Join(home, "state", "tesseract", "state.json")
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("the state should have been saved: %v", err)
	}

	out, code := run(t, home, "reset")
	if code != 0 {
		t.Fatalf("reset exited with %d: %s", code, out)
	}
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Fatalf("the state should have disappeared: %v", err)
	}
	if _, err := os.Stat(config); err != nil {
		t.Fatalf("the configuration had to be preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "state", "tesseract", "history")); !os.IsNotExist(err) {
		t.Fatal("the history should have disappeared along with the state")
	}
}

// TestUnknownCommandExitsWithTwo.
func TestUnknownCommandExitsWithTwo(t *testing.T) {
	home := testHome(t)
	out, code := run(t, home, "fly")
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d: %s", code, out)
	}
	if !strings.Contains(out, "ts new") {
		t.Fatalf("usage should have been shown: %q", out)
	}
}

// TestHelpListsTheCommands.
func TestHelpListsTheCommands(t *testing.T) {
	home := testHome(t)
	out, code := run(t, home, "--help")
	if code != 0 {
		t.Fatalf("help exited with %d", code)
	}
	for _, cmd := range []string{"ts new", "ts status", "ts stop", "ts reset"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("help doesn't mention %q", cmd)
		}
	}
}

// TestScreenOpensDrawsAndCloses is the proof that the screen works end to
// end: `ts` starts the engine, draws the mosaic with a cell, accepts a key
// and closes, leaving the engine alive.
func TestScreenOpensDrawsAndCloses(t *testing.T) {
	home := testHome(t)
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	cmd := command(t, home)
	cmd.Dir = project
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	term := vt.NewSafeEmulator(100, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 100, Rows: 30})
	if err != nil {
		t.Fatalf("open the screen: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = pipe(term, terminal) }()
	go func() { _, _ = pipe(terminal, term) }()
	defer func() { lastScreen = term.Render() }()

	// The screen opens with a bash cell in full screen.
	waitUntil(t, 10*time.Second, func() bool {
		return strings.Contains(term.Render(), theme.Name) && strings.Contains(term.Render(), "claude")
	})
	render := term.Render()
	if !strings.Contains(render, "BROWSE") {
		t.Fatalf("the screen should open in BROWSE:\n%s", render)
	}
	if !strings.Contains(render, "n create") {
		t.Fatalf("the footer should be lit:\n%s", render)
	}

	// Help opens and closes.
	_, _ = terminal.Write([]byte("?"))
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "HELP") })
	_, _ = terminal.Write([]byte{0x1b})
	waitUntil(t, 3*time.Second, func() bool { return !strings.Contains(term.Render(), "HELP") })

	// The create form opens with the focused project already filled in.
	_, _ = terminal.Write([]byte("n"))
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "NEW") })
	if !strings.Contains(term.Render(), "PROJECT") {
		t.Fatalf("the form should ask for the project:\n%s", term.Render())
	}
	_, _ = terminal.Write([]byte{0x1b})
	waitUntil(t, 3*time.Second, func() bool { return !strings.Contains(term.Render(), "NEW") })

	// Enters TYPE, types in the shell and comes back.
	_, _ = terminal.Write([]byte("\r"))
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "TYPE") })
	_, _ = terminal.Write([]byte("echo tesseract-alive\r"))
	waitUntil(t, 5*time.Second, func() bool {
		lastScreen = term.Render()
		return strings.Contains(term.Render(), "tesseract-alive")
	})
	_, _ = terminal.Write([]byte{0x0c}) // ctrl-l
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "BROWSE") })

	// Closes the screen — and the engine keeps running.
	_, _ = terminal.Write([]byte("q"))
	if err := waitExit(cmd, 5*time.Second); err != nil {
		t.Fatalf("the screen didn't close: %v", err)
	}
	out, code := run(t, home, "status")
	if code != 0 || !strings.Contains(out, "1 cell") {
		t.Fatalf("the engine should still be alive with the cell: %q", out)
	}
}

// goToShellTab switches tabs until it reaches the shell, the only tab a
// test can drive without depending on a real agent.
func goToShellTab(terminal *os.File, term *vt.SafeEmulator) {
	for range 4 {
		if strings.Contains(term.Render(), "$ ") || strings.Contains(term.Render(), "➜") {
			return
		}
		_, _ = terminal.Write([]byte("\t"))
		time.Sleep(900 * time.Millisecond)
	}
}

func waitExit(cmd *exec.Cmd, deadline time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(deadline):
		_ = cmd.Process.Kill()
		return os.ErrDeadlineExceeded
	}
}

func waitUntil(t *testing.T, deadline time.Duration, condition func() bool) {
	t.Helper()
	limit := time.Now().Add(deadline)
	for time.Now().Before(limit) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !condition() {
		t.Fatalf("the condition didn't happen within %s\n%s", deadline, lastScreen)
	}
}

// lastScreen keeps the last drawn screen, so a failure can say what was
// written on it.
var lastScreen string

// pipe wires up both ends of the pseudo terminal.
func pipe(dest interface{ Write([]byte) (int, error) }, src interface {
	Read([]byte) (int, error)
},
) (int64, error) {
	buf := make([]byte, 32<<10)
	var total int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			written, writeErr := dest.Write(buf[:n])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
		}
		if err != nil {
			return total, err
		}
	}
}

// TestGridWithTwoProjects is the mosaic's visual proof: both projects show
// up at the same time, with every cell in view, and the arrow moves between
// them. Killing the last cell takes the project off the screen.
func TestGridWithTwoProjects(t *testing.T) {
	home := testHome(t)
	first := filepath.Join(home, "cortz-web")
	second := filepath.Join(home, "doxar-api")
	for _, dir := range []string{first, second} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("prepare: %v", err)
		}
		if out, code := run(t, home, "new", dir); code != 0 {
			t.Fatalf("create project: %s", out)
		}
	}

	cmd := command(t, home)
	cmd.Dir = first
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	term := vt.NewSafeEmulator(120, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("open the screen: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = pipe(term, terminal) }()
	go func() { _, _ = pipe(terminal, term) }()

	waitUntil(t, 10*time.Second, func() bool { return strings.Contains(term.Render(), "CORTZ-WEB") })
	// Both projects stay on screen at once, neither turns into a strip.
	waitUntil(t, 5*time.Second, func() bool {
		render := term.Render()
		return strings.Contains(render, "CORTZ-WEB") && strings.Contains(render, "DOXAR-API")
	})

	// The arrow moves to the project on the side.
	_, _ = terminal.Write([]byte("\x1b[C")) // right arrow
	time.Sleep(500 * time.Millisecond)

	// D asks for confirmation, and warns that the project leaves the screen.
	_, _ = terminal.Write([]byte("D"))
	waitUntil(t, 5*time.Second, func() bool { return strings.Contains(term.Render(), "KILL") })
	if !strings.Contains(term.Render(), "leaves the screen") {
		t.Fatalf("the confirmation should warn the project leaves the screen:\n%s", term.Render())
	}

	_, _ = terminal.Write([]byte("\r")) // confirm
	waitUntil(t, 10*time.Second, func() bool { return !strings.Contains(term.Render(), "DOXAR-API") })

	// The directory stays untouched on disk.
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("the disk was touched: %v", err)
	}

	_, _ = terminal.Write([]byte("q"))
	_ = waitExit(cmd, 5*time.Second)
}

// TestListShowsSameAsGrid — the switch-screen key leads to the index, with
// the same projects and cells.
func TestListShowsSameAsGrid(t *testing.T) {
	home := testHome(t)
	project := filepath.Join(home, "cortz-web")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if out, code := run(t, home, "new", project); code != 0 {
		t.Fatalf("create project: %s", out)
	}

	cmd := command(t, home)
	cmd.Dir = project
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	term := vt.NewSafeEmulator(120, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("open the screen: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = pipe(term, terminal) }()
	go func() { _, _ = pipe(terminal, term) }()

	waitUntil(t, 10*time.Second, func() bool { return strings.Contains(term.Render(), "CORTZ-WEB") })

	_, _ = terminal.Write([]byte("v"))
	waitUntil(t, 5*time.Second, func() bool {
		render := term.Render()
		return strings.Contains(render, "CORTZ-WEB") && strings.Contains(render, "bash")
	})

	_, _ = terminal.Write([]byte("q"))
	_ = waitExit(cmd, 5*time.Second)

	// The chosen screen is remembered across runs.
	second := command(t, home)
	second.Dir = project
	second.Env = append(second.Env, "TERM=xterm-256color")
	term2 := vt.NewSafeEmulator(120, 30)
	terminal2, err := pty.StartWithSize(second, &pty.Winsize{Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("reopen the screen: %v", err)
	}
	defer terminal2.Close()
	go func() { _, _ = pipe(term2, terminal2) }()
	go func() { _, _ = pipe(terminal2, term2) }()

	waitUntil(t, 10*time.Second, func() bool { return strings.Contains(term2.Render(), "CORTZ-WEB") })
	if !strings.Contains(term2.Render(), "┬") && !strings.Contains(term2.Render(), "│") {
		t.Fatalf("the screen should have reopened:\n%s", term2.Render())
	}
	_, _ = terminal2.Write([]byte("q"))
	_ = waitExit(second, 5*time.Second)
}

// TestSearchInHistoryFromScreen — the search key asks for the term, the
// engine searches the focused cell's history and the screen shows what it
// found.
func TestSearchInHistoryFromScreen(t *testing.T) {
	home := testHome(t)
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	cmd := command(t, home)
	cmd.Dir = project
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	term := vt.NewSafeEmulator(110, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 110, Rows: 30})
	if err != nil {
		t.Fatalf("open the screen: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = pipe(term, terminal) }()
	go func() { _, _ = pipe(terminal, term) }()

	waitUntil(t, 10*time.Second, func() bool { return strings.Contains(term.Render(), "bash") })

	// Goes to the shell tab, the only place a test can type into.
	goToShellTab(terminal, term)

	// Types something in the shell so there's history.
	_, _ = terminal.Write([]byte("\r"))
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "TYPE") })
	_, _ = terminal.Write([]byte("echo needle-in-the-haystack\r"))
	waitUntil(t, 5*time.Second, func() bool { return strings.Contains(term.Render(), "needle-in-the-haystack") })
	_, _ = terminal.Write([]byte{0x0c}) // ctrl-l
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "BROWSE") })

	// Search.
	_, _ = terminal.Write([]byte("/"))
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "SEARCH") })
	_, _ = terminal.Write([]byte("needle-in-the-haystack\r"))
	waitUntil(t, 5*time.Second, func() bool { return strings.Contains(term.Render(), "SEARCH · needle-in-the-haystack") })
	if !strings.Contains(term.Render(), "needle-in-the-haystack") {
		t.Fatalf("the search should show the matched line:\n%s", term.Render())
	}

	_, _ = terminal.Write([]byte{0x1b}) // esc closes
	waitUntil(t, 3*time.Second, func() bool { return !strings.Contains(term.Render(), "SEARCH ·") })

	_, _ = terminal.Write([]byte("q"))
	_ = waitExit(cmd, 5*time.Second)
}

// TestTabSwitchesTabInSession — creating doesn't ask what the session will
// be, and the tab key moves between claude, cursor and shell inside the
// same cell.
func TestTabSwitchesTabInSession(t *testing.T) {
	home := testHome(t)
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	cmd := command(t, home)
	cmd.Dir = project
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	term := vt.NewSafeEmulator(120, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("open the screen: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = pipe(term, terminal) }()
	go func() { _, _ = pipe(terminal, term) }()

	// The cell is born with the three tabs on the border, with nobody having
	// chosen anything.
	waitUntil(t, 10*time.Second, func() bool {
		render := term.Render()
		return strings.Contains(render, "claude") && strings.Contains(render, "cursor") && strings.Contains(render, "bash")
	})

	// The footer tells us the key exists.
	if !strings.Contains(term.Render(), "tab tab") {
		t.Errorf("the footer should show the tab key:\n%s", term.Render())
	}

	// The tab key leads to the shell, where it's possible to type.
	goToShellTab(terminal, term)

	_, _ = terminal.Write([]byte("\r"))
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "TYPE") })
	_, _ = terminal.Write([]byte("echo im-on-the-shell-tab\r"))
	waitUntil(t, 6*time.Second, func() bool { return strings.Contains(term.Render(), "im-on-the-shell-tab") })

	_, _ = terminal.Write([]byte{0x0c}) // ctrl-l
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "BROWSE") })
	_, _ = terminal.Write([]byte("q"))
	_ = waitExit(cmd, 5*time.Second)
}

// TestFormStartsAtUserHome — the path field already points to home.
func TestFormStartsAtUserHome(t *testing.T) {
	home := testHome(t)
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	cmd := command(t, home)
	cmd.Dir = project
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	term := vt.NewSafeEmulator(120, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("open the screen: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = pipe(term, terminal) }()
	go func() { _, _ = pipe(terminal, term) }()

	waitUntil(t, 10*time.Second, func() bool { return strings.Contains(term.Render(), theme.Name) })
	_, _ = terminal.Write([]byte("n"))
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "NEW") })

	render := term.Render()
	if !strings.Contains(render, os.Getenv("HOME")) {
		t.Errorf("the form should start at the user's home:\n%s", render)
	}
	if strings.Contains(render, "TYPE") {
		t.Errorf("the form no longer asks for the type:\n%s", render)
	}
	if !strings.Contains(render, "MD") {
		t.Errorf("the form should offer the markdown field:\n%s", render)
	}

	_, _ = terminal.Write([]byte{0x1b})
	_, _ = terminal.Write([]byte("q"))
	_ = waitExit(cmd, 5*time.Second)
}

// TestMarkdownTabListsSearchesAndOpens — the markdown tab shows the
// project's files, filters by name as you type and opens the chosen one on
// enter.
func TestMarkdownTabListsSearchesAndOpens(t *testing.T) {
	home := testHome(t)
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(filepath.Join(project, "docs"), 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	files := map[string]string{
		"README.md":         "# Read this\n\nthe start of everything\n",
		"docs/spec-m7.md":   "# Module 7\n\nclinical charts with PHI\n",
		"docs/decisions.md": "# Decisions\n\nnothing here\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(project, name), []byte(content), 0o644); err != nil {
			t.Fatalf("prepare: %v", err)
		}
	}

	cmd := command(t, home)
	cmd.Dir = project
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	term := vt.NewSafeEmulator(120, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("open the screen: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = pipe(term, terminal) }()
	go func() { _, _ = pipe(terminal, term) }()
	defer func() { lastScreen = term.Render() }()

	waitUntil(t, 10*time.Second, func() bool { return strings.Contains(term.Render(), "md") })

	// Moves to the markdown tab.
	for range 4 {
		if strings.Contains(term.Render(), "search:") {
			break
		}
		_, _ = terminal.Write([]byte("\t"))
		time.Sleep(700 * time.Millisecond)
	}
	waitUntil(t, 5*time.Second, func() bool {
		lastScreen = term.Render()
		render := term.Render()
		return strings.Contains(render, "search:") && strings.Contains(render, "README.md") &&
			strings.Contains(render, "spec-m7.md")
	})

	// The search filters by name as you type.
	_, _ = terminal.Write([]byte("\r")) // enters TYPE
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "TYPE") })
	_, _ = terminal.Write([]byte("m7"))
	waitUntil(t, 3*time.Second, func() bool {
		lastScreen = term.Render()
		render := term.Render()
		return strings.Contains(render, "spec-m7.md") && !strings.Contains(render, "decisions.md")
	})

	// Enter opens the chosen file.
	_, _ = terminal.Write([]byte("\r"))
	waitUntil(t, 5*time.Second, func() bool {
		lastScreen = term.Render()
		return strings.Contains(term.Render(), "clinical charts")
	})

	// Esc goes back to the list.
	_, _ = terminal.Write([]byte{0x1b})
	waitUntil(t, 3*time.Second, func() bool {
		lastScreen = term.Render()
		return strings.Contains(term.Render(), "search:")
	})

	_, _ = terminal.Write([]byte{0x0c}) // ctrl-l
	waitUntil(t, 3*time.Second, func() bool { return strings.Contains(term.Render(), "BROWSE") })
	_, _ = terminal.Write([]byte("q"))
	_ = waitExit(cmd, 5*time.Second)
}
