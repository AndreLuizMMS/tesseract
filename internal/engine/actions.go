package engine

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/docker"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// Resume brings a stopped or crashed cell back up. Nothing restarts on its
// own: automatic restart hides the root cause and produces a silent crash
// loop.
func (e *Engine) Resume(cellID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, p := range e.projects {
		for _, c := range p.cells {
			if c.id != cellID {
				continue
			}
			if c.proc != nil {
				state := c.proc.State()
				if state != cell.Crashed && state != cell.Stopped && state != cell.Orphaned {
					return fmt.Errorf("cell %s is already running", c.name)
				}
				_ = c.proc.Kill()
			}
			if c.record != nil {
				_ = c.record.Close()
			}
			if err := e.wake(p, c); err != nil {
				return err
			}
			e.markDirty()
			return nil
		}
	}
	return fmt.Errorf("cell %s does not exist", cellID)
}

// SwitchTab moves to the neighboring tab inside the cell. It's what lets a
// session have claude, cursor and shell without choosing anything at
// creation.
func (e *Engine) SwitchTab(cellID string, step int) error {
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", cellID)
	}
	withTabs, ok := c.proc.(cell.WithTabs)
	if !ok {
		return fmt.Errorf("cell %s has no tabs", c.name)
	}
	if err := withTabs.SwitchTab(step); err != nil {
		return err
	}
	e.mu.Lock()
	c.tab = withTabs.ActiveTab()
	e.save()
	e.mu.Unlock()
	e.markDirty()
	return nil
}

// Prompt sends work to the cell without the user entering it.
func (e *Engine) Prompt(cellID, text string) error {
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", cellID)
	}
	withPrompt, accepts := c.proc.(cell.WithPrompt)
	if !accepts {
		return fmt.Errorf("cell %s does not accept prompts", c.name)
	}
	if err := withPrompt.Prompt(text); err != nil {
		return err
	}
	e.markDirty()
	return nil
}

// AutoRename asks the agent to pick and write the conversation's own name.
// The name doesn't exist yet: the watcher is what applies it to the cell,
// when the turn ends (see AdoptAgentName).
func (e *Engine) AutoRename(cellID string) error {
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", cellID)
	}
	withConversation, ok := c.proc.(cell.WithConversation)
	if !ok {
		return fmt.Errorf("cell %s has no named conversation", c.name)
	}
	if err := withConversation.RequestAutoName(); err != nil {
		return err
	}
	e.mu.Lock()
	c.pendingRename = true
	e.mu.Unlock()
	return nil
}

// AdoptAgentName pulls into the cell the name the agent gave the
// conversation and disarms the wait, whether it succeeds or not — retrying on
// its own doesn't help.
func (e *Engine) AdoptAgentName(cellID string) error {
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", cellID)
	}
	e.mu.Lock()
	c.pendingRename = false
	e.mu.Unlock()

	withConversation, ok := c.proc.(cell.WithConversation)
	if !ok {
		return fmt.Errorf("cell %s has no named conversation", c.name)
	}
	name, err := withConversation.ConversationName()
	if err != nil {
		return err
	}
	return e.Rename(cellID, name)
}

// ideCandidates is every IDE tesseract knows how to launch — not a scan of
// the filesystem for anything that looks like an editor, but the small,
// known set the picker checks for, on each of the two places it could live.
var ideCandidates = []struct{ ID, Label string }{
	{"code", "VS Code"},
	{"code-insiders", "VS Code Insiders"},
	{"codium", "VSCodium"},
	{"cursor", "Cursor"},
}

// DetectIDEs finds which of the known IDEs are reachable from here, inside
// WSL and on the Windows side alike. The two checks per candidate run
// concurrently — the Windows one crosses into cmd.exe and is slow enough on
// its own that eight of them in a row would make ctrl+e feel stuck.
func (e *Engine) DetectIDEs() []protocol.IDE {
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		list []protocol.IDE
	)
	add := func(id protocol.IDE) {
		mu.Lock()
		list = append(list, id)
		mu.Unlock()
	}
	for _, c := range ideCandidates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if wslInstalled(c.ID) {
				add(protocol.IDE{ID: c.ID, Label: c.Label, Location: "wsl"})
			}
			if windowsInstalled(c.ID) {
				add(protocol.IDE{ID: c.ID, Label: c.Label, Location: "windows"})
			}
		}()
	}
	wg.Wait()
	sort.Slice(list, func(i, j int) bool {
		if list[i].Label != list[j].Label {
			return list[i].Label < list[j].Label
		}
		return list[i].Location < list[j].Location
	})
	return list
}

// wslInstalled says whether the id names a real Linux install on PATH. The
// remote CLI an IDE drops under ~/.<name>-server only talks to a window
// someone else opened — it doesn't count as an install of its own.
func wslInstalled(id string) bool {
	binary, err := exec.LookPath(id)
	return err == nil && !strings.Contains(binary, "-server/")
}

// windowsInstalled asks Windows' own PATH, through cmd.exe, whether it knows
// the id. Only makes sense inside WSL — outside it there's no Windows on the
// other end to ask.
func windowsInstalled(id string) bool {
	if _, err := os.Stat(cmdExePath); err != nil {
		return false
	}
	return exec.Command(cmdExePath, "/c", "where", id).Run() == nil
}

// OpenInEditor opens the project directory in the picked IDE, at the place
// the picker said it lives.
func (e *Engine) OpenInEditor(projectID, id, location string) error {
	e.mu.Lock()
	path := ""
	for _, p := range e.projects {
		if p.id == projectID {
			path = p.path
		}
	}
	e.mu.Unlock()

	if path == "" {
		return fmt.Errorf("project %s does not exist", projectID)
	}
	if id == "" {
		return fmt.Errorf("no IDE picked")
	}

	// The `code`/`codium` launcher inside WSL asks, on stderr, whether to
	// really open the Linux install, and waits for an answer on stdin. The
	// engine is a service and has no stdin: the read hits EOF, the answer
	// comes out empty, and the launcher exits without opening anything. This
	// variable is the launcher's own way out of that prompt.
	environment := append(withDisplay(os.Environ()), "DONT_PROMPT_WSL_INSTALL=1")
	binaryName, arguments := id, []string{path}
	if location == "windows" {
		// A window already open for this IDE is reused through its remote
		// CLI: the socket it leaves behind on WSL keeps responding even
		// after the window has closed, so "socket alive" is what proves a
		// window is still there to receive the request.
		if cli, window := ideAlreadyOpen(id); cli != "" {
			binaryName = cli
			environment = append(environment, "VSCODE_IPC_HOOK_CLI="+window)
		} else if distro := wslDistro(); distro != "" {
			binaryName, arguments = cmdExePath, []string{"/c", "start", "", id, "--folder-uri", "vscode-remote://wsl+" + distro + path}
		} else {
			return fmt.Errorf("could not find the WSL distro name")
		}
	}
	command := exec.Command(binaryName, arguments...)
	command.Dir = path
	command.Env = environment
	output := &strings.Builder{}
	command.Stdout, command.Stderr = output, output
	if err := command.Start(); err != nil {
		return fmt.Errorf("could not open %s: %w", id, err)
	}

	// The IDE that opens a window stays alive; the one that didn't dies right
	// away. Half a second separates the two, and that's what gets the error to
	// show up on screen instead of the shortcut looking like it did nothing.
	died := make(chan error, 1)
	go func() { died <- command.Wait() }()
	select {
	case err := <-died:
		if err != nil {
			return fmt.Errorf("%s did not open: %s", binaryName, firstLine(output.String(), err))
		}
	case <-time.After(500 * time.Millisecond):
	}
	return nil
}

// cmdExePath is fixed because it's where Windows always installs its own
// command interpreter — there's no per-machine variation to discover.
const cmdExePath = "/mnt/c/Windows/System32/cmd.exe"

// wslDistro asks Windows for the name of the registered distro. With more
// than one installed it's ambiguous — takes the first in the list; ponytail
// environment: only one distro, so there's nothing to really choose from.
func wslDistro() string {
	output, err := exec.Command(cmdExePath, "/c", "wsl.exe", "-l", "-q").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(strings.Map(func(r rune) rune {
			if r == 0 {
				return -1
			}
			return r
		}, line))
		if line != "" {
			return line
		}
	}
	return ""
}

// ideAlreadyOpen looks for the IDE's remote CLI and the socket of a live
// window. It only returns both together: a CLI with no open window opens
// nothing, and a window with no CLI has no way to receive the request.
func ideAlreadyOpen(name string) (cli string, window string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	found, _ := filepath.Glob(filepath.Join(home, "."+name+"-server", "bin", "*", "bin", "remote-cli", name))
	if len(found) == 0 {
		return "", ""
	}
	window = liveWindow(os.Getenv("XDG_RUNTIME_DIR"))
	if window == "" {
		return "", ""
	}
	return newest(found), window
}

// liveWindow is the socket of the window that still responds. IDEs leave
// behind the socket of every window that has already closed, so only a
// connection test tells them apart.
func liveWindow(runtimeDir string) string {
	if runtimeDir == "" {
		return ""
	}
	sockets, _ := filepath.Glob(filepath.Join(runtimeDir, "vscode-ipc-*.sock"))
	for _, s := range sortedByNewest(sockets) {
		if conn, err := net.DialTimeout("unix", s, 200*time.Millisecond); err == nil {
			_ = conn.Close()
			return s
		}
	}
	return ""
}

// newest picks the path touched most recently — the most recent install.
func newest(paths []string) string {
	sorted := sortedByNewest(paths)
	if len(sorted) == 0 {
		return ""
	}
	return sorted[0]
}

func sortedByNewest(paths []string) []string {
	age := func(c string) int64 {
		info, err := os.Stat(c)
		if err != nil {
			return 0
		}
		return info.ModTime().Unix()
	}
	sorted := append([]string(nil), paths...)
	sort.Slice(sorted, func(i, j int) bool { return age(sorted[i]) > age(sorted[j]) })
	return sorted
}

// withDisplay fills in the environment with the graphical server when it's
// missing. The engine runs as a service, and a service inherits no DISPLAY at
// all — without this the IDE comes up with no display to draw on and dies
// silently.
func withDisplay(environment []string) []string {
	has := func(name string) bool {
		for _, line := range environment {
			if strings.HasPrefix(line, name+"=") && len(line) > len(name)+1 {
				return true
			}
		}
		return false
	}
	if !has("DISPLAY") {
		if _, err := os.Stat("/tmp/.X11-unix/X0"); err == nil {
			environment = append(environment, "DISPLAY=:0")
		}
	}
	if home := os.Getenv("XDG_RUNTIME_DIR"); home != "" && !has("WAYLAND_DISPLAY") {
		if _, err := os.Stat(filepath.Join(home, "wayland-0")); err == nil {
			environment = append(environment, "WAYLAND_DISPLAY=wayland-0")
		}
	}
	return environment
}

// firstLine summarizes what the editor complained about, so the alert fits
// in the footer.
func firstLine(output string, fallback error) string {
	for _, line := range strings.Split(output, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return fallback.Error()
}

// GoToLine moves the cell's reading position to a line in the history — it's
// where the search ends.
func (e *Engine) GoToLine(cellID string, line int) error {
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", cellID)
	}
	total, err := e.historyOf(c).Lines()
	if err != nil {
		return err
	}
	// Scrolling counts lines backward from the end, and the search counts from
	// the start.
	c.proc.Scroll(0, true)
	c.proc.Scroll(max(total-line, 0), false)
	e.markDirty()
	return nil
}

// Docker acts on the project's stack, or one of its services. Listing is what
// the panel asks for on open; log turns into a cell in the mosaic.
func (e *Engine) Docker(request protocol.Docker) (protocol.Services, error) {
	e.mu.Lock()
	var target *project
	for _, p := range e.projects {
		if p.id == request.Project {
			target = p
		}
	}
	e.mu.Unlock()

	if target == nil {
		return protocol.Services{}, fmt.Errorf("project %s does not exist", request.Project)
	}
	if target.compose == "" {
		return protocol.Services{}, fmt.Errorf("project %s has no compose file at its root", target.path)
	}

	response := protocol.Services{
		Project: target.id, File: target.compose,
		Action: request.Action, Service: request.Service,
	}
	switch request.Action {
	case "list":
	case "log", "shell":
		if request.Service == "" {
			return response, fmt.Errorf("which service?")
		}
		kind, label := "logs", "logs "
		if request.Action == "shell" {
			kind, label = "shell", "shell "
		}
		_, err := e.Create(protocol.Create{
			Path:   target.path,
			Type:   kind,
			Name:   label + request.Service,
			Target: request.Service,
		})
		return response, err
	default:
		if err := docker.Act(target.path, target.compose, request.Action, request.Service); err != nil {
			response.Error = err.Error()
			return response, nil
		}
	}

	services, err := docker.Services(target.path, target.compose)
	if err != nil {
		response.Error = err.Error()
		return response, nil
	}
	for _, service := range services {
		response.List = append(response.List, protocol.Service{
			Name:   service.Name,
			State:  service.State,
			Port:   service.Port,
			Health: service.Health,
			Uptime: service.Uptime,
		})
	}

	e.mu.Lock()
	target.docker = docker.Summary(services)
	e.mu.Unlock()
	e.markDirty()
	return response, nil
}
