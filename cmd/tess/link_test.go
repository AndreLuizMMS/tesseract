package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// savedCell reads the first cell the engine wrote to its desired state.
func savedCell(t *testing.T, home string) (id string, conversations map[string]string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(home, "state", "tesseract", "state.json"))
	if err != nil {
		return "", nil
	}
	var state struct {
		Projects []struct {
			Cells []struct {
				ID            string            `json:"id"`
				Conversations map[string]string `json:"conversations"`
			} `json:"cells"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(content, &state); err != nil || len(state.Projects) == 0 || len(state.Projects[0].Cells) == 0 {
		return "", nil
	}
	return state.Projects[0].Cells[0].ID, state.Projects[0].Cells[0].Conversations
}

// TestModReportsAndReceivesThroughTheBinary is the mod's whole path with the
// real binary on both ends: `ts link` holds the line, `ts report` speaks for
// the cell it runs in, and the engine answers each one.
func TestModReportsAndReceivesThroughTheBinary(t *testing.T) {
	home := testHome(t)
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if out, code := run(t, home, "new", project, "claude"); code != 0 {
		t.Fatalf("new exited with %d: %s", code, out)
	}
	id, _ := savedCell(t, home)
	if id == "" {
		t.Fatal("the engine saved no cell")
	}
	asCell := func(args ...string) (string, int) {
		t.Helper()
		cmd := command(t, home, args...)
		cmd.Env = append(cmd.Env, "TESSERACT_CELL_ID="+id)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), 1
		}
		return string(out), 0
	}

	// Outside a cell there's nobody to speak for.
	if out, code := run(t, home, "report", `{"kind":"state","state":"working"}`); code == 0 {
		t.Fatalf("a report from outside a cell was accepted: %s", out)
	}

	link := command(t, home, "link")
	link.Env = append(link.Env, "TESSERACT_CELL_ID="+id)
	if err := link.Start(); err != nil {
		t.Fatalf("link: %v", err)
	}
	defer link.Process.Kill()

	// The state only counts once the link is up, so the report is retried
	// until the engine shows it.
	approved := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && !approved; {
		body := `{"kind":"state","state":"approve","detail":"Bash: pnpm migrate","at":` + strconv.FormatInt(time.Now().UnixMilli(), 10) + `}`
		if out, code := asCell("report", body); code != 0 {
			t.Fatalf("report refused: %s", out)
		}
		status, _ := run(t, home, "status")
		approved = strings.Contains(status, "⏵")
	}
	if !approved {
		t.Fatal("the cell never showed the approval the mod reported")
	}

	const conversation = "0b0e7e0c-58a5-4b6e-9f0c-0f6f3c1d2e3f"
	if out, code := asCell("report", `{"kind":"session","session":"`+conversation+`"}`); code != 0 {
		t.Fatalf("session report refused: %s", out)
	}
	kept := ""
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline) && kept != conversation; {
		_, conversations := savedCell(t, home)
		kept = conversations["claude"]
		time.Sleep(50 * time.Millisecond)
	}
	if kept != conversation {
		t.Fatalf("the engine kept %q as the conversation", kept)
	}

	for name, body := range map[string]string{
		"a path as session": `{"kind":"session","session":"../../etc/passwd"}`,
		"an unknown field":  `{"kind":"state","state":"working","cell":"other","shell":"rm"}`,
		"not json":          `working`,
	} {
		if out, code := asCell("report", body); code == 0 {
			t.Errorf("%s was accepted: %s", name, out)
		}
	}

	// A Claude Code background session inherits the cell's environment but
	// isn't what the cell shows: it can neither rename the cell's
	// conversation after itself nor hold the line prompts go through.
	for _, args := range [][]string{
		{"report", `{"kind":"session","session":"73f5ad9e-58a5-4b6e-9f0c-0f6f3c1d2e3f"}`},
		{"link"},
	} {
		background := command(t, home, args...)
		background.Env = append(background.Env, "TESSERACT_CELL_ID="+id, "CLAUDE_CODE_SESSION_KIND=bg")
		if out, err := background.CombinedOutput(); err == nil {
			t.Errorf("a background session's %s was accepted: %s", args[0], out)
		}
	}
	if _, conversations := savedCell(t, home); conversations["claude"] != conversation {
		t.Fatalf("a background session changed the cell's conversation to %q", conversations["claude"])
	}

	// The link going away hands the cell back to the screen reading.
	link.Process.Kill()
	_ = link.Wait()
}
