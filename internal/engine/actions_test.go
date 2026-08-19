package engine

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/protocol"
)

// TestPromptReachesTheCellWithoutEnteringIt — the prompt key sends work to
// the agent without the user entering the cell.
func TestPromptReachesTheCellWithoutEnteringIt(t *testing.T) {
	stateDir := t.TempDir()
	projectDir := t.TempDir()
	log := filepath.Join(t.TempDir(), "received.txt")
	program := agentThatLogsWhatItReceives(t, projectDir, log)

	m := New(stateDir, configWithFakeAgent(program))
	if err := m.Start(); err != nil {
		t.Fatalf("starting: %v", err)
	}
	defer m.Shutdown()

	id, err := m.Create(protocol.Create{Path: projectDir, Type: "claude"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	if err := m.Prompt(id, "covers the mobile menu too"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	// The whole text and the enter at the end: without the enter, the turn
	// doesn't start. The terminal delivers the enter as an end of line.
	if !waitFor(t, 5*time.Second, func() bool {
		content, _ := os.ReadFile(log)
		return strings.Contains(string(content), "covers the mobile menu too\n")
	}) {
		content, _ := os.ReadFile(log)
		t.Fatalf("the prompt didn't reach the agent whole, received: %q", content)
	}
}

// TestPromptOnACellThatDoesNotAcceptItFailsClearly — a shell doesn't accept
// prompts.
func TestPromptOnACellThatDoesNotAcceptItFailsClearly(t *testing.T) {
	m := testEngine(t)
	id, err := m.Create(protocol.Create{Path: t.TempDir(), Type: "bash"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	err = m.Prompt(id, "oi")
	if err == nil {
		t.Fatal("a cell without prompt support should have refused")
	}
	if !strings.Contains(err.Error(), "prompt") {
		t.Fatalf("unclear message: %v", err)
	}
}

// TestAutoRenameSendsThePureCommand — Ctrl+R doesn't ask for a name: it sends
// the plain /rename for the agent to choose, and arms the adoption wait.
func TestAutoRenameSendsThePureCommand(t *testing.T) {
	stateDir := t.TempDir()
	projectDir := t.TempDir()
	log := filepath.Join(t.TempDir(), "received.txt")
	program := agentThatLogsWhatItReceives(t, projectDir, log)

	m := New(stateDir, configWithFakeAgent(program))
	if err := m.Start(); err != nil {
		t.Fatalf("starting: %v", err)
	}
	defer m.Shutdown()

	id, err := m.Create(protocol.Create{Path: projectDir, Type: "claude"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	nameBefore := m.Snapshot().Projects[0].Cells[0].Name
	if err := m.AutoRename(id); err != nil {
		t.Fatalf("auto rename: %v", err)
	}

	// The plain command, with no name at all, reached the agent…
	if !waitFor(t, 3*time.Second, func() bool {
		content, _ := os.ReadFile(log)
		return strings.Contains(string(content), "/rename\n")
	}) {
		content, _ := os.ReadFile(log)
		t.Fatalf("the command did not arrive plain, received: %q", content)
	}
	// …and the label doesn't change until the agent replies with a real name.
	if m.Snapshot().Projects[0].Cells[0].Name != nameBefore {
		t.Fatal("the label could not change before the agent replied")
	}
}

// TestAutoRenameOnACellWithoutConversationFailsClearly — bash, logs and md
// have no named conversation, so there's nothing to ask.
func TestAutoRenameOnACellWithoutConversationFailsClearly(t *testing.T) {
	m := testEngine(t)
	id, err := m.Create(protocol.Create{Path: t.TempDir(), Type: "bash"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	if err := m.AutoRename(id); err == nil {
		t.Fatal("a cell without conversation should have refused")
	}
}

// TestAdoptNameWarnsWhenTheConversationHasNoName — instead of renaming to
// empty.
func TestAdoptNameWarnsWhenTheConversationHasNoName(t *testing.T) {
	stateDir := t.TempDir()
	projectDir := t.TempDir()
	program := agentThatLogsWhatItReceives(t, projectDir, filepath.Join(t.TempDir(), "junk.txt"))

	m := New(stateDir, configWithFakeAgent(program))
	if err := m.Start(); err != nil {
		t.Fatalf("starting: %v", err)
	}
	defer m.Shutdown()

	id, err := m.Create(protocol.Create{Path: projectDir, Type: "claude"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	nameBefore := m.Snapshot().Projects[0].Cells[0].Name

	err = m.AdoptAgentName(id)
	if err == nil {
		t.Fatal("with no conversation name, adopting should have warned")
	}
	if m.Snapshot().Projects[0].Cells[0].Name != nameBefore {
		t.Fatal("the cell's name could not change")
	}
}

// TestResumeBringsUpACrashedCell — nothing restarts on its own; the resume
// key is what brings it back up.
func TestResumeBringsUpACrashedCell(t *testing.T) {
	m := testEngine(t)
	id, err := m.Create(protocol.Create{Path: t.TempDir(), Type: "bash"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	pasteAndEnter(t, m, id, "exit")
	if !waitFor(t, 5*time.Second, func() bool {
		return m.Snapshot().Projects[0].Cells[0].State == "down"
	}) {
		t.Fatalf("the cell should have crashed, it's %q", m.Snapshot().Projects[0].Cells[0].State)
	}
	// Stays crashed: nothing restarts on its own.
	time.Sleep(1500 * time.Millisecond)
	if m.Snapshot().Projects[0].Cells[0].State != "down" {
		t.Fatal("a crashed cell can't restart on its own")
	}

	if err := m.Resume(id); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !waitFor(t, 5*time.Second, func() bool {
		return m.Snapshot().Projects[0].Cells[0].State == "working"
	}) {
		t.Fatalf("after resuming, the cell is %q", m.Snapshot().Projects[0].Cells[0].State)
	}
}

// TestResumeRefusesALiveCell — resuming something already running makes no
// sense.
func TestResumeRefusesALiveCell(t *testing.T) {
	m := testEngine(t)
	id, err := m.Create(protocol.Create{Path: t.TempDir(), Type: "bash"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	if err := m.Resume(id); err == nil {
		t.Fatal("resuming a live cell should have warned")
	}
}

// TestSearchInTheCellHistory — the search finds what the cell wrote.
func TestSearchInTheCellHistory(t *testing.T) {
	m := testEngine(t)
	id, err := m.Create(protocol.Create{Path: t.TempDir(), Type: "bash"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	pasteAndEnter(t, m, id, "echo needle-in-history")
	if !waitFor(t, 3*time.Second, func() bool {
		found, _ := m.Search(id, "needle-in-history")
		return len(found) > 0
	}) {
		t.Fatal("the search didn't find what the cell wrote")
	}

	found, err := m.Search(id, "needle-in-history")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if found[0].Line < 1 {
		t.Fatalf("the match came with no line number: %#v", found[0])
	}
	if err := m.GoToLine(id, found[0].Line); err != nil {
		t.Fatalf("go to line: %v", err)
	}
}

// TestEditorWithNoConfigurationWarns.
func TestEditorWithNoConfigurationWarns(t *testing.T) {
	config := testConfig()
	config.Editor = ""
	m := New(t.TempDir(), config)
	if err := m.Start(); err != nil {
		t.Fatalf("starting: %v", err)
	}
	defer m.Shutdown()

	if _, err := m.Create(protocol.Create{Path: t.TempDir(), Type: "bash"}); err != nil {
		t.Fatalf("creating: %v", err)
	}
	projectID := m.Snapshot().Projects[0].ID
	if err := m.OpenInEditor(projectID); err == nil {
		t.Fatal("with no editor configured, it should have warned")
	}
}

// TestDockerRefusesAProjectWithoutCompose — a project without compose has no
// panel.
func TestDockerRefusesAProjectWithoutCompose(t *testing.T) {
	m := testEngine(t)
	if _, err := m.Create(protocol.Create{Path: t.TempDir(), Type: "bash"}); err != nil {
		t.Fatalf("creating: %v", err)
	}
	projectID := m.Snapshot().Projects[0].ID
	if _, err := m.Docker(protocol.Docker{Project: projectID, Action: "list"}); err == nil {
		t.Fatal("a project without compose has no panel")
	}
}

// TestDockerRefusesAnUnknownAction — the action list is closed, and that's
// how nothing destructive gets in.
func TestDockerRefusesAnUnknownAction(t *testing.T) {
	m := testEngine(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if _, err := m.Create(protocol.Create{Path: dir, Type: "bash"}); err != nil {
		t.Fatalf("creating: %v", err)
	}
	projectID := m.Snapshot().Projects[0].ID

	response, err := m.Docker(protocol.Docker{Project: projectID, Action: "down -v"})
	if err == nil && response.Error == "" {
		t.Fatal("a destructive action must not go through")
	}
}

// TestLiveWindowIgnoresADeadSocket makes sure the socket left behind by a
// window that has already closed is never chosen: only what still accepts a
// connection gets in.
func TestLiveWindowIgnoresADeadSocket(t *testing.T) {
	dir := t.TempDir()

	dead := filepath.Join(dir, "vscode-ipc-dead.sock")
	if err := os.WriteFile(dead, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	alive := filepath.Join(dir, "vscode-ipc-alive.sock")
	listener, err := net.Listen("unix", alive)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	if found := liveWindow(dir); found != alive {
		t.Fatalf("expected the live socket %s, got %q", alive, found)
	}

	if found := liveWindow(t.TempDir()); found != "" {
		t.Fatalf("with no open window there's no socket, got %q", found)
	}
}

// TestEditorDoesNotStallOnTheWSLPrompt — the `code`/`codium` launcher inside
// WSL asks on stdin whether to open the Linux install, and the engine has no
// stdin to answer with. The fake editor here refuses exactly like the real
// launcher does, and only opens when the variable that skips the question is
// in the environment.
func TestEditorDoesNotStallOnTheWSLPrompt(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "opened")
	fake := filepath.Join(dir, "fake-editor")
	script := "#!/bin/sh\nif [ -z \"$DONT_PROMPT_WSL_INSTALL\" ]; then\n\tread -r answer\n\texit 1\nfi\nprintf '%s' \"$1\" > " + marker + "\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("preparing: %v", err)
	}

	config := testConfig()
	config.Editor = fake
	m := New(t.TempDir(), config)
	if err := m.Start(); err != nil {
		t.Fatalf("starting: %v", err)
	}
	defer m.Shutdown()

	project := t.TempDir()
	if _, err := m.Create(protocol.Create{Path: project, Type: "bash"}); err != nil {
		t.Fatalf("creating: %v", err)
	}
	if err := m.OpenInEditor(m.Snapshot().Projects[0].ID); err != nil {
		t.Fatalf("the editor should have opened: %v", err)
	}
	opened, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("the editor didn't open: %v", err)
	}
	if string(opened) != project {
		t.Fatalf("opened %q, expected the project's directory %q", opened, project)
	}
}
