package screen

import (
	"slices"
	"strings"

	"github.com/andreluiz/tesseract/internal/docker"
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
		// The log becomes a cell in the grid, so the panel closes. No service
		// picked is no log: the whole stack's is L, never a fallback.
		if chosen == "" {
			return nil, false
		}
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
	case keyboard.LogStackAsCell:
		return p.request("log", ""), false
	}
	return nil, true
}

func (p *DockerPanel) request(action, service string) *protocol.Docker {
	if action != "up" && action != "down" && action != "restart" && action != "rebuild" && action != "log" && action != "shell" {
		return nil
	}
	if service == "" && action == "shell" {
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
	// A column every service leaves empty says nothing: it's left out, and
	// comes back with the first service that has something to put in it.
	columns := []struct {
		title string
		width int
		value func(protocol.Service) string
	}{
		{"PORT", 9, func(s protocol.Service) string { return s.Port }},
		{"HEALTH", 12, func(s protocol.Service) string { return s.Health }},
		{"UPTIME", 0, func(s protocol.Service) string { return s.Uptime }},
	}
	shown := columns[:0]
	for _, column := range columns {
		if slices.ContainsFunc(p.List, func(s protocol.Service) bool { return column.value(s) != "" }) {
			shown = append(shown, column)
		}
	}
	// rest is a service's line past its name and state, the last column
	// left unpadded.
	rest := func(text func(title string, value func(protocol.Service) string) string) string {
		var line strings.Builder
		for i, column := range shown {
			cell := text(column.title, column.value)
			if i < len(shown)-1 {
				cell = pad(cell, column.width)
			}
			line.WriteString(cell)
		}
		return line.String()
	}

	body := []string{
		"  " + faintColor.Render(pad("SERVICE", nameColumn)+pad("STATE", 16)+
			rest(func(title string, _ func(protocol.Service) string) string { return title })),
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
		others := rest(func(_ string, value func(protocol.Service) string) string { return orDash(value(service)) })
		if i == p.Selected {
			// The picked line is a raised row with a bar on its left, not a
			// caret: a whole painted row is found by the eye without reading.
			row := pad(service.Name, nameColumn) + pad(dot+" "+service.State, 16) + others
			body = append(body, selectedBar+selectedRowColor.Render(" "+pad(row, max(inner-5, 1))))
			continue
		}
		body = append(body, "  "+
			barColor.Render(pad(service.Name, nameColumn))+
			paint.Render(pad(dot+" "+service.State, 16))+
			mutedColor.Render(others))
	}

	// The keys, short and once: what each one does at length is in the help.
	body = append(append(body, ""), dialogKeys(keyboard.DockerPanel, inner-4)...)
	title := "DOCKER · " + p.Name
	if p.File != "" {
		title += " · " + fileName(p.File)
	}
	return floatingBox(title, body, inner)
}

// serviceMarker reads a service with the same alphabet as the cells: up is a
// live process producing, one that fell is a dead process, anything else is
// stopped. No glyph of its own, no color of its own.
func serviceMarker(state string) marker {
	switch {
	case strings.HasPrefix(state, "up"):
		return markerFor("working")
	case docker.Fell(state):
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
