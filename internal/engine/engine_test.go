package engine

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"

	_ "github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/protocol"
)

func waitFor(t *testing.T, deadlineIn time.Duration, condition func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(deadlineIn)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return condition()
}

// pasteAndEnter does what the user does inside a cell: pastes the command
// and then sends enter. The paste goes marked as a paste, so the line break
// can't come bundled with the pasted text.
func pasteAndEnter(t *testing.T, e *Engine, id, command string) {
	t.Helper()
	if err := e.Key(protocol.Key{Cell: id, Paste: command}); err != nil {
		t.Fatalf("paste: %v", err)
	}
	if err := e.Key(protocol.Key{Cell: id, Code: vt.KeyEnter}); err != nil {
		t.Fatalf("enter: %v", err)
	}
}

// testConfig turns off the alerts: tests shouldn't beep or pop notifications.
func testConfig() Config {
	config := DefaultConfig()
	config.Sound, config.Notify = false, false
	config.HistoryLimit = 64 << 10
	return config
}

func testEngine(t *testing.T) *Engine {
	t.Helper()
	e := New(t.TempDir(), testConfig())
	if err := e.Start(); err != nil {
		t.Fatalf("start engine: %v", err)
	}
	t.Cleanup(e.Shutdown)
	return e
}

// TestCreateSpawnsProjectAlongside — creating a project and creating a cell
// are the same gesture.
func TestCreateSpawnsProjectAlongside(t *testing.T) {
	e := testEngine(t)
	dir := t.TempDir()

	id, err := e.Create(protocol.Create{Path: dir, Type: "bash"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	snapshot := e.Snapshot()
	if len(snapshot.Projects) != 1 || len(snapshot.Projects[0].Cells) != 1 {
		t.Fatalf("expected one project with one cell: %#v", snapshot)
	}
	if snapshot.Projects[0].Cells[0].ID != id {
		t.Fatalf("the created cell is not the one on screen")
	}
	if name := snapshot.Projects[0].Cells[0].Name; name != "bash 1" {
		t.Fatalf("auto name came out %q, expected \"bash 1\"", name)
	}

	// A second cell on the same path joins the same project.
	if _, err := e.Create(protocol.Create{Path: dir, Type: "bash"}); err != nil {
		t.Fatalf("create second: %v", err)
	}
	snapshot = e.Snapshot()
	if len(snapshot.Projects) != 1 || len(snapshot.Projects[0].Cells) != 2 {
		t.Fatalf("the second cell should have joined the same project: %#v", snapshot)
	}
}

// TestKillLastCellTakesProjectOffScreen and disk stays untouched.
func TestKillLastCellTakesProjectOffScreen(t *testing.T) {
	e := testEngine(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "user-file.txt")
	if err := os.WriteFile(marker, []byte("intact"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	first, err := e.Create(protocol.Create{Path: dir, Type: "bash"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	second, err := e.Create(protocol.Create{Path: dir, Type: "bash"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if e.IsLastCell(first) {
		t.Fatal("with two cells, neither is the last")
	}
	if err := e.Kill(first); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if len(e.Snapshot().Projects) != 1 {
		t.Fatal("killing a non-last cell must not remove the project")
	}

	if !e.IsLastCell(second) {
		t.Fatal("the one that's left is the project's last")
	}
	if err := e.Kill(second); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if len(e.Snapshot().Projects) != 0 {
		t.Fatal("the project should have left the screen with its last cell")
	}

	if content, err := os.ReadFile(marker); err != nil || string(content) != "intact" {
		t.Fatalf("the project's directory was touched: %v %q", err, content)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the project's directory vanished from disk: %v", err)
	}
}

// TestCreateWithInvalidPathDoesNotChangeState protects the grid from a bad
// request.
func TestCreateWithInvalidPathDoesNotChangeState(t *testing.T) {
	e := testEngine(t)

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := e.Create(protocol.Create{Path: missing, Type: "bash"}); err == nil {
		t.Fatal("a missing path should fail")
	} else if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("unclear message: %v", err)
	}

	readOnly := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(readOnly, 0o500); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if os.Geteuid() != 0 { // root writes anywhere
		if _, err := e.Create(protocol.Create{Path: readOnly, Type: "bash"}); err == nil {
			t.Fatal("a directory with no write permission should fail")
		} else if !strings.Contains(err.Error(), "permission") {
			t.Fatalf("unclear message: %v", err)
		}
	}

	if _, err := e.Create(protocol.Create{Path: t.TempDir(), Type: "spreadsheet"}); err == nil {
		t.Fatal("a nonexistent type should fail")
	}

	if len(e.Snapshot().Projects) != 0 {
		t.Fatalf("no invalid request should have touched the grid: %#v", e.Snapshot())
	}
}

// TestReconstructionKeepsTheGrid is what saves the work after a WSL crash:
// the engine dies, comes back up, and the grid returns the same.
func TestReconstructionKeepsTheGrid(t *testing.T) {
	stateDir := t.TempDir()
	projectDir := t.TempDir()

	first := New(stateDir, testConfig())
	if err := first.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := first.Create(protocol.Create{Path: projectDir, Type: "bash", Name: "tests"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := first.Create(protocol.Create{Path: projectDir, Type: "bash", Name: "build"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	before := first.Snapshot()
	first.Shutdown()

	second := New(stateDir, testConfig())
	if err := second.Start(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	defer second.Shutdown()

	after := second.Snapshot()
	if len(after.Projects) != 1 || len(after.Projects[0].Cells) != 2 {
		t.Fatalf("the grid did not come back: %#v", after)
	}
	for i, cellBefore := range before.Projects[0].Cells {
		cellAfter := after.Projects[0].Cells[i]
		if cellBefore.ID != cellAfter.ID || cellBefore.Type != cellAfter.Type || cellBefore.Name != cellAfter.Name {
			t.Fatalf("cell %d came back different:\nbefore %#v\nafter %#v", i, cellBefore, cellAfter)
		}
	}
}

// TestKeyReachesCell closes the vertical slice: a key goes in through the
// engine and the output shows up in the snapshot.
func TestKeyReachesCell(t *testing.T) {
	e := testEngine(t)
	id, err := e.Create(protocol.Create{Path: t.TempDir(), Type: "bash"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	pasteAndEnter(t, e, id, "echo tesseract")
	if !waitFor(t, 2*time.Second, func() bool {
		return strings.Contains(strings.Join(e.Snapshot().Projects[0].Cells[0].Lines, "\n"), "tesseract")
	}) {
		t.Fatalf("the output did not reach the snapshot:\n%#v", e.Snapshot().Projects[0].Cells[0].Lines)
	}
}

// TestSocketServesAScreen covers the whole path through the protocol.
func TestSocketServesAScreen(t *testing.T) {
	e := testEngine(t)
	socketPath := filepath.Join(t.TempDir(), "engine.sock")
	server, err := Serve(e, socketPath)
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	defer server.Close()

	if _, err := Serve(e, socketPath); err == nil {
		t.Fatal("two engines on the same socket cannot coexist")
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)
	request := func(kind string, data any) {
		t.Helper()
		envelope, err := protocol.Pack(kind, data)
		if err != nil {
			t.Fatalf("pack: %v", err)
		}
		if err := encoder.Encode(envelope); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	request(protocol.TypeCreate, protocol.Create{Path: t.TempDir(), Type: "bash"})

	// The first snapshot with a cell arrives on its own, without the screen
	// asking.
	var cellID string
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for cellID == "" {
		var envelope protocol.Message
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatalf("read snapshot: %v", err)
		}
		switch envelope.Type {
		case protocol.TypeState:
			state, err := protocol.Unpack[protocol.State](envelope)
			if err != nil {
				t.Fatalf("unpack state: %v", err)
			}
			if len(state.Projects) == 1 && len(state.Projects[0].Cells) == 1 {
				cellID = state.Projects[0].Cells[0].ID
			}
		case protocol.TypeError:
			errMsg, _ := protocol.Unpack[protocol.Error](envelope)
			t.Fatalf("the engine refused the request: %s", errMsg.Message)
		}
	}

	request(protocol.TypeSize, protocol.Size{Cell: cellID, Cols: 60, Rows: 10})
	request(protocol.TypeKey, protocol.Key{Cell: cellID, Paste: "echo through-the-socket"})
	request(protocol.TypeKey, protocol.Key{Cell: cellID, Code: vt.KeyEnter})

	found := false
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for !found {
		var envelope protocol.Message
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatalf("read snapshot: %v", err)
		}
		if envelope.Type != protocol.TypeState {
			continue
		}
		state, err := protocol.Unpack[protocol.State](envelope)
		if err != nil {
			t.Fatalf("unpack state: %v", err)
		}
		if len(state.Projects) == 0 {
			continue
		}
		found = strings.Contains(strings.Join(state.Projects[0].Cells[0].Lines, "\n"), "through-the-socket")
	}

	request(protocol.TypeStatus, struct{}{})
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var envelope protocol.Message
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatalf("read summary: %v", err)
		}
		if envelope.Type != protocol.TypeSummary {
			continue
		}
		summary, err := protocol.Unpack[protocol.Summary](envelope)
		if err != nil {
			t.Fatalf("unpack summary: %v", err)
		}
		if !strings.Contains(summary.Text, "1 cell") {
			t.Fatalf("summary doesn't count the cell: %q", summary.Text)
		}
		break
	}
}
