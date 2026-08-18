package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/docker"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// agentThatLogsWhatItReceives is a fake agent that records everything typed
// into it. It's what lets us prove that reattaching doesn't trigger work.
func agentThatLogsWhatItReceives(t *testing.T, dir, log string) string {
	t.Helper()
	path := filepath.Join(dir, "agent-that-logs")
	// Reads byte by byte with dd, unbuffered: the test needs to see even a
	// single character that reached the agent's keyboard.
	body := "#!/bin/sh\nwhile true; do dd bs=1 count=1 2>/dev/null >> " + log + " || exit 0; done\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("preparing agent: %v", err)
	}
	return path
}

func configWithFakeAgent(program string) Config {
	config := testConfig()
	config.Agents = map[string]AgentProfile{
		"claude": {Program: program},
		"cursor": {Program: program},
	}
	return config
}

// TestReconstructionDoesNotTriggerWork is the risk automatic recovery
// creates, cut at the root: the agent wakes up waiting, and not a single byte
// is written to its keyboard.
func TestReconstructionDoesNotTriggerWork(t *testing.T) {
	stateDir := t.TempDir()
	projectDir := t.TempDir()
	log := filepath.Join(t.TempDir(), "received.txt")
	program := agentThatLogsWhatItReceives(t, projectDir, log)

	first := New(stateDir, configWithFakeAgent(program))
	if err := first.Start(); err != nil {
		t.Fatalf("starting: %v", err)
	}
	if _, err := first.Create(protocol.Create{Path: projectDir, Type: "claude", Name: "refactor auth"}); err != nil {
		t.Fatalf("creating: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	first.Shutdown()

	// Clears what the first startup wrote, to measure only the reconstruction.
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatalf("preparing: %v", err)
	}

	second := New(stateDir, configWithFakeAgent(program))
	if err := second.Start(); err != nil {
		t.Fatalf("restarting: %v", err)
	}
	defer second.Shutdown()
	time.Sleep(1500 * time.Millisecond)

	received, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("reading what the agent received: %v", err)
	}
	if len(received) > 0 {
		t.Fatalf("the reconstruction wrote %d byte(s) to the agent's keyboard: %q", len(received), received)
	}

	// And the grid came back whole.
	snapshot := second.Snapshot()
	if len(snapshot.Projects) != 1 || len(snapshot.Projects[0].Cells) != 1 {
		t.Fatalf("the grid did not come back: %#v", snapshot)
	}
	if snapshot.Projects[0].Cells[0].Name != "refactor auth" {
		t.Fatalf("the cell came back with another name: %q", snapshot.Projects[0].Cells[0].Name)
	}
}

// TestReconstructionReattachesTheSameConversation — the conversation's
// identity survives the crash, which is what makes the agent pick back up
// where it left off.
func TestReconstructionReattachesTheSameConversation(t *testing.T) {
	stateDir := t.TempDir()
	projectDir := t.TempDir()
	program := agentThatLogsWhatItReceives(t, projectDir, filepath.Join(t.TempDir(), "junk.txt"))

	first := New(stateDir, configWithFakeAgent(program))
	if err := first.Start(); err != nil {
		t.Fatalf("starting: %v", err)
	}
	if _, err := first.Create(protocol.Create{Path: projectDir, Type: "claude"}); err != nil {
		t.Fatalf("creating: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	first.Shutdown()

	saved, err := Load(StatePath(stateDir))
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	conversation := saved.Projects[0].Cells[0].Conversations["claude"]
	if conversation == "" {
		t.Fatalf("the conversation's identity was not saved: %#v", saved.Projects[0].Cells[0])
	}

	second := New(stateDir, configWithFakeAgent(program))
	if err := second.Start(); err != nil {
		t.Fatalf("restarting: %v", err)
	}
	defer second.Shutdown()
	time.Sleep(300 * time.Millisecond)

	after, err := Load(StatePath(stateDir))
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if after.Projects[0].Cells[0].Conversations["claude"] != conversation {
		t.Fatalf("the conversation changed during reconstruction: %q → %q", conversation, after.Projects[0].Cells[0].Conversations["claude"])
	}
}

// TestDockerDoesNotComeUpOnItsOwn — bringing up the stack is the user's
// decision, even after a WSL crash.
func TestDockerDoesNotComeUpOnItsOwn(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("no docker on this machine")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker is not responding")
	}

	stateDir := t.TempDir()
	projectDir := t.TempDir()
	compose := filepath.Join(projectDir, "docker-compose.yml")
	body := "services:\n  web:\n    image: nginx:alpine\n    command: [\"sh\", \"-c\", \"sleep 600\"]\n"
	if err := os.WriteFile(compose, []byte(body), 0o644); err != nil {
		t.Fatalf("preparing: %v", err)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "compose", "--file", compose, "stop").Run()
		_ = exec.Command("docker", "compose", "--file", compose, "rm", "-f").Run()
	})

	first := New(stateDir, testConfig())
	if err := first.Start(); err != nil {
		t.Fatalf("starting: %v", err)
	}
	if _, err := first.Create(protocol.Create{Path: projectDir, Type: "bash"}); err != nil {
		t.Fatalf("creating: %v", err)
	}
	first.Shutdown()

	second := New(stateDir, testConfig())
	if err := second.Start(); err != nil {
		t.Fatalf("restarting: %v", err)
	}
	go second.Watch()
	defer second.Shutdown()
	time.Sleep(2 * time.Second)

	services, err := docker.Services(projectDir, compose)
	if err != nil {
		t.Fatalf("reading the stack: %v", err)
	}
	for _, service := range services {
		if strings.HasPrefix(service.State, "up") {
			t.Fatalf("the reconstruction brought up the stack on its own: %#v", service)
		}
	}

	// And the project recognizes it has compose, so the panel exists.
	snapshot := second.Snapshot()
	if !snapshot.Projects[0].HasCompose {
		t.Fatal("the project has compose at its root and the engine didn't notice")
	}
}

// TestReconstructionOfACellWithAnUnknownTypeDoesNotBringDownTheEngine —
// state written by a future version can't stop the grid from coming back.
func TestReconstructionOfACellWithAnUnknownTypeDoesNotBringDownTheEngine(t *testing.T) {
	stateDir := t.TempDir()
	projectDir := t.TempDir()
	state := DesiredState{Projects: []SavedProject{{
		ID: "p1", Path: projectDir,
		Cells: []SavedCell{
			{ID: "c1", Kind: "spreadsheet", Name: "from the future"},
			{ID: "c2", Kind: "bash", Name: "tests"},
		},
	}}}
	if err := Save(StatePath(stateDir), state); err != nil {
		t.Fatalf("preparing: %v", err)
	}

	m := New(stateDir, testConfig())
	if err := m.Start(); err != nil {
		t.Fatalf("starting: %v", err)
	}
	defer m.Shutdown()

	snapshot := m.Snapshot()
	if len(snapshot.Projects) != 1 || len(snapshot.Projects[0].Cells) != 2 {
		t.Fatalf("the grid should come back whole, with the unknown cell stopped: %#v", snapshot)
	}
	if snapshot.Notice == "" {
		t.Error("the engine should have warned it didn't know how to bring up a cell")
	}
}
