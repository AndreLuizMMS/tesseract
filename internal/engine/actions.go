package engine

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
					return fmt.Errorf("%s is already running. ↵ enters it.", c.name)
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
		return fmt.Errorf("%s has a single screen. No tab to switch to.", c.name)
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
		return fmt.Errorf("%s takes no prompt. ↵ enters it to type.", c.name)
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
		return fmt.Errorf("%s has no conversation to name. ctrl+r works on an agent's cell.", c.name)
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
		return fmt.Errorf("%s has no conversation to name. ctrl+r works on an agent's cell.", c.name)
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
// windowsExe is the install's own binary, relative to where Windows puts
// third-party programs (a per-user install under AppData, or one of the two
// Program Files, machine-wide). Installers don't reliably add themselves to
// Windows' PATH — VS Code's own install on this project's dev machine
// isn't on it — so PATH is a bonus signal, checked separately, not the only
// way in.
var ideCandidates = []struct{ ID, Label, WindowsExe string }{
	{"code", "VS Code", "Microsoft VS Code/Code.exe"},
	{"code-insiders", "VS Code Insiders", "Microsoft VS Code Insiders/Code - Insiders.exe"},
	{"codium", "VSCodium", "VSCodium/VSCodium.exe"},
	{"cursor", "Cursor", "cursor/Cursor.exe"},
}

// windowsExeFor looks up a candidate's Windows binary by id. Empty for an id
// outside the known set — windowsInstalled then falls straight to the PATH
// check.
func windowsExeFor(id string) string {
	for _, c := range ideCandidates {
		if c.ID == id {
			return c.WindowsExe
		}
	}
	return ""
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
			_, onWindows := windowsInstalled(c.ID, c.WindowsExe)
			// The launcher a Windows install drops on WSL's PATH is that same
			// install: it shows up once, as windows.
			if binary := linuxInstall(c.ID); binary != "" && !(onWindows && windowsDrive.MatchString(binary)) {
				add(protocol.IDE{ID: c.ID, Label: c.Label, Location: "wsl"})
			}
			if onWindows {
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

// linuxInstall is where the id's binary on PATH really lives, links
// followed, or empty when there's none. The remote CLI an IDE drops under
// ~/.<name>-server only talks to a window someone else opened — it doesn't
// count as an install of its own.
func linuxInstall(id string) string {
	binary, err := exec.LookPath(id)
	if err != nil || strings.Contains(binary, "-server/") {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(binary); err == nil {
		return resolved
	}
	return binary
}

// windowsDrive is a path on a Windows disk, as WSL mounts it.
var windowsDrive = regexp.MustCompile(`^/mnt/[a-zA-Z]/`)

// windowsRoots is where Windows keeps programs: a per-user install and the
// two machine-wide Program Files, 64 and 32-bit alike.
var windowsRoots = []string{
	"/mnt/c/Users/*/AppData/Local/Programs",
	"/mnt/c/Program Files",
	"/mnt/c/Program Files (x86)",
}

// windowsInstalled finds the id on the Windows side and says how to run it.
// The install's own binary, found on disk under one of the usual program
// roots, is the reliable signal — Windows' PATH, asked through cmd.exe, is
// only a fallback: installers don't reliably add themselves to it. Either
// way, only makes sense inside WSL — outside it there's no Windows on the
// other end to ask.
func windowsInstalled(id, windowsExe string) (target string, ok bool) {
	if windowsExe != "" {
		var found []string
		for _, root := range windowsRoots {
			matches, _ := filepath.Glob(filepath.Join(root, windowsExe))
			found = append(found, matches...)
		}
		if path := newest(found); path != "" {
			return path, true
		}
	}
	if _, err := os.Stat(cmdExePath); err != nil {
		return "", false
	}
	if exec.Command(cmdExePath, "/c", "where", id).Run() == nil {
		return id, true
	}
	return "", false
}

// OpenInEditor opens the project directory in the picked IDE, at the place
// the picker said it lives. With a cell, it opens the last file that cell's
// agent edited, at the line.
func (e *Engine) OpenInEditor(projectID, id, location, cellID string) error {
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
		return fmt.Errorf("No IDE picked. ↑↓ picks one, ↵ opens it.")
	}
	file, line := "", 0
	if cellID != "" {
		var err error
		if file, line, err = e.lastEdit(cellID); err != nil {
			return err
		}
	}

	// The `code`/`codium` launcher inside WSL asks, on stderr, whether to
	// really open the Linux install, and waits for an answer on stdin. The
	// engine is a service and has no stdin: the read hits EOF, the answer
	// comes out empty, and the launcher exits without opening anything. This
	// variable is the launcher's own way out of that prompt.
	environment := append(withDisplay(os.Environ()), "DONT_PROMPT_WSL_INSTALL=1")
	binaryName, arguments := id, editorArguments(path, "", file, line)
	if location == "windows" {
		// A window already open for this IDE is reused through its remote
		// CLI: the socket it leaves behind on WSL keeps responding even
		// after the window has closed, so "socket alive" is what proves a
		// window is still there to receive the request.
		if cli, window := ideAlreadyOpen(id); cli != "" {
			binaryName = cli
			environment = append(environment, "VSCODE_IPC_HOOK_CLI="+window)
		} else {
			distro := wslDistro()
			if distro == "" {
				return fmt.Errorf("WSL distro name not found. ctrl+e picks an IDE installed on Linux.")
			}
			target, ok := windowsInstalled(id, windowsExeFor(id))
			if !ok {
				return fmt.Errorf("%s isn't installed on the Windows side. ctrl+e picks another IDE.", id)
			}
			// The install's own binary, found on disk: WSL can run a Windows
			// .exe directly, no cmd.exe or PATH in the middle.
			binaryName, arguments = target, editorArguments(path, distro, file, line)
			if target == id {
				// Only found through Windows' PATH — cmd.exe is what
				// resolves a bare name into that PATH.
				binaryName, arguments = cmdExePath, append([]string{"/c", "start", "", id}, arguments...)
			}
		}
	}
	command := exec.Command(binaryName, arguments...)
	command.Dir = path
	command.Env = environment
	return launchDetached(command)
}

// editorArguments is what the IDE is asked to open: the project's directory
// and, on top of it, the file at the line when there is one. distro is set
// when the IDE runs on the Windows side and reaches the project through WSL.
func editorArguments(path, distro, file string, line int) []string {
	var goTo []string
	if file != "" {
		goTo = []string{"--goto", file + ":" + strconv.Itoa(line)}
	}
	switch {
	case distro == "":
		return append([]string{path}, goTo...)
	case file == "":
		return []string{"--folder-uri", "vscode-remote://wsl+" + distro + path}
	}
	return append([]string{"--remote", "wsl+" + distro, path}, goTo...)
}

// launchDetached starts the IDE with nothing tying it to the engine: a
// session of its own, and its output in a file instead of a pipe the engine
// would have to keep reading. So the call comes back at once, and the IDE
// isn't taken down by a signal meant for the engine or by a pipe that closed.
//
// ponytail: under systemd the IDE still shares the engine's control group,
// and stopping the service kills a Linux IDE it opened. `systemd-run --scope`
// around the command lifts that, if it ever matters — a Windows IDE and a
// window that was already open aren't affected.
func launchDetached(command *exec.Cmd) error {
	name := filepath.Base(command.Path)
	output, err := os.CreateTemp("", "ts-ide-*.log")
	if err != nil {
		return fmt.Errorf("Could not open %s (%w). ctrl+e picks another IDE.", name, err)
	}
	defer os.Remove(output.Name())
	defer output.Close()
	command.Stdout, command.Stderr = output, output
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return fmt.Errorf("Could not open %s (%w). ctrl+e picks another IDE.", name, err)
	}

	// The IDE that opens a window stays alive; the one that didn't dies right
	// away. Half a second separates the two, and that's what gets the error to
	// show up on screen instead of the shortcut looking like it did nothing.
	died := make(chan error, 1)
	go func() { died <- command.Wait() }()
	select {
	case err := <-died:
		if err != nil {
			said, _ := os.ReadFile(output.Name())
			return fmt.Errorf("%s did not open: %s. ctrl+e picks another IDE.", name, strings.TrimSuffix(firstLine(string(said), err), "."))
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
	found := remoteCLIs(home, name)
	if len(found) == 0 {
		return "", ""
	}
	window = liveWindow(os.Getenv("XDG_RUNTIME_DIR"))
	if window == "" {
		return "", ""
	}
	return newest(found), window
}

// serverFolders is where each IDE keeps its remote server under the home
// directory, for the ones whose folder isn't named after the launcher: VS
// Code's launcher is `code`, and its server lives in ~/.vscode-server.
var serverFolders = map[string]string{
	"code":          ".vscode-server",
	"code-insiders": ".vscode-server-insiders",
	"codium":        ".vscodium-server",
}

// remoteCLIs lists every install of the IDE's remote CLI on this side — one
// per server version the IDE has ever pushed here.
func remoteCLIs(home, name string) []string {
	folder, known := serverFolders[name]
	if !known {
		folder = "." + name + "-server"
	}
	found, _ := filepath.Glob(filepath.Join(home, folder, "bin", "*", "bin", "remote-cli", name))
	return found
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
		return protocol.Services{}, fmt.Errorf("No compose file at the root of %s. Add compose.yml there, then d opens the panel.", filepath.Base(target.path))
	}

	response := protocol.Services{
		Project: target.id, File: target.compose,
		Action: request.Action, Service: request.Service,
	}
	switch request.Action {
	case "list":
	case "log", "shell":
		if request.Service == "" {
			return response, fmt.Errorf("No service picked. ↑↓ picks one.")
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
	target.docker, target.dockerDown = docker.Summary(services), docker.AnyFell(services)
	e.mu.Unlock()
	e.markDirty()
	return response, nil
}
