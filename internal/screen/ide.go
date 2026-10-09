package screen

import (
	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// IDEPicker is the dialog ctrl+e opens: every IDE tesseract found on the
// machine, wsl and windows alike, one pick away from opening the project.
// While it's open, it owns the keyboard.
type IDEPicker struct {
	Project string
	Name    string
	// Cell is the focused cell and Edited the last file its agent changed,
	// as "path:line"; empty when it hasn't changed any.
	Cell     string
	Edited   string
	List     []protocol.IDE
	Selected int
	Error    string
	Loading  bool
	frame    int
}

// NewIDEPicker opens the picker already asking the engine what's out there.
func NewIDEPicker(projectID, name string, focused *protocol.Cell) *IDEPicker {
	picker := &IDEPicker{Project: projectID, Name: name, Loading: true}
	if focused != nil {
		picker.Cell, picker.Edited = focused.ID, focused.Edited
	}
	return picker
}

// Received gets the detected list from the engine.
func (p *IDEPicker) Received(ides protocol.IDEs) {
	p.Loading = false
	p.List, p.Error = ides.List, ides.Error
	p.Selected = min(max(p.Selected, 0), max(len(p.List)-1, 0))
}

// Spin advances one frame of the picker's animation — the loading spinner
// while it's still detecting.
func (p *IDEPicker) Spin() { p.frame = (p.frame + 1) % len(spinnerFrames) }

// Key handles a key for the picker. Returns the request to send when an IDE
// is chosen, and whether the picker stays open.
func (p *IDEPicker) Key(key string) (request *protocol.Editor, open bool) {
	action := keyboard.Lookup(keyboard.IDEPanel, key)
	switch action {
	case keyboard.Back:
		return nil, false
	case keyboard.PreviousIDE:
		p.Selected = max(p.Selected-1, 0)
	case keyboard.NextIDE:
		p.Selected++
		// While the machine is still being searched there's no list to stop
		// at: the arrows are kept, and the list clamps them when it lands.
		if !p.Loading {
			p.Selected = min(p.Selected, max(len(p.List)-1, 0))
		}
	case keyboard.ChooseIDE, keyboard.ChooseFile:
		request := protocol.Editor{Project: p.Project}
		if action == keyboard.ChooseFile {
			if p.Edited == "" {
				break
			}
			request.Cell = p.Cell
		}
		if p.Selected < len(p.List) {
			chosen := p.List[p.Selected]
			request.ID, request.Location = chosen.ID, chosen.Location
			return &request, false
		}
	}
	return nil, true
}

// Draw builds the picker's box.
func (p *IDEPicker) Draw(width int) []string {
	inner := min(width-8, 76)
	var body []string
	switch {
	case p.Loading:
		body = append(body, "  "+barColor.Render(spinnerFrames[p.frame]+" looking for IDEs…"))
	case p.Error != "":
		body = append(body, "  "+errorColor.Render(truncate(theme.Do(theme.Down).Glyph+" "+p.Error, inner-6)))
	case len(p.List) == 0:
		body = append(body, "  "+mutedColor.Render("No IDE found in WSL or on Windows."))
	}
	for i, ide := range p.List {
		row := pad(ide.Label, 20) + "(" + ide.Location + ")"
		if i == p.Selected {
			// The picked line is a raised row with a bar, not a caret — same
			// language as the Docker panel's service list, so the two
			// dialogs read as one family.
			body = append(body, selectedBar+selectedRowColor.Render(" "+pad(row, max(inner-5, 1))))
			continue
		}
		body = append(body, "  "+barColor.Render(pad(ide.Label, 20))+mutedColor.Render("("+ide.Location+")"))
	}
	body = append(body, "")
	for _, line := range keyboard.HelpLines(keyboard.IDEPanel) {
		if keyboard.Lookup(keyboard.IDEPanel, line.Keys) == keyboard.ChooseFile {
			// The file's own line only exists when there is a file, and it
			// names it: the key opens that and nothing else.
			if p.Edited == "" {
				continue
			}
			line.Help = truncate("opens "+p.Edited+", the agent's last edit", max(inner-26, 1))
		}
		body = append(body, "  "+keyColor.Render(pad(line.Keys, 22))+mutedColor.Render(line.Help))
	}
	return floatingBox("OPEN IN · "+p.Name, body, inner)
}
