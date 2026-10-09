package engine

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/protocol"
)

func cellOf(e *Engine, id string) protocol.Cell {
	for _, p := range e.Snapshot().Projects {
		for _, c := range p.Cells {
			if c.ID == id {
				return c
			}
		}
	}
	return protocol.Cell{}
}

// sessionWithFiles brings up a session whose agents are fake, in a project
// that has a markdown and a source file to report.
func sessionWithFiles(t *testing.T) (e *Engine, id, doc, source string) {
	t.Helper()
	dir := t.TempDir()
	program := agentThatLogsWhatItReceives(t, dir, filepath.Join(t.TempDir(), "received.txt"))
	e = New(t.TempDir(), configWithFakeAgent(program))
	if err := e.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(e.Shutdown)
	id, err := e.Create(protocol.Create{Path: dir, Type: "session", Name: "auth"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	doc, source = filepath.Join(dir, "docs", "plan.md"), filepath.Join(dir, "main.go")
	if err := os.MkdirAll(filepath.Dir(doc), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, text := range map[string]string{doc: "# The plan\n\nstep one\n", source: "package main\n"} {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return e, id, doc, source
}

// TestAWrittenMarkdownOpensInTheCellsReader — the md mod only says a file was
// written; the reader that shows it is the cell's own md tab, already on the
// file.
func TestAWrittenMarkdownOpensInTheCellsReader(t *testing.T) {
	e, id, doc, _ := sessionWithFiles(t)

	if err := e.OpenDoc(id); err == nil {
		t.Fatal("with nothing written, there is nothing to read")
	}
	if err := e.Report(protocol.Report{Cell: id, Tab: "claude", Kind: "doc", File: doc}); err != nil {
		t.Fatalf("report: %v", err)
	}
	if got := cellOf(e, id).Doc; got != filepath.Join("docs", "plan.md") {
		t.Fatalf("the screen should hear about the file, relative to the project: %q", got)
	}

	if err := e.OpenDoc(id); err != nil {
		t.Fatalf("open: %v", err)
	}
	shown := cellOf(e, id)
	if shown.Tab != "md" {
		t.Fatalf("the cell should be on its md tab, it's on %q", shown.Tab)
	}
	// The reader's header names the file and the way back to the list.
	if len(shown.Lines) == 0 || !strings.Contains(shown.Lines[0], "docs/plan.md") || !strings.Contains(shown.Lines[0], "back to list") {
		t.Fatalf("the reader should be on the file, not on the list: %q", shown.Lines)
	}
	if shown.Doc != "" {
		t.Fatalf("a file that was opened stops being announced: %q", shown.Doc)
	}
}

// TestAWrittenMarkdownOpensBesideACellWithNoReader — a cell that is a single
// agent has no md tab: the file opens in a reading cell of the same project.
func TestAWrittenMarkdownOpensBesideACellWithNoReader(t *testing.T) {
	e, id, _, _ := linkedCell(t)
	project := e.Snapshot().Projects[0]
	doc := filepath.Join(project.Path, "plan.md")
	if err := os.WriteFile(doc, []byte("# The plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := e.Report(protocol.Report{Cell: id, Kind: "doc", File: doc}); err != nil {
		t.Fatalf("report: %v", err)
	}
	if err := e.OpenDoc(id); err != nil {
		t.Fatalf("open: %v", err)
	}
	cells := e.Snapshot().Projects[0].Cells
	if len(cells) != 2 || cells[1].Type != "md" {
		t.Fatalf("a reading cell should have joined the project: %+v", cells)
	}
	if len(cells[1].Lines) == 0 || !strings.Contains(cells[1].Lines[0], "plan.md") {
		t.Fatalf("the reading cell should be on the file: %q", cells[1].Lines)
	}
	if cells[0].Doc != "" {
		t.Fatalf("a file that was opened stops being announced: %q", cells[0].Doc)
	}
}

// TestReportedFileHasToBeAFile — the path came from outside and ends up on a
// command line.
func TestReportedFileHasToBeAFile(t *testing.T) {
	e, id, _, source := sessionWithFiles(t)

	for name, file := range map[string]string{
		"relative":    "main.go",
		"missing":     source + ".nope",
		"a directory": filepath.Dir(source),
		"control":     source + "\n--wait",
		"empty":       "",
	} {
		if err := e.Report(protocol.Report{Cell: id, Kind: "edit", File: file, Line: 3}); err == nil {
			t.Errorf("%s: should have been refused", name)
		}
	}
	if got := cellOf(e, id).Edited; got != "" {
		t.Fatalf("a refused report left a file behind: %q", got)
	}
}

// TestEditorOpensTheLastEditAtTheLine — the ide mod reports the edit, and the
// picker's file key asks for it: the IDE gets the project and the place.
func TestEditorOpensTheLastEditAtTheLine(t *testing.T) {
	e, id, _, source := sessionWithFiles(t)
	project := e.Snapshot().Projects[0]
	record := filepath.Join(t.TempDir(), "called")
	fake := filepath.Join(t.TempDir(), "fake-editor")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+record+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := e.OpenInEditor(project.ID, fake, "wsl", id); err == nil {
		t.Fatal("with no edit reported, the file key has nothing to open")
	}
	if err := e.Report(protocol.Report{Cell: id, Kind: "edit", File: source, Line: 42}); err != nil {
		t.Fatalf("report: %v", err)
	}
	if got := cellOf(e, id).Edited; got != "main.go:42" {
		t.Fatalf("the picker should name the file and the line: %q", got)
	}
	if err := e.OpenInEditor(project.ID, fake, "wsl", id); err != nil {
		t.Fatalf("open: %v", err)
	}
	called, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the IDE was not called: %v", err)
	}
	want := []string{project.Path, "--goto", source + ":42"}
	if got := strings.Fields(string(called)); !slices.Equal(got, want) {
		t.Fatalf("the IDE got %v, expected %v", got, want)
	}
}

// TestEditorArgumentsPerSide — the Windows IDE reaches the project through
// WSL, and a folder URI can't carry a line.
func TestEditorArgumentsPerSide(t *testing.T) {
	for name, c := range map[string]struct {
		distro, file string
		want         []string
	}{
		"linux, project":   {"", "", []string{"/p"}},
		"linux, file":      {"", "/p/a.go", []string{"/p", "--goto", "/p/a.go:7"}},
		"windows, project": {"Ubuntu", "", []string{"--folder-uri", "vscode-remote://wsl+Ubuntu/p"}},
		"windows, file":    {"Ubuntu", "/p/a.go", []string{"--remote", "wsl+Ubuntu", "/p", "--goto", "/p/a.go:7"}},
	} {
		if got := editorArguments("/p", c.distro, c.file, 7); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, expected %v", name, got, c.want)
		}
	}
}

// TestTheWindowsLauncherIsNotALinuxInstall — the `code` a Windows install
// drops on WSL's PATH lives on a Windows disk.
func TestTheWindowsLauncherIsNotALinuxInstall(t *testing.T) {
	for path, onWindows := range map[string]bool{
		"/mnt/c/Users/me/AppData/Local/Programs/Microsoft VS Code/bin/code": true,
		"/mnt/D/tools/cursor": true,
		"/usr/bin/code":       false,
		"/mnt/wsl/bin/code":   false,
		"/home/me/mnt/c/code": false,
	} {
		if windowsDrive.MatchString(path) != onWindows {
			t.Errorf("%s: on a Windows disk should be %v", path, onWindows)
		}
	}
}

// TestEditorIsLaunchedOnItsOwn — an IDE that stays up doesn't hold the call,
// and it runs apart from the engine's process group, so a signal meant for
// the engine doesn't reach it.
func TestEditorIsLaunchedOnItsOwn(t *testing.T) {
	e := testEngine(t)
	if _, err := e.Create(protocol.Create{Path: t.TempDir(), Type: "bash"}); err != nil {
		t.Fatalf("creating: %v", err)
	}
	record := filepath.Join(t.TempDir(), "pid")
	fake := filepath.Join(t.TempDir(), "fake-editor")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho $$ > "+record+"\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	if err := e.OpenInEditor(e.Snapshot().Projects[0].ID, fake, "wsl", ""); err != nil {
		t.Fatalf("open: %v", err)
	}
	if took := time.Since(started); took > 3*time.Second {
		t.Fatalf("a live IDE held the call for %s", took)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("the IDE was not called: %v", err)
	}
	var pid int
	for _, digit := range strings.TrimSpace(string(raw)) {
		pid = pid*10 + int(digit-'0')
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	// A session leader leads its process group too, and that's what the
	// standard library can ask about.
	group, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatalf("the IDE didn't stay up: %v", err)
	}
	if group != pid || group == syscall.Getpgrp() {
		t.Fatalf("the IDE should lead a group of its own: group %d, pid %d, ours %d", group, pid, syscall.Getpgrp())
	}
}
