package screen

import (
	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// IDEPicker is the dialog ctrl+e opens: every IDE tesseract found on the
// machine, wsl and windows alike, one pick away from opening the project.
// While it's open, it owns the keyboard.
type IDEPicker struct {
	Project  string
	Name     string
	List     []protocol.IDE
	Selected int
	Error    string
	Loading  bool
	frame    int
}

// NewIDEPicker opens the picker already asking the engine what's out there.
func NewIDEPicker(projectID, name string) *IDEPicker {
	return &IDEPicker{Project: projectID, Name: name, Loading: true}
}

// Received gets the detected list from the engine.
func (p *IDEPicker) Received(ides protocol.IDEs) {
	p.Loading = false
	p.List, p.Error = ides.List, ides.Error
	p.Selected = min(max(p.Selected, 0), max(len(p.List)-1, 0))
}

// Spin advances one frame of the picker's animation — the loading spinner
// while it's still detecting, the pulsing selection once the list is in.
func (p *IDEPicker) Spin() { p.frame = (p.frame + 1) % len(spinnerFrames) }

// Key handles a key for the picker. Returns the request to send when an IDE
// is chosen, and whether the picker stays open.
func (p *IDEPicker) Key(key string) (request *protocol.Editor, open bool) {
	switch keyboard.Lookup(keyboard.IDEPanel, key) {
	case keyboard.Back:
		return nil, false
	case keyboard.PreviousIDE:
		p.Selected = max(p.Selected-1, 0)
	case keyboard.NextIDE:
		p.Selected = min(p.Selected+1, max(len(p.List)-1, 0))
	case keyboard.ChooseIDE:
		if p.Selected < len(p.List) {
			chosen := p.List[p.Selected]
			return &protocol.Editor{Project: p.Project, ID: chosen.ID, Location: chosen.Location}, false
		}
	}
	return nil, true
}

// Draw builds the picker's box.
func (p *IDEPicker) Draw(width int) []string {
	inner := min(width-8, 60)
	var body []string
	switch {
	case p.Loading:
		body = append(body, "  "+scrollColor.Render(spinnerFrames[p.frame]+" looking for IDEs…"))
	case p.Error != "":
		body = append(body, "  "+errorColor.Render(truncate(p.Error, inner-6)))
	case len(p.List) == 0:
		body = append(body, "  "+faintColor.Render("no IDE found, inside WSL or on the Windows side"))
	}
	for i, ide := range p.List {
		row := pad(ide.Label, 20) + "(" + ide.Location + ")"
		if i == p.Selected {
			// The picked line is a lit bar, not a caret — same language as
			// the Docker panel's service list, so the two dialogs read as
			// one family.
			body = append(body, pulseColor(p.frame).Render("▌")+
				selectedRowColor.Render(" "+pad(row, max(inner-5, 1))))
			continue
		}
		body = append(body, "  "+barColor.Render(pad(ide.Label, 20))+faintColor.Render("("+ide.Location+")"))
	}
	body = append(body, "")
	for _, line := range keyboard.HelpLines(keyboard.IDEPanel) {
		body = append(body, "  "+barColor.Render(pad(line.Keys, 22))+faintColor.Render(line.Help))
	}
	return floatingBox("OPEN IN · "+p.Name, body, inner)
}
