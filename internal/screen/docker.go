package screen

import (
	"fmt"
	"strings"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// spinnerFrames is the drawing for work in progress. Without it, bringing up
// a stack looks like nothing happened until everything suddenly comes up.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// DockerPanel is the focused project's Docker panel, opened on top of the
// screen. While it's open, it owns the keyboard.
type DockerPanel struct {
	Project  string
	Name     string
	File     string
	List     []protocol.Service
	Selected int
	Error    string
	Loading  bool
	// Working describes what's running right now; empty means the panel is
	// idle, waiting for a key.
	Working string
	frame   int
}

// NewDockerPanel opens the panel already requesting the list.
func NewDockerPanel(projectID, name string) *DockerPanel {
	return &DockerPanel{Project: projectID, Name: name, Loading: true}
}

// Received gets the list of services from the engine. The answer to the
// request in flight is what ends the spin.
func (p *DockerPanel) Received(services protocol.Services) {
	p.Loading = false
	p.File, p.List, p.Error = services.File, services.List, services.Error
	p.Selected = min(max(p.Selected, 0), max(len(p.List)-1, 0))
	if services.Action != "list" {
		p.Working = ""
	}
}

// Started marks that an action is running, so the panel can say so while it
// waits.
func (p *DockerPanel) Started(action, service string) {
	target := "the whole stack"
	if service != "" {
		target = service
	}
	p.Working = actionDescription(action) + " " + target
	p.Error = ""
	p.frame = 0
}

// Spin advances one frame of the work-in-progress drawing.
func (p *DockerPanel) Spin() {
	p.frame = (p.frame + 1) % len(spinnerFrames)
}

// IsWorking says whether the panel is waiting for Docker to finish
// something.
func (p *DockerPanel) IsWorking() bool { return p.Working != "" }

func actionDescription(action string) string {
	switch action {
	case "up":
		return "bringing up"
	case "down":
		return "stopping"
	case "restart":
		return "restarting"
	case "rebuild":
		return "rebuilding"
	}
	return action
}

// Key handles a key for the panel. Returns the request to send to the
// engine, whether the action becomes a log cell, and whether the panel stays
// open.
func (p *DockerPanel) Key(key string) (request *protocol.Docker, open bool) {
	action := keyboard.Lookup(keyboard.DockerPanel, key)
	chosen := ""
	if p.Selected < len(p.List) {
		chosen = p.List[p.Selected].Name
	}

	switch action {
	case keyboard.Back:
		return nil, false
	case keyboard.PreviousService:
		p.Selected = max(p.Selected-1, 0)
	case keyboard.NextService:
		p.Selected = min(p.Selected+1, max(len(p.List)-1, 0))

	// On the chosen service.
	case keyboard.UpService:
		return p.request("up", chosen), true
	case keyboard.StopService:
		return p.request("down", chosen), true
	case keyboard.RestartService:
		return p.request("restart", chosen), true
	case keyboard.RebuildService:
		return p.request("rebuild", chosen), true
	case keyboard.LogAsCell:
		// The log becomes a cell in the grid, so the panel closes.
		return p.request("log", chosen), false
	case keyboard.ShellAsCell:
		// Same as the log: the shell becomes a cell, and the panel gives the
		// keyboard back to the grid.
		return p.request("shell", chosen), false

	// On the whole stack: the uppercase key is always the stack version of
	// the lowercase one.
	case keyboard.UpStack:
		return p.request("up", ""), true
	case keyboard.StopStack:
		return p.request("down", ""), true
	case keyboard.RestartStack:
		return p.request("restart", ""), true
	}
	return nil, true
}

func (p *DockerPanel) request(action, service string) *protocol.Docker {
	if action != "up" && action != "down" && action != "restart" && action != "rebuild" && action != "log" && action != "shell" {
		return nil
	}
	if service == "" && (action == "log" || action == "shell") {
		return nil
	}
	return &protocol.Docker{Project: p.Project, Action: action, Service: service}
}

// Draw builds the panel's box.
func (p *DockerPanel) Draw(width int) []string {
	inner := min(width-8, 92)
	// The name column follows the longest service name: a real stack name
	// overflows the original design's 14 and would get cut off.
	nameColumn := 14
	for _, service := range p.List {
		if n := widthOf(service.Name) + 2; n > nameColumn {
			nameColumn = min(n, 32)
		}
	}
	body := []string{
		"  " + borderColor.Render(pad("SERVICE", nameColumn)+pad("STATE", 16)+pad("PORT", 9)+pad("HEALTH", 12)+"UPTIME"),
	}
	switch {
	case p.Working != "":
		body = append(body,
			"  "+barColor.Render(spinnerFrames[p.frame]+" "+p.Working+"…"),
			"  "+mutedColor.Render(truncate("Docker takes its own time: pulling images, creating containers, waiting for health.", inner-6)))
	case p.Loading:
		body = append(body, "  "+mutedColor.Render("reading the stack…"))
	case p.Error != "":
		body = append(body, "  "+errorColor.Render(truncate(theme.Do(theme.Down).Glyph+" "+p.Error, inner-6)))
	case len(p.List) == 0:
		body = append(body, "  "+mutedColor.Render("Stack never started in this project. ")+hint("U brings it up."))
	}

	for i, service := range p.List {
		found := serviceMarker(service.State)
		dot, paint := found.symbol, found.tone()
		if i == p.Selected {
			// The picked line is a raised row with a bar on its left, not a
			// caret: a whole painted row is found by the eye without reading.
			row := pad(service.Name, nameColumn) + pad(dot+" "+service.State, 16) +
				pad(orDash(service.Port), 9) + pad(orDash(service.Health), 12) + orDash(service.Uptime)
			body = append(body, selectedBar+selectedRowColor.Render(" "+pad(row, max(inner-5, 1))))
			continue
		}
		body = append(body, "  "+
			barColor.Render(pad(service.Name, nameColumn))+
			paint.Render(pad(dot+" "+service.State, 16))+
			mutedColor.Render(pad(orDash(service.Port), 9))+
			mutedColor.Render(pad(orDash(service.Health), 12))+
			mutedColor.Render(orDash(service.Uptime)))
	}

	body = append(body, "")
	for _, line := range keyboard.HelpLines(keyboard.DockerPanel) {
		body = append(body, "  "+keyColor.Render(pad(line.Keys, 22))+mutedColor.Render(line.Help))
	}
	title := "DOCKER · " + p.Name
	if p.File != "" {
		title += " · " + fileName(p.File)
	}
	return floatingBox(title, body, inner)
}

// serviceMarker reads a service with the same alphabet as the cells, and as
// the Docker mod inside a cell: up is a live process producing, one that
// exited on an error is a dead process, anything else is stopped. No glyph
// of its own, no color of its own.
func serviceMarker(state string) marker {
	if strings.HasPrefix(state, "up") {
		return markerFor("working")
	}
	var exit int
	if _, err := fmt.Sscanf(state, "exited (%d)", &exit); err == nil {
		// 137 and 143 are SIGKILL and SIGTERM: what a stop leaves behind.
		if exit != 0 && exit != 137 && exit != 143 {
			return markerFor("down")
		}
	} else if state == "dead" {
		return markerFor("down")
	}
	return markerFor("stopped")
}

func orDash(text string) string {
	if text == "" {
		return "—"
	}
	return text
}

func fileName(path string) string {
	if cut := strings.LastIndex(path, "/"); cut >= 0 {
		return path[cut+1:]
	}
	return path
}
