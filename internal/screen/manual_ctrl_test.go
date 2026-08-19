package screen

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andreluiz/tesseract/internal/engine"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// TestManualCtrlEOpensIDE is the end-to-end validation of the shortcut: the
// ctrl+e key on the real screen, a real socket, a real engine. It opens the
// picker, waits for the detected list, presses enter on the first IDE, and
// checks the spy (with the same anatomy as Cursor's remote install) got the
// project's path and the live window's socket — exactly what opens the
// folder in the open window.
//
// Only runs with MANUAL=1: brings up a real engine and cell.
//
//	MANUAL=1 go test ./internal/screen -run TestManualCtrlEOpensIDE -v
//
// To target a real IDE, with a Cursor window open on the Windows side, put
// its binary on Windows' PATH (or use a real "code"/"codium"/"cursor" that's
// already there) and skip SOCKET_DA_JANELA to exercise the cmd.exe launch
// instead of the reuse-the-open-window path:
//
//	MANUAL=1 EDITOR_DO_TESTE=cursor SOCKET_DA_JANELA=/run/user/1000/vscode-ipc-....sock ...
func TestManualCtrlEOpensIDE(t *testing.T) {
	if os.Getenv("MANUAL") == "" {
		t.Skip("manual")
	}

	// An XDG_RUNTIME_DIR just for this engine, so it doesn't fight the
	// engine socket already running on the machine. The IDE window's socket
	// goes in here.
	run := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", run)

	editor := os.Getenv("EDITOR_DO_TESTE")
	record := filepath.Join(t.TempDir(), "chamada.txt")
	if editor == "" {
		editor = "spy"
		t.Setenv("HOME", t.TempDir())
		createSpyIDE(t, editor, record)
	}

	if live := os.Getenv("SOCKET_DA_JANELA"); live != "" {
		if err := os.Symlink(live, filepath.Join(run, "vscode-ipc-teste.sock")); err != nil {
			t.Fatal(err)
		}
	} else {
		listener, err := net.Listen("unix", filepath.Join(run, "vscode-ipc-teste.sock"))
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
	}

	project := os.Getenv("PROJETO_DO_TESTE")
	if project == "" {
		project = t.TempDir()
	}

	m := engine.New(t.TempDir(), engine.DefaultConfig())
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown()

	server, err := engine.Serve(m, engine.SocketPath())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	if _, err := m.Create(protocol.Create{Path: project, Type: "bash"}); err != nil {
		t.Fatal(err)
	}

	client, err := Connect(engine.SocketPath())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	model := NewModel(client, m.Snapshot())
	model.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	if model.idePicker == nil {
		t.Fatal("ctrl+e should have opened the IDE picker")
	}

	// The picker's own detection only finds what's really on the machine;
	// the spy poses as one, wired straight in, so the pick is deterministic
	// regardless of what else happens to be installed here.
	model.idePicker.Received(protocol.IDEs{Project: model.idePicker.Project, List: []protocol.IDE{
		{ID: editor, Label: editor, Location: "windows"},
	}})
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// The request crosses the socket; the error, if any, comes back the same
	// way.
	time.Sleep(2 * time.Second)
	if model.err != "" {
		t.Fatalf("ctrl+e complained: %s", model.err)
	}

	if os.Getenv("EDITOR_DO_TESTE") != "" {
		t.Log("ctrl+e went through with no error — check the IDE window")
		return
	}
	call, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the IDE was not called: %v", err)
	}
	text := string(call)
	if !strings.Contains(text, "PASTA="+project) {
		t.Fatalf("the IDE did not receive the project path: %s", text)
	}
	if !strings.Contains(text, "JANELA="+filepath.Join(run, "vscode-ipc-teste.sock")) {
		t.Fatalf("the IDE did not receive the live window's socket: %s", text)
	}
	t.Logf("ctrl+e reached the IDE: %s", strings.TrimSpace(text))
}

// createSpyIDE builds the same anatomy as the IDE's remote install
// (~/.<name>-server/bin/<version>/bin/remote-cli/<name>) with a script
// standing in for the CLI, so the test can read what the engine sent.
func createSpyIDE(t *testing.T, name, record string) {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, "."+name+"-server", "bin", "abc123", "bin", "remote-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		"printf 'PASTA=%s\\nJANELA=%s\\n' \"$1\" \"$VSCODE_IPC_HOOK_CLI\" > " + record + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
