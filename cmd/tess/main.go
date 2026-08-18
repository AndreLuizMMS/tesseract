// Command ts: opens the Tesseract screen, connecting to the engine — and
// starts the engine if it isn't running.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	term "github.com/charmbracelet/x/term"

	_ "github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/engine"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/screen"
	"github.com/andreluiz/tesseract/internal/theme"
)

// engineWait is how long the screen gives the engine to open the socket
// before giving up.
const engineWait = 5 * time.Second

const usage = `⧉ ts — terminal agent mosaic

  ts                 opens the screen, starting the engine if needed
  ts new <dir>       adds a project without opening the screen
  ts status          engine state and a summary of projects and cells
  ts stop            shuts down the engine and every cell
  ts reset           clears the saved state and tears everything down, keeping the configuration
  ts engine          runs the engine in the foreground (what systemd calls)
`

func main() {
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	var err error
	switch command {
	case "":
		err = openScreen()
	case "engine":
		err = runEngine()
	case "new":
		err = addProject(os.Args[2:])
	case "status":
		err = showStatus()
	case "stop":
		err = stopEngine()
	case "reset":
		err = clearState()
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", command, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// openScreen connects to the engine, starting it if needed, and draws.
func openScreen() error {
	// The opening runs while the engine is being looked for, on one thread,
	// and the connection runs on the other. The animation's time is time
	// that was going to pass anyway — no wait is invented just to show it.
	opening := screen.NewOpening(os.Stderr, animateOpening())

	type arrival struct {
		client  *screen.Client
		initial protocol.State
		took    time.Duration
		err     error
	}
	connected := make(chan arrival, 1)
	go func() {
		// The stopwatch lives in here: what it counts is the engine
		// assembling the grid, not the animation running alongside. Timing
		// the animation together would lie in a number that exists
		// precisely to be true.
		start := time.Now()
		client, err := connectOrStart()
		if err != nil {
			connected <- arrival{err: err}
			return
		}
		// The first snapshot arrives as soon as the screen connects; it's
		// the starting point of the drawing.
		initial, err := firstState(client)
		connected <- arrival{client: client, initial: initial, took: time.Since(start), err: err}
	}()

	opening.Build()

	arrived := <-connected
	if arrived.err != nil {
		if arrived.client != nil {
			arrived.client.Close()
		}
		return arrived.err
	}
	client, initial := arrived.client, arrived.initial
	defer client.Close()

	opening.Tally(engineCount(initial, arrived.took))
	// An empty grid starts with a shell right here: there's always
	// something to look at.
	if len(initial.Projects) == 0 {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		// No type: the engine decides, and the session already comes with
		// the agent tabs inside.
		if err := client.Send(protocol.TypeCreate, protocol.Create{Path: dir}); err != nil {
			return err
		}
	}

	model := screen.NewModel(client, initial)
	program := tea.NewProgram(model)
	model.Listen(program)
	_, err := program.Run()
	return err
}

// animateOpening says whether the opening runs as an animation. With no real
// terminal it would turn into escape-code garbage in a log file, and
// whoever doesn't want to wait for it turns it off in the environment. In a
// panel too narrow for the block's widest line, the animation also
// disappears: the terminal would wrap the line, the fixed cursor-up would
// miscount and leftover garbage would pile up frame after frame — better
// the usual static banner.
func animateOpening() bool {
	if _, off := os.LookupEnv("TESSERACT_NO_OPENING"); off {
		return false
	}
	info, err := os.Stderr.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	width, _, err := term.GetSize(os.Stderr.Fd())
	return err != nil || width >= screen.RequiredWidth()
}

// engineCount is what the engine returned: proof that the grid came back
// whole, put in numbers. It disappears together with the opening once full
// screen opens.
func engineCount(state protocol.State, took time.Duration) []string {
	cells := 0
	for _, project := range state.Projects {
		cells += len(project.Cells)
	}
	return []string{
		screen.EngineLine("session engine: alive"),
		screen.EngineLine(fmt.Sprintf("%s · %s · same position",
			count(cells, "cell recovered", "cells recovered"),
			count(len(state.Projects), "project", "projects"))),
		screen.EngineLine(fmt.Sprintf("grid assembled in %dms", took.Milliseconds())),
	}
}

// count writes the number next to the noun in the right form. One cell isn't
// "1 cells".
func count(howMany int, singular, plural string) string {
	if howMany == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(howMany) + " " + plural
}

// firstState waits for the snapshot the engine sends as soon as the screen
// connects.
func firstState(client *screen.Client) (protocol.State, error) {
	for {
		envelope, err := client.Receive()
		if err != nil {
			return protocol.State{}, err
		}
		if envelope.Type != protocol.TypeState {
			continue
		}
		return protocol.Unpack[protocol.State](envelope)
	}
}

// runEngine is the service: owner of the processes, the history and the
// state.
func runEngine() error {
	stateDir := engine.StateDir()
	config, err := engine.LoadConfig(engine.ConfigPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: unreadable configuration, falling back to default:", err)
	}

	// The service doesn't inherit the user's terminal: without this, the
	// agents and the tools they call wouldn't exist for the engine.
	engine.InheritLoginPath()

	m := engine.New(stateDir, config)
	if err := m.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	go m.Watch()

	server, socketErr := engine.Serve(m, engine.SocketPath())
	if errors.Is(socketErr, engine.ErrEngineAlreadyRunning) {
		// Whoever asked already has what they wanted: exiting cleanly
		// avoids the service entering a restart loop over an engine that's
		// already up.
		fmt.Fprintln(os.Stderr, "an engine is already running")
		m.Shutdown()
		return nil
	}
	if socketErr != nil {
		return socketErr
	}
	defer server.Close()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-signals:
	case <-server.Stopped():
	}
	m.Shutdown()
	return nil
}

// addProject creates a project without opening the screen. A project is
// born together with its first cell, so the cell comes along.
func addProject(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: ts new <dir> [type]")
	}
	path, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	typ := ""
	if len(args) > 1 {
		typ = args[1]
	}

	client, err := connectOrStart()
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Send(protocol.TypeCreate, protocol.Create{Path: path, Type: typ}); err != nil {
		return err
	}
	// The reply is the snapshot with the new cell, or the engine's error.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		envelope, err := client.Receive()
		if err != nil {
			return err
		}
		switch envelope.Type {
		case protocol.TypeError:
			msg, _ := protocol.Unpack[protocol.Error](envelope)
			return errors.New(msg.Message)
		case protocol.TypeState:
			state, _ := protocol.Unpack[protocol.State](envelope)
			for _, project := range state.Projects {
				if project.Path == path {
					fmt.Printf("%s joined the grid\n", project.Name)
					return nil
				}
			}
		}
	}
	return errors.New("the engine did not confirm the creation")
}

// showStatus prints the engine's state and the grid summary.
func showStatus() error {
	client, err := screen.Connect(engine.SocketPath())
	if err != nil {
		fmt.Println("engine not running")
		return nil
	}
	defer client.Close()

	if err := client.Send(protocol.TypeStatus, struct{}{}); err != nil {
		return err
	}
	summary, err := waitSummary(client)
	if err != nil {
		return err
	}
	fmt.Println(theme.Glyph, summary)
	return nil
}

// stopEngine shuts down the engine and every cell.
func stopEngine() error {
	client, err := screen.Connect(engine.SocketPath())
	if err != nil {
		fmt.Println("engine not running")
		return nil
	}
	defer client.Close()

	if err := client.Send(protocol.TypeStop, struct{}{}); err != nil {
		return err
	}
	summary, err := waitSummary(client)
	if err != nil {
		return err
	}
	fmt.Println(summary)
	return nil
}

// clearState tears everything down and forgets the grid, keeping the
// configuration.
func clearState() error {
	if err := stopEngine(); err != nil {
		return err
	}
	// The engine needs to release the files before they disappear.
	waitEngineExit()

	stateDir := engine.StateDir()
	if err := os.Remove(engine.StatePath(stateDir)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(filepath.Join(stateDir, "history")); err != nil {
		return err
	}
	fmt.Println("state cleared; configuration stays where it was")
	return nil
}

func waitEngineExit() {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		client, err := screen.Connect(engine.SocketPath())
		if err != nil {
			return
		}
		client.Close()
		time.Sleep(100 * time.Millisecond)
	}
}

func waitSummary(client *screen.Client) (string, error) {
	for {
		envelope, err := client.Receive()
		if err != nil {
			return "", err
		}
		if envelope.Type != protocol.TypeSummary {
			continue
		}
		summary, err := protocol.Unpack[protocol.Summary](envelope)
		if err != nil {
			return "", err
		}
		return summary.Text, nil
	}
}

// connectOrStart returns a conversation with the engine, starting the
// service when it isn't up yet.
func connectOrStart() (*screen.Client, error) {
	if client, err := screen.Connect(engine.SocketPath()); err == nil {
		return client, nil
	}
	if err := startEngine(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(engineWait)
	for time.Now().Before(deadline) {
		if client, err := screen.Connect(engine.SocketPath()); err == nil {
			return client, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, errors.New("the engine did not respond")
}

// startEngine launches the service in the background, detached from the
// terminal, so it survives the screen closing.
func startEngine() error {
	// With the systemd service installed, whoever starts the engine is it —
	// and then the engine comes back on its own after a `wsl --shutdown`.
	if _, err := exec.LookPath("systemctl"); err == nil {
		if err := exec.Command("systemctl", "--user", "start", "tesseract.service").Run(); err == nil {
			return nil
		}
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}
	stateDir := engine.StateDir()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(stateDir, "engine.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	service := exec.Command(self, "engine")
	service.Stdout = logFile
	service.Stderr = logFile
	service.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return service.Start()
}
