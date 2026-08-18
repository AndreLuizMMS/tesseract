package screen

import (
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/protocol"
)

func testPanel() *DockerPanel {
	panel := NewDockerPanel("p1", "doxar-api")
	panel.Received(protocol.Services{
		Project: "p1", File: "/home/dev/doxar-api/docker-compose.yml",
		List: []protocol.Service{
			{Name: "api", State: "up", Port: ":3000", Health: "healthy", Uptime: "2h14m"},
			{Name: "postgres", State: "up", Port: ":5432", Health: "healthy", Uptime: "2h14m"},
			{Name: "worker", State: "exited (1)"},
		},
	})
	return panel
}

// TestPanelSelectsWithoutRunning — the arrows select the service and don't
// run any action.
func TestPanelSelectsWithoutRunning(t *testing.T) {
	panel := testPanel()
	for _, key := range []string{"down", "down", "up"} {
		request, open := panel.Key(key)
		if request != nil {
			t.Fatalf("key %q ran %#v", key, request)
		}
		if !open {
			t.Fatalf("key %q closed the panel", key)
		}
	}
	if panel.Selected != 1 {
		t.Fatalf("the selected one is %d", panel.Selected)
	}
}

// TestPanelActsOnServiceAndStack — the uppercase key is always the whole
// stack's version of the matching lowercase one.
func TestPanelActsOnServiceAndStack(t *testing.T) {
	cases := []struct {
		key     string
		action  string
		service string
	}{
		{"u", "up", "api"},
		{"s", "down", "api"},
		{"r", "restart", "api"},
		{"b", "rebuild", "api"},
		{"U", "up", ""},
		{"S", "down", ""},
		{"R", "restart", ""},
	}
	for _, c := range cases {
		panel := testPanel()
		request, open := panel.Key(c.key)
		if request == nil {
			t.Fatalf("key %q asked for nothing", c.key)
		}
		if request.Action != c.action || request.Service != c.service {
			t.Fatalf("key %q requested %#v", c.key, request)
		}
		if !open {
			t.Fatalf("key %q shouldn't close the panel", c.key)
		}
	}
}

// TestPanelLogBecomesACell — requesting the log closes the panel and a cell
// is born.
func TestPanelLogBecomesACell(t *testing.T) {
	panel := testPanel()
	panel.Key("down") // postgres
	request, open := panel.Key("l")
	if request == nil || request.Action != "log" || request.Service != "postgres" {
		t.Fatalf("log request came out %#v", request)
	}
	if open {
		t.Fatal("the log becomes a cell in the grid, so the panel closes")
	}
}

// TestPanelEscGivesBackTheKeyboard.
func TestPanelEscGivesBackTheKeyboard(t *testing.T) {
	panel := testPanel()
	request, open := panel.Key("esc")
	if request != nil || open {
		t.Fatal("esc only closes")
	}
}

// TestPanelIgnoresKeysFromOutside — while the panel is open, it owns the
// keyboard: a key that isn't its does nothing.
func TestPanelIgnoresKeysFromOutside(t *testing.T) {
	panel := testPanel()
	for _, key := range []string{"n", "D", "q", "v", "p", "/", "?"} {
		request, open := panel.Key(key)
		if request != nil {
			t.Errorf("key %q isn't the panel's, but it asked for %#v", key, request)
		}
		if !open {
			t.Errorf("key %q closed the panel", key)
		}
	}
}

// TestPanelDrawsTheStackState.
func TestPanelDrawsTheStackState(t *testing.T) {
	drawing := noStyle(strings.Join(testPanel().Draw(90), "\n"))
	for _, piece := range []string{"DOCKER · doxar-api", "docker-compose.yml", "api", ":3000", "healthy", "2h14m", "exited (1)"} {
		if !strings.Contains(drawing, piece) {
			t.Errorf("%q is missing from the panel:\n%s", piece, drawing)
		}
	}
	for _, forbidden := range []string{"down -v", "apagar", "volume"} {
		if strings.Contains(drawing, forbidden) {
			t.Errorf("the panel can't offer %q", forbidden)
		}
	}
}

// TestPanelWithNoStackDoesNotBreak.
func TestPanelWithNoStackDoesNotBreak(t *testing.T) {
	panel := NewDockerPanel("p1", "novo")
	panel.Received(protocol.Services{Project: "p1"})
	drawing := noStyle(strings.Join(panel.Draw(90), "\n"))
	if !strings.Contains(drawing, "never come up") {
		t.Errorf("an empty stack needs to say so:\n%s", drawing)
	}
	request, _ := panel.Key("l")
	if request != nil {
		t.Error("with no service selected there's no log to open")
	}
}

// TestPanelShowsItIsWorking — bringing up the stack takes a while, and the
// panel needs to say it started instead of looking like the key did
// nothing.
func TestPanelShowsItIsWorking(t *testing.T) {
	panel := testPanel()

	request, open := panel.Key("U")
	if request == nil || !open {
		t.Fatal("U brings up the stack and keeps the panel open")
	}
	panel.Started(request.Action, request.Service)
	if !panel.IsWorking() {
		t.Fatal("the panel should be marked as working")
	}

	drawing := noStyle(strings.Join(panel.Draw(90), "\n"))
	if !strings.Contains(drawing, "bringing up the whole stack") {
		t.Errorf("the panel should say what it's doing:\n%s", drawing)
	}
	if !strings.Contains(drawing, "Docker takes its own time") {
		t.Errorf("the panel should warn that Docker takes a while:\n%s", drawing)
	}

	// The work-in-progress drawing advances every frame.
	first := noStyle(strings.Join(panel.Draw(90), "\n"))
	panel.Spin()
	if second := noStyle(strings.Join(panel.Draw(90), "\n")); second == first {
		t.Error("the work-in-progress drawing should move")
	}

	// A check midway through doesn't end the work.
	panel.Received(protocol.Services{Project: "p1", Action: "list", List: []protocol.Service{
		{Name: "api", State: "up"},
	}})
	if !panel.IsWorking() {
		t.Error("a midway check doesn't end the work")
	}

	// The action's own answer ends it.
	panel.Received(protocol.Services{Project: "p1", Action: "up", List: []protocol.Service{
		{Name: "api", State: "up"},
	}})
	if panel.IsWorking() {
		t.Error("the action's answer should end the work")
	}
}

// TestPanelForOneServiceNamesIt — working on a single service names it.
func TestPanelForOneServiceNamesIt(t *testing.T) {
	panel := testPanel()
	request, _ := panel.Key("r")
	panel.Started(request.Action, request.Service)
	drawing := noStyle(strings.Join(panel.Draw(90), "\n"))
	if !strings.Contains(drawing, "restarting api") {
		t.Errorf("the panel should say the service and the action:\n%s", drawing)
	}
}
