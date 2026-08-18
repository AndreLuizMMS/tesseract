package screen

import (
	"strconv"
	"strings"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// field is one line of the form: a label and what's been typed so far.
type field struct {
	label string
	value string
	hint  string
	// options filled in turns the field into a picker instead of free text.
	options   []string
	selected  int
	completes bool // accepts tab to complete a path
	dirOnly   bool
}

func (c *field) write(text string) {
	if len(c.options) > 0 {
		return
	}
	c.value += text
}

func (c *field) erase() {
	if len(c.options) > 0 || c.value == "" {
		return
	}
	runes := []rune(c.value)
	c.value = string(runes[:len(runes)-1])
}

func (c *field) move(step int) {
	if len(c.options) == 0 {
		return
	}
	c.selected = (c.selected + step + len(c.options)) % len(c.options)
}

func (c *field) chosen() string {
	if len(c.options) == 0 {
		return c.value
	}
	return c.options[c.selected]
}

// Form is the only path to creation: it starts by asking for the project and
// ends by creating the cell. There is no separate "create project" — and no
// choosing what the session will be: it's born with the agent tabs already
// inside.
type Form struct {
	fields  []*field
	current int
	warning string
}

// NewForm opens creation with the home path pre-filled, since that's where
// every project starts from.
func NewForm(_ []protocol.CellType, _ string) *Form {
	f := &Form{}
	f.fields = []*field{
		{label: "PROJECT", value: homeWithSlash(), hint: "tab completes · a new path creates a project", completes: true, dirOnly: true},
		{label: "NAME", hint: "(empty → automatic name)"},
		{label: "MD", hint: "(optional — a markdown file becomes a reading cell)", completes: true},
		{label: "PROMPT", hint: "(optional — comes up already working)"},
	}
	return f
}

// homeWithSlash is the account's home directory, ending in a slash so tab
// completion starts from inside it.
func homeWithSlash() string {
	h := home()
	if h == "" {
		return ""
	}
	return strings.TrimSuffix(h, "/") + "/"
}

// Key handles a key for the form. It returns the create request when the
// user confirms, and whether the form should stay open.
func (f *Form) Key(action keyboard.Action, text string, erased bool) (request *protocol.Create, open bool) {
	f.warning = ""
	switch action {
	case keyboard.Cancel:
		return nil, false
	case keyboard.Confirm:
		path := strings.TrimSpace(f.fields[0].value)
		if path == "" {
			f.warning = "the project needs a path"
			return nil, true
		}
		// The type isn't asked: the engine decides that, from what was
		// filled in. Without a markdown file, a session is born with the
		// agent tabs.
		create := &protocol.Create{
			Path:   path,
			Name:   strings.TrimSpace(f.valueOf("NAME")),
			Target: strings.TrimSpace(f.valueOf("MD")),
			Prompt: f.valueOf("PROMPT"),
		}
		return create, false
	case keyboard.PreviousField:
		f.current = (f.current - 1 + len(f.fields)) % len(f.fields)
	case keyboard.NextField:
		f.current = (f.current + 1) % len(f.fields)
	case keyboard.PreviousOpt:
		f.fields[f.current].move(-1)
	case keyboard.NextOpt:
		f.fields[f.current].move(1)
	default:
		if erased {
			f.fields[f.current].erase()
			return nil, true
		}
		f.fields[f.current].write(text)
	}
	return nil, true
}

// WantsComplete says whether the complete key has anything to do on the
// current field.
func (f *Form) WantsComplete(action keyboard.Action) (protocol.Complete, bool) {
	if action != keyboard.Complete {
		return protocol.Complete{}, false
	}
	current := f.fields[f.current]
	if !current.completes {
		return protocol.Complete{}, false
	}
	path := current.value
	if !current.dirOnly && !strings.Contains(path, "/") {
		// The file is looked up inside the project being created.
		path = strings.TrimSuffix(f.fields[0].value, "/") + "/" + path
	}
	return protocol.Complete{Path: path, DirsOnly: current.dirOnly}, true
}

// Completed receives the engine's answer about the typed path.
func (f *Form) Completed(response protocol.Completed) {
	current := f.fields[f.current]
	if !current.completes {
		return
	}
	current.value = response.Path
	switch response.Count {
	case 0:
		f.warning = "nothing matches that"
	case 1:
		f.warning = ""
	default:
		if current.dirOnly {
			f.warning = plural(response.Count, "folder matches", "folders match")
			return
		}
		f.warning = plural(response.Count, "file matches", "files match")
	}
}

func (f *Form) valueOf(label string) string {
	if label == "" {
		return ""
	}
	for _, c := range f.fields {
		if c.label == label {
			return c.value
		}
	}
	return ""
}

// Draw builds the form's box.
func (f *Form) Draw(width int) []string {
	inner := min(width-8, 60)
	var body []string
	for i, c := range f.fields {
		mark := "  "
		if i == f.current {
			mark = "▸ "
		}
		value := c.value + "▏"
		if len(c.options) > 0 {
			var options []string
			for j, option := range c.options {
				if j == c.selected {
					options = append(options, badgeColor.Render(" "+option+" "))
					continue
				}
				options = append(options, faintColor.Render(" "+option+" "))
			}
			value = strings.Join(options, " ")
		}
		body = append(body, mark+titleColor.Render(pad(c.label, 8))+value)
		if c.hint != "" {
			body = append(body, "  "+strings.Repeat(" ", 8)+faintColor.Render(c.hint))
		}
	}
	if f.warning != "" {
		body = append(body, "", "  "+scrollColor.Render(f.warning))
	}
	body = append(body, "", "  "+faintColor.Render("↵ create        esc cancel"))
	return floatingBox("NEW", body, inner)
}

// floatingBox draws a box with a title, at the requested size.
func floatingBox(title string, body []string, width int) []string {
	lines := []string{focusedBorderColor.Render("┌ ") + titleColor.Render(title) + focusedBorderColor.Render(" "+strings.Repeat("─", max(width-len([]rune(title))-4, 0))+"┐")}
	lines = append(lines, focusedBorderColor.Render("│")+pad("", width-2)+focusedBorderColor.Render("│"))
	for _, line := range body {
		lines = append(lines, focusedBorderColor.Render("│")+pad(line, width-2)+focusedBorderColor.Render("│"))
	}
	lines = append(lines, focusedBorderColor.Render("│")+pad("", width-2)+focusedBorderColor.Render("│"))
	lines = append(lines, focusedBorderColor.Render("└"+strings.Repeat("─", width-2)+"┘"))
	return lines
}

func plural(count int, one, many string) string {
	if count == 1 {
		return strconv.Itoa(count) + " " + one
	}
	return strconv.Itoa(count) + " " + many
}
