package screen

import (
	"strconv"
	"strings"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/theme"
)

// visibleFolders is how many folders of the list fit under the field without
// the box eating the screen.
const visibleFolders = 6

// field is one line of the form: a label and what's been typed so far.
type field struct {
	label string
	value string
	hint  string
	// completes accepts tab to complete a file path.
	completes bool
	// browses turns the field into a folder browser: a live list under it,
	// walked with the arrows, that never needs a full path typed by hand.
	browses bool
}

func (c *field) write(text string) { c.value += text }

func (c *field) erase() {
	if c.value == "" {
		return
	}
	runes := []rune(c.value)
	c.value = string(runes[:len(runes)-1])
}

// Form is the only path to creation: it starts by asking for the project and
// ends by creating the cell. There is no separate "create project" — and no
// choosing what the session will be: it's born with the agent tabs already
// inside.
type Form struct {
	fields  []*field
	current int
	warning string
	// recents are the directories opened most recently, which is what the
	// folder list shows before anything is typed.
	recents []string
	// options is the folder list as it stands, count how many matched (which
	// can be more than travelled), and highlight the one picked.
	options   []string
	count     int
	highlight int
	// pending is what the form still needs from the engine, drained by the
	// screen on the next key.
	pending *protocol.Complete
}

// NewForm opens creation on the folder browser, starting from the focused
// project's directory — the common case is another cell right there. With no
// project focused it opens on the recent folders, and on home for whoever
// has no history yet, so there is always something to walk without typing.
func NewForm(recents []string, current string) *Form {
	f := &Form{recents: recents}
	if current == "" && len(recents) == 0 {
		current = strings.TrimSuffix(home(), "/") + "/"
	}
	f.fields = []*field{
		{label: "PROJECT", value: current, hint: "↑↓ picks · → enters · ← goes up · ↵ confirms", browses: true},
		{label: "NAME", hint: "(empty → automatic name)"},
		{label: "MD", hint: "(optional — opens a markdown file to read)", completes: true},
		{label: "PROMPT", hint: "(optional — comes up already working)"},
	}
	f.refresh()
	return f
}

// Key handles a key for the form. It returns the create request when the
// user confirms, and whether the form should stay open.
func (f *Form) Key(action keyboard.Action, text string, erased bool) (request *protocol.Create, open bool) {
	f.warning = ""
	current := f.fields[f.current]
	switch action {
	case keyboard.Cancel:
		return nil, false

	case keyboard.Confirm:
		if current.browses {
			// On the folder field ↵ settles the folder and walks on: only
			// from the other fields does it create.
			f.accept()
			f.current = 1
			return nil, true
		}
		path := strings.TrimSpace(f.fields[0].value)
		if path == "" {
			f.warning = "Project needs a path. Type one or pick a folder."
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

	case keyboard.Complete:
		if current.browses {
			f.enter()
			return nil, true
		}
		if current.completes {
			f.pending = &protocol.Complete{Path: f.completionPath()}
		}

	case keyboard.PreviousField:
		// The arrows walk the folder list first and only leave the field
		// once there's nowhere left to go — the list is the field.
		if current.browses && f.highlight > 0 {
			f.highlight--
			return nil, true
		}
		f.current = (f.current - 1 + len(f.fields)) % len(f.fields)
	case keyboard.NextField:
		if current.browses && f.highlight < len(f.options)-1 {
			f.highlight++
			return nil, true
		}
		f.current = (f.current + 1) % len(f.fields)

	case keyboard.PreviousOpt:
		if current.browses {
			f.up()
		}
	case keyboard.NextOpt:
		if current.browses {
			f.enter()
		}

	default:
		if erased {
			current.erase()
		} else {
			current.write(text)
		}
		if current.browses {
			f.refresh()
		}
	}
	return nil, true
}

// Pending is what the form is waiting on from the engine — the live folder
// list, or the markdown field's completion.
func (f *Form) Pending() (protocol.Complete, bool) {
	if f.pending == nil {
		return protocol.Complete{}, false
	}
	request := *f.pending
	f.pending = nil
	return request, true
}

// refresh rebuilds the folder list for whatever is in the field: nothing
// typed shows the folders opened recently, a loose fragment filters those,
// and anything with a slash in it goes to the engine for the real folders on
// disk.
func (f *Form) refresh() {
	f.highlight = 0
	value := strings.TrimSpace(f.fields[0].value)
	if strings.Contains(value, "/") {
		f.pending = &protocol.Complete{Path: value, DirsOnly: true, List: true}
		return
	}
	f.pending = nil
	f.options = filterRecents(f.recents, value)
	f.count = len(f.options)
}

// filterRecents keeps the recent folders whose path carries the fragment, so
// "cor" finds the project without anyone typing where it lives.
func filterRecents(recents []string, fragment string) []string {
	fragment = strings.ToLower(fragment)
	var found []string
	for _, path := range recents {
		if fragment == "" || strings.Contains(strings.ToLower(path), fragment) {
			found = append(found, path)
		}
	}
	return found
}

// accept takes the highlighted folder into the field. With nothing
// highlighted whatever was typed stands, which is how a path nobody has
// opened before still gets in.
func (f *Form) accept() {
	if f.highlight < len(f.options) {
		f.fields[0].value = strings.TrimSuffix(f.options[f.highlight], "/")
	}
}

// enter walks into the highlighted folder — the half of browsing that goes
// down.
func (f *Form) enter() {
	if f.highlight >= len(f.options) {
		return
	}
	f.fields[0].value = strings.TrimSuffix(f.options[f.highlight], "/") + "/"
	f.refresh()
}

// up climbs one folder — the half that goes back. From the recent list,
// where nothing is typed, it climbs from the folder under the pick: that's
// how you get from a project to whatever sits next to it.
func (f *Form) up() {
	value := strings.TrimSuffix(f.fields[0].value, "/")
	if value == "" && f.highlight < len(f.options) {
		value = strings.TrimSuffix(f.options[f.highlight], "/")
	}
	switch cut := strings.LastIndex(value, "/"); {
	case cut > 0:
		f.fields[0].value = value[:cut+1]
	case strings.HasPrefix(value, "/"):
		f.fields[0].value = "/"
	default:
		f.fields[0].value = ""
	}
	f.refresh()
}

// completionPath is what the markdown field asks the engine about: a bare
// name is looked up inside the project being created.
func (f *Form) completionPath() string {
	path := f.fields[f.current].value
	if !strings.Contains(path, "/") {
		return strings.TrimSuffix(f.fields[0].value, "/") + "/" + path
	}
	return path
}

// Completed receives the engine's answer about the typed path.
func (f *Form) Completed(response protocol.Completed) {
	current := f.fields[f.current]
	if current.browses {
		f.options, f.count = response.Options, response.Count
		f.highlight = min(f.highlight, max(len(f.options)-1, 0))
		return
	}
	if !current.completes {
		return
	}
	current.value = response.Path
	switch response.Count {
	case 0:
		f.warning = "No file matches that path. Erase a letter and tab again."
	case 1:
		f.warning = ""
	default:
		f.warning = plural(response.Count, "file matches", "files match") + ". Keep typing to narrow it."
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
		// One field has the keyboard at a time, and only that one shows a
		// caret: four carets on screen would be four owners.
		mark, cursor := "  ", ""
		if i == f.current {
			mark, cursor = selectedBar+" ", caret
		}
		body = append(body, mark+borderColor.Render(pad(c.label, 8))+c.value+cursor)
		body = append(body, hintLines(c.hint, inner)...)
		if c.browses && i == f.current {
			body = append(body, f.folders(inner)...)
		}
	}
	if f.warning != "" {
		body = append(body, "", "  "+warnColor.Render(f.warning))
	}
	return floatingBox("NEW", body, inner)
}

// folders draws the live list under the project field: the folders opened
// recently while nothing is typed, what's on disk once a path is.
func (f *Form) folders(inner int) []string {
	room := max(inner-6, 1)
	if len(f.options) == 0 {
		if strings.Contains(f.fields[0].value, "/") {
			return []string{"    " + mutedColor.Render("No folders in here. ") + hint("← goes up.")}
		}
		return []string{"    " + mutedColor.Render("No recent folders. Type a path.")}
	}

	// The window follows the pick, keeping it in the middle whenever the list
	// is longer than the room it has.
	first := max(min(f.highlight-visibleFolders/2, len(f.options)-visibleFolders), 0)
	last := min(first+visibleFolders, len(f.options))
	var lines []string
	for i := first; i < last; i++ {
		label := shortenHome(f.options[i])
		if i == f.highlight {
			lines = append(lines, "  "+selectedBar+selectedRowColor.Render(" "+pad(label, room)))
			continue
		}
		lines = append(lines, "    "+mutedColor.Render(truncate(label, room)))
	}
	if hidden := f.count - last; hidden > 0 {
		lines = append(lines, "    "+faintColor.Render("… "+strconv.Itoa(hidden)+" more"))
	}
	return lines
}

// shortenHome writes home as ~, so the list stays readable inside the box.
func shortenHome(path string) string {
	h := strings.TrimSuffix(home(), "/")
	if h == "" || !strings.HasPrefix(path, h+"/") {
		return path
	}
	return "~" + strings.TrimPrefix(path, h)
}

// caret is the cursor of the field being written. Phosphor, like every
// cursor the brand draws: it's where the keyboard is.
var caret = typingColor.Render("▏")

// The dialog's header is a raised band, like a cell's.
var (
	boxTitleColor = theme.WithBackground(titleColor, theme.BgRaised)
	boxRuleColor  = theme.WithBackground(focusedBorderColor, theme.BgRaised)
)

// floatingBox draws a box with a title, at the requested size. A dialog
// holds the keyboard while it's open, so its line is the live green; its
// body sits on the surface, like a cell's.
func floatingBox(title string, body []string, width int) []string {
	side := focusedBorderColor.Render("│")
	row := func(line string) string {
		return side + surfaceInk.Under(pad(line, width-2)) + side
	}
	lines := []string{focusedBorderColor.Render("┌") +
		boxTitleColor.Render(" "+title+" ") +
		boxRuleColor.Render(strings.Repeat("─", max(width-len([]rune(title))-4, 0))) +
		focusedBorderColor.Render("┐")}
	lines = append(lines, row(""))
	for _, line := range body {
		lines = append(lines, row(line))
	}
	lines = append(lines, row(""))
	lines = append(lines, focusedBorderColor.Render("└"+strings.Repeat("─", width-2)+"┘"))
	return lines
}

func plural(count int, one, many string) string {
	if count == 1 {
		return strconv.Itoa(count) + " " + one
	}
	return strconv.Itoa(count) + " " + many
}
