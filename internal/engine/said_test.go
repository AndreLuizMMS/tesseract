package engine

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// cellWithMod brings up one Claude cell over a fake agent that logs every
// byte typed into it, with a mod attached.
func cellWithMod(t *testing.T) (e *Engine, id, log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(t.TempDir(), "received.txt")
	e = New(t.TempDir(), configWithFakeAgent(agentThatLogsWhatItReceives(t, dir, log)))
	if err := e.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(e.Shutdown)
	id, err := e.Create(protocol.Create{Path: dir, Type: "claude", Name: "auth"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	detach, err := e.Link(id, "", func(cell.Delivery) error { return nil })
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	t.Cleanup(detach)
	return e, id, log
}

func portraitOf(e *Engine, id string) protocol.Cell {
	for _, p := range e.Snapshot().Projects {
		for _, c := range p.Cells {
			if c.ID == id {
				return c
			}
		}
	}
	return protocol.Cell{}
}

// TestAnswerOnlySettlesAToolApproval — enter and esc from the mosaic reach
// the cell's terminal only for what the mod reported as a tool's permission
// dialog, and only once.
func TestAnswerOnlySettlesAToolApproval(t *testing.T) {
	e, id, log := cellWithMod(t)
	typed := func() string {
		content, _ := os.ReadFile(log)
		return string(content)
	}

	if err := e.Answer(protocol.Answer{Cell: id, Approve: true}); err == nil {
		t.Fatal("a cell that isn't blocked took an approval")
	}

	// A question with options isn't a yes or no.
	ask := protocol.Report{Cell: id, Kind: "state", State: "approve", Ask: "question", Detail: "question: which db?", At: 1}
	if err := e.Report(ask); err != nil {
		t.Fatalf("report: %v", err)
	}
	if got := portraitOf(e, id); got.Ask != "question" || got.Detail != "question: which db?" {
		t.Fatalf("the screen should know what the cell waits on: %+v", got)
	}
	if err := e.Answer(protocol.Answer{Cell: id, Approve: true}); err == nil {
		t.Fatal("a question was answered from the mosaic")
	}
	if typed() != "" {
		t.Fatalf("a refused answer typed %q into the cell", typed())
	}

	ask.Ask, ask.Detail, ask.At = "tool", "Bash: pnpm migrate", 2
	if err := e.Report(ask); err != nil {
		t.Fatalf("report: %v", err)
	}
	if err := e.Answer(protocol.Answer{Cell: id, Approve: true}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// The terminal hands the agent a line feed for the enter typed into it.
	if !waitFor(t, 2*time.Second, func() bool { return typed() == "\n" }) {
		t.Fatalf("approving should type one enter, the cell got %q", typed())
	}
	if got := portraitOf(e, id); got.State != string(cell.Working) || got.Ask != "" {
		t.Fatalf("an answered cell stops asking: %+v", got)
	}
	if err := e.Answer(protocol.Answer{Cell: id}); err == nil {
		t.Fatal("the same dialog was answered twice")
	}

	// Declining is esc.
	ask.At = 3
	if err := e.Report(ask); err != nil {
		t.Fatalf("report: %v", err)
	}
	if err := e.Answer(protocol.Answer{Cell: id}); err != nil {
		t.Fatalf("decline: %v", err)
	}
	// The fake agent's terminal holds a line until its enter arrives.
	if err := e.Key(protocol.Key{Cell: id, Code: '\r'}); err != nil {
		t.Fatalf("key: %v", err)
	}
	if !waitFor(t, 2*time.Second, func() bool { return typed() == "\n\x1b\n" }) {
		t.Fatalf("declining should type one esc, the cell got %q", typed())
	}
}

// TestIdleAndInterruptedHaveTheirOwnWord — an agent at its prompt isn't
// working, and a turn the user stopped isn't an answer.
func TestIdleAndInterruptedHaveTheirOwnWord(t *testing.T) {
	e, id, _ := cellWithMod(t)
	at := int64(0)
	say := func(report protocol.Report) protocol.Cell {
		t.Helper()
		at++
		report.Cell, report.Kind, report.At = id, "state", at
		if err := e.Report(report); err != nil {
			t.Fatalf("report %s: %v", report.State, err)
		}
		return portraitOf(e, id)
	}

	if got := say(protocol.Report{State: "idle"}); got.State != "idle" {
		t.Fatalf("a session at its prompt came out %q", got.State)
	}
	if got := say(protocol.Report{State: "interrupted"}); got.State != "interrupted" {
		t.Fatalf("a stopped turn came out %q", got.State)
	}

	long := strings.Repeat("a very long summary ", 6)
	got := say(protocol.Report{State: "answered", Detail: "First line of the answer.", Summary: long})
	if got.State != "answered" || len([]rune(got.Detail)) != summaryLimit {
		t.Fatalf("the header takes the summary, cut to %d: %+v", summaryLimit, got)
	}
	e.mu.Lock()
	notice := e.projects[0].cells[0].takeDetail()
	e.mu.Unlock()
	if notice != got.Detail {
		t.Fatalf("the notice should carry the summary, got %q", notice)
	}
	// Without a summary the opening of the answer is the notice's alone.
	if got := say(protocol.Report{State: "answered", Detail: "First line of the answer."}); got.Detail != "" {
		t.Fatalf("the header showed %q with no summary asked for", got.Detail)
	}

	// Reading it leaves the agent idle, not working.
	if err := e.Key(protocol.Key{Cell: id, Text: "x", Code: 'x'}); err != nil {
		t.Fatalf("key: %v", err)
	}
	if got := portraitOf(e, id); got.State != "idle" {
		t.Fatalf("a read answer leaves the cell %q", got.State)
	}

	// The window's fill is the conversation's, and a bad number is refused.
	if err := e.Report(protocol.Report{Cell: id, Kind: "context", Context: 83.6}); err != nil {
		t.Fatalf("report: %v", err)
	}
	if got := portraitOf(e, id).Context; got != 84 {
		t.Fatalf("context came out %d", got)
	}
	if err := e.Report(protocol.Report{Cell: id, Kind: "context", Context: -1}); err == nil {
		t.Fatal("a negative context was taken")
	}
}

// TestRosterStaysOutOfCells — the roster mod isn't loaded into a cell unless
// the user asks for it.
func TestRosterStaysOutOfCells(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TESSERACT_MODS", root)
	for _, name := range []string{"tesseract-link", "tesseract-roster"} {
		dir := filepath.Join(root, name, ".claude-plugin")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte("{}"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	roster := filepath.Join(root, "tesseract-roster")

	config := DefaultConfig()
	if args := New(t.TempDir(), config).profiles()["claude"].Args; slices.Contains(args, roster) {
		t.Fatalf("the roster was loaded by default: %v", args)
	}
	config.Roster = true
	if args := New(t.TempDir(), config).profiles()["claude"].Args; !slices.Contains(args, roster) {
		t.Fatalf("the roster should load when asked for: %v", args)
	}
}
