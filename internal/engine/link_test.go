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

// forgetReportedQuota discards what a mod reported, so the next test reads
// the statusline file again.
func forgetReportedQuota(t *testing.T) {
	t.Helper()
	clear := func() {
		reportedQuota.Lock()
		reportedQuota.at = time.Time{}
		reportedQuota.Unlock()
	}
	clear()
	t.Cleanup(clear)
}

// linkedCell brings up an engine with one Claude cell whose agent is fake,
// and attaches a mod that keeps whatever is delivered to it.
func linkedCell(t *testing.T) (e *Engine, id string, delivered *[]string, detach func()) {
	t.Helper()
	dir := t.TempDir()
	program := agentThatLogsWhatItReceives(t, dir, filepath.Join(t.TempDir(), "received.txt"))
	e = New(t.TempDir(), configWithFakeAgent(program))
	if err := e.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(e.Shutdown)
	id, err := e.Create(protocol.Create{Path: dir, Type: "claude", Name: "auth"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	delivered = &[]string{}
	detach, err = e.Link(id, "", func(delivery cell.Delivery) error {
		*delivered = append(*delivered, delivery.Text)
		// The mod's receipt comes back the way every report does.
		return e.Report(protocol.Report{Cell: id, Kind: "delivered", Delivery: delivery.ID})
	})
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	return e, id, delivered, detach
}

func stateOf(e *Engine, id string) string {
	for _, p := range e.Snapshot().Projects {
		for _, c := range p.Cells {
			if c.ID == id {
				return c.State
			}
		}
	}
	return ""
}

// TestReportDrivesTheCell is the whole point of the link: what the mod says
// becomes the cell's state, its conversation and the body of the notice.
func TestReportDrivesTheCell(t *testing.T) {
	e, id, delivered, detach := linkedCell(t)

	report := protocol.Report{Cell: id, Kind: "state", State: "approve", Detail: "Bash: pnpm\nmigrate", At: 1}
	if err := e.Report(report); err != nil {
		t.Fatalf("report: %v", err)
	}
	if state := stateOf(e, id); state != string(cell.Approve) {
		t.Fatalf("the cell should be waiting for approval, got %q", state)
	}
	e.mu.Lock()
	detail := e.projects[0].cells[0].takeDetail()
	again := e.projects[0].cells[0].takeDetail()
	e.mu.Unlock()
	if detail != "Bash: pnpm migrate" {
		t.Fatalf("the notice detail came out %q", detail)
	}
	if again != "" {
		t.Fatalf("the detail should be used once, came back %q", again)
	}

	const conversation = "0b0e7e0c-58a5-4b6e-9f0c-0f6f3c1d2e3f"
	if err := e.Report(protocol.Report{Cell: id, Kind: "session", Session: conversation}); err != nil {
		t.Fatalf("report session: %v", err)
	}
	saved := func() string {
		state, _ := Load(StatePath(e.stateDir))
		if len(state.Projects) == 0 || len(state.Projects[0].Cells) == 0 {
			return ""
		}
		return state.Projects[0].Cells[0].Conversations["claude"]
	}
	if !waitFor(t, 2*time.Second, func() bool { return saved() == conversation }) {
		t.Fatalf("the new conversation didn't reach the saved state: %q", saved())
	}

	if err := e.Prompt(id, "cover the mobile menu"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if !slices.Equal(*delivered, []string{"cover the mobile menu"}) {
		t.Fatalf("the prompt should have gone through the mod: %v", *delivered)
	}

	// With the mod gone the screen is in charge again, and a report that
	// still arrives changes nothing.
	detach()
	if err := e.Report(protocol.Report{Cell: id, Kind: "state", State: "answered", At: 2}); err != nil {
		t.Fatalf("report after detach: %v", err)
	}
	if state := stateOf(e, id); state == string(cell.Replied) {
		t.Fatal("a report with no mod linked changed the cell")
	}
}

// TestReportRefusesWhatItCannotTrust — everything in a report came from
// outside the engine.
func TestReportRefusesWhatItCannotTrust(t *testing.T) {
	e, id, _, _ := linkedCell(t)
	forgetReportedQuota(t)

	for name, report := range map[string]protocol.Report{
		"unknown cell":         {Cell: "nope", Kind: "state", State: "working"},
		"unknown kind":         {Cell: id, Kind: "shutdown"},
		"unknown state":        {Cell: id, Kind: "state", State: "exploded"},
		"session as a path":    {Cell: id, Kind: "session", Session: "../../etc/passwd"},
		"empty session":        {Cell: id, Kind: "session"},
		"negative quota":       {Cell: id, Kind: "quota", Percent: -3},
		"quota with bad reset": {Cell: id, Kind: "quota", Percent: 10, ResetsAt: "tomorrow"},
	} {
		if err := e.Report(report); err == nil {
			t.Errorf("%s: the report should have been refused", name)
		}
	}
	if ReadQuota() != nil && reportedQuotaAt(time.Now()) != nil {
		t.Fatal("a refused quota report was kept")
	}

	bashDir := t.TempDir()
	bash, err := e.Create(protocol.Create{Path: bashDir, Type: "bash"})
	if err != nil {
		t.Fatalf("create bash: %v", err)
	}
	if err := e.Report(protocol.Report{Cell: bash, Kind: "state", State: "approve"}); err == nil {
		t.Error("a shell has no agent to report for")
	}
	if _, err := e.Link(bash, "", func(cell.Delivery) error { return nil }); err == nil {
		t.Error("a shell took a link")
	}
}

// TestReportedQuotaBeatsTheFile — the mod's number comes first, and the
// statusline file is what's left when no mod has spoken.
func TestReportedQuotaBeatsTheFile(t *testing.T) {
	e, id, _, _ := linkedCell(t)
	prepareQuota(t, "tesseract-quota.json", 12, time.Now().Add(time.Hour), 0)
	forgetReportedQuota(t)

	if quota := ReadQuota(); quota == nil || quota.Percent != 12 {
		t.Fatalf("with no report the file should be read, got %+v", quota)
	}

	resets := time.Now().Add(2*time.Hour + 47*time.Minute + 30*time.Second)
	report := protocol.Report{Cell: id, Kind: "quota", Percent: 58.6, ResetsAt: resets.Format(time.RFC3339)}
	if err := e.Report(report); err != nil {
		t.Fatalf("report quota: %v", err)
	}
	quota := ReadQuota()
	if quota == nil || quota.Percent != 59 || quota.Rollover != "2:47" {
		t.Fatalf("the mod's number should win, got %+v", quota)
	}

	reportedQuota.Lock()
	reportedQuota.at = time.Now().Add(-quotaStaleAfter - time.Minute)
	reportedQuota.Unlock()
	forgetQuota(t)
	if quota := ReadQuota(); quota == nil || quota.Percent != 12 {
		t.Fatalf("a stale report should give way to the file, got %+v", quota)
	}
}

// TestCleanDetailFitsANotice — a command with line breaks and escape codes
// becomes one short line.
func TestCleanDetailFitsANotice(t *testing.T) {
	if got := cleanDetail("  Bash:\tpnpm\x1b[31m \n migrate\x00 "); got != "Bash: pnpm [31m migrate" {
		t.Fatalf("came out %q", got)
	}
	long := cleanDetail(strings.Repeat("ç", 500))
	if size := len([]rune(long)); size != detailLimit || !strings.HasSuffix(long, "…") {
		t.Fatalf("a long detail should be cut at %d, came out with %d", detailLimit, size)
	}
	if withDetail("auth · cortz responded", "") != "auth · cortz responded" {
		t.Fatal("a notice with no detail should stay as it was")
	}
}

// TestModsComeFromWhereTheyAre — every tesseract-* folder with a manifest is
// loaded, whatever else sits next to it is ignored, and a missing folder
// just means no mod.
func TestModsComeFromWhereTheyAre(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TESSERACT_MODS", root)
	for _, name := range []string{"tesseract-link", "tesseract-skin"} {
		manifest := filepath.Join(root, name, ".claude-plugin")
		if err := os.MkdirAll(manifest, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(filepath.Join(manifest, "plugin.json"), []byte("{}"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	// A folder that was planned and never written, and one that isn't ours.
	for _, name := range []string{"tesseract-docker", "something-else/.claude-plugin"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	want := []string{filepath.Join(root, "tesseract-link"), filepath.Join(root, "tesseract-skin")}
	if got := ModDirs(); !slices.Equal(got, want) {
		t.Fatalf("mods found: %v", got)
	}

	e := New(t.TempDir(), DefaultConfig())
	args := e.profiles()["claude"].Args
	wantArgs := []string{"--plugin-dir", want[0], "--plugin-dir", want[1], "--settings", `{"theme":"dark-ansi"}`}
	if !slices.Equal(args, wantArgs) {
		t.Fatalf("claude comes up with %v", args)
	}
	if cursor := e.profiles()["cursor"].Args; len(cursor) != 0 {
		t.Fatalf("the mods are Claude Code's, cursor got %v", cursor)
	}

	config := DefaultConfig()
	config.Mods, config.ClaudeTheme = false, ""
	config.Agents["claude"] = AgentProfile{Args: []string{"--verbose"}}
	if args := New(t.TempDir(), config).profiles()["claude"].Args; !slices.Equal(args, []string{"--verbose"}) {
		t.Fatalf("with both turned off only the user's args should remain: %v", args)
	}

	config = DefaultConfig()
	config.Mods = false
	config.Agents["claude"] = AgentProfile{Args: []string{"--settings", "mine.json"}}
	if args := New(t.TempDir(), config).profiles()["claude"].Args; !slices.Equal(args, []string{"--settings", "mine.json"}) {
		t.Fatalf("the user's own --settings should be the only one: %v", args)
	}

	t.Setenv("TESSERACT_MODS", filepath.Join(root, "missing"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "missing"))
	if got := ModDirs(); len(got) != 0 {
		t.Fatalf("no folder, no mod — got %v", got)
	}
}
