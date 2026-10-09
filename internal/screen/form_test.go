package screen

import (
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
)

func testForm() *Form {
	return NewForm(nil, "")
}

// browsing is a form already showing a folder list, as it looks after the
// engine answered.
func browsing(options ...string) *Form {
	f := testForm()
	f.options, f.count = options, len(options)
	return f
}

func typeText(f *Form, text string) {
	for _, letter := range text {
		f.Key(keyboard.None, string(letter), false)
	}
}

func labels(f *Form) []string {
	var list []string
	for _, c := range f.fields {
		list = append(list, c.label)
	}
	return list
}

// TestFormStartsAtHomeWithoutHistory — with nothing opened before there is
// still somewhere to walk from.
func TestFormStartsAtHomeWithoutHistory(t *testing.T) {
	f := testForm()
	value := f.fields[0].value
	if !strings.HasPrefix(value, home()) || !strings.HasSuffix(value, "/") {
		t.Fatalf("the field should start inside home, got %q", value)
	}
	request, waiting := f.Pending()
	if !waiting || !request.List || !request.DirsOnly {
		t.Fatalf("opening should already ask for the folder list: %#v", request)
	}
}

// TestFormOpensOnTheRecentFolders — with history, the list is what was
// opened before and nothing is asked of the engine.
func TestFormOpensOnTheRecentFolders(t *testing.T) {
	recents := []string{"/home/dev/cortz-web", "/home/dev/doxar"}
	f := NewForm(recents, "")
	if f.fields[0].value != "" {
		t.Fatalf("nothing is typed for the user: %q", f.fields[0].value)
	}
	if strings.Join(f.options, " ") != strings.Join(recents, " ") {
		t.Fatalf("the list should be the recent folders, got %v", f.options)
	}
	if _, waiting := f.Pending(); waiting {
		t.Fatal("the recent folders don't need the engine")
	}
}

// TestFormFiltersRecentsByFragment — a loose fragment, no slash in it,
// searches the history instead of the disk.
func TestFormFiltersRecentsByFragment(t *testing.T) {
	f := NewForm([]string{"/home/dev/cortz-web", "/home/dev/doxar"}, "")
	typeText(f, "dox")
	if len(f.options) != 1 || f.options[0] != "/home/dev/doxar" {
		t.Fatalf("the fragment should filter the history, got %v", f.options)
	}
	if _, waiting := f.Pending(); waiting {
		t.Fatal("filtering the history is nobody else's job")
	}
}

// TestFormEntersFolderWithoutTyping — the whole point: the arrow picks and
// walks in, and walking in asks for what's in there.
func TestFormEntersFolderWithoutTyping(t *testing.T) {
	f := browsing("/home/dev/cortz-web/", "/home/dev/doxar/")
	f.Key(keyboard.NextField, "", false) // picks the second one
	f.Key(keyboard.NextOpt, "", false)   // walks in

	if f.fields[0].value != "/home/dev/doxar/" {
		t.Fatalf("the field should have walked into the folder, got %q", f.fields[0].value)
	}
	request, waiting := f.Pending()
	if !waiting || request.Path != "/home/dev/doxar/" || !request.List {
		t.Fatalf("walking in should ask what's inside: %#v", request)
	}
}

// TestFormClimbsOutOfFolder — and the other arrow goes back up.
func TestFormClimbsOutOfFolder(t *testing.T) {
	f := testForm()
	f.fields[0].value = "/home/dev/doxar/"
	f.Key(keyboard.PreviousOpt, "", false)
	if f.fields[0].value != "/home/dev/" {
		t.Fatalf("← should climb one folder, got %q", f.fields[0].value)
	}
	f.Key(keyboard.PreviousOpt, "", false)
	f.Key(keyboard.PreviousOpt, "", false)
	if f.fields[0].value != "/" {
		t.Fatalf("climbing stops at the root, got %q", f.fields[0].value)
	}
}

// TestFormClimbsFromTheRecentPick — from the history, where nothing is
// typed, climbing starts at the picked folder.
func TestFormClimbsFromTheRecentPick(t *testing.T) {
	f := NewForm([]string{"/home/dev/cortz-web"}, "")
	f.Key(keyboard.PreviousOpt, "", false)
	if f.fields[0].value != "/home/dev/" {
		t.Fatalf("← should climb out of the picked folder, got %q", f.fields[0].value)
	}
}

// TestFormConfirmTakesThePickedFolder — on the folder field ↵ settles the
// folder and walks on, it doesn't create.
func TestFormConfirmTakesThePickedFolder(t *testing.T) {
	f := browsing("/home/dev/cortz-web/", "/home/dev/doxar/")
	f.Key(keyboard.NextField, "", false)
	request, open := f.Key(keyboard.Confirm, "", false)
	if request != nil || !open {
		t.Fatal("↵ on the folder field settles the folder, it doesn't create")
	}
	if f.fields[0].value != "/home/dev/doxar" {
		t.Fatalf("the picked folder should be in the field, got %q", f.fields[0].value)
	}
	if f.current != 1 {
		t.Fatalf("after the folder it walks to the name, it's at %d", f.current)
	}
}

// TestFormArrowsWalkTheListBeforeTheFields — the list is the field: only
// once it runs out does the arrow leave.
func TestFormArrowsWalkTheListBeforeTheFields(t *testing.T) {
	f := browsing("/a/", "/b/")
	f.Key(keyboard.NextField, "", false)
	if f.current != 0 || f.highlight != 1 {
		t.Fatalf("↓ should walk the list first: field %d, pick %d", f.current, f.highlight)
	}
	f.Key(keyboard.NextField, "", false)
	if f.current != 1 {
		t.Fatalf("with the list over, ↓ moves on to the next field, it's at %d", f.current)
	}
}

// TestFormDoesNotAskForType — creating a session doesn't force a choice
// between claude, cursor and shell.
func TestFormDoesNotAskForType(t *testing.T) {
	f := testForm()
	for _, label := range labels(f) {
		if label == "TYPE" {
			t.Fatalf("the form no longer asks for the type: %v", labels(f))
		}
	}
	expected := []string{"PROJECT", "NAME", "MD", "PROMPT"}
	if strings.Join(labels(f), " ") != strings.Join(expected, " ") {
		t.Fatalf("fields came out %v, expected %v", labels(f), expected)
	}
}

// TestFormCreatesWithoutSayingTheType — the request goes out with no type,
// and the engine decides.
func TestFormCreatesWithoutSayingTheType(t *testing.T) {
	f := testForm()
	for range 40 {
		f.Key(keyboard.None, "", true) // clears the home path
	}
	typeText(f, "/home/dev/novo")
	f.Key(keyboard.NextField, "", false) // NAME
	typeText(f, "refatora auth")

	request, open := f.Key(keyboard.Confirm, "", false)
	if open {
		t.Fatal("confirm closes the form")
	}
	if request == nil {
		t.Fatal("confirm had to return the create request")
	}
	if request.Type != "" {
		t.Fatalf("the form doesn't choose a type: %#v", request)
	}
	if request.Path != "/home/dev/novo" || request.Name != "refatora auth" {
		t.Fatalf("the request came out wrong: %#v", request)
	}
}

// TestFormSendsMarkdownAsTarget — a filled-in file becomes a reading cell,
// and the engine is the one that translates that.
func TestFormSendsMarkdownAsTarget(t *testing.T) {
	f := testForm()
	f.current = 2 // MD
	typeText(f, "docs/spec-m7.md")

	request, _ := f.Key(keyboard.Confirm, "", false)
	if request == nil {
		t.Fatal("the request should go out")
	}
	if request.Target != "docs/spec-m7.md" {
		t.Fatalf("the file should land in the target: %#v", request)
	}
}

// TestFormListsWhatWasTypedWithoutRewritingIt — typing a path asks the
// engine for the folders in there, and the answer fills the list without
// touching what's being typed.
func TestFormListsWhatWasTypedWithoutRewritingIt(t *testing.T) {
	f := testForm()
	f.fields[0].value = ""
	typeText(f, "/home/dev/cor")

	request, waiting := f.Pending()
	if !waiting {
		t.Fatal("typing a path should ask for the folder list")
	}
	if !request.DirsOnly || !request.List || request.Path != "/home/dev/cor" {
		t.Fatalf("the list request came out wrong: %#v", request)
	}

	f.Completed(protocol.Completed{Path: request.Path, Count: 2, Options: []string{"/home/dev/cortz-api/", "/home/dev/cortz-web/"}})
	if f.fields[0].value != "/home/dev/cor" {
		t.Fatalf("the list must not rewrite what is being typed: %q", f.fields[0].value)
	}
	if len(f.options) != 2 {
		t.Fatalf("the list should be showing both folders, got %v", f.options)
	}
}

// TestFormCompletesFileInsideTheProject — on the markdown field, tab looks
// inside the project being created.
func TestFormCompletesFileInsideTheProject(t *testing.T) {
	f := testForm()
	f.fields[0].value = "/home/dev/cortz"
	f.current = 2
	typeText(f, "READ")

	f.Key(keyboard.Complete, "", false)
	request, waiting := f.Pending()
	if !waiting {
		t.Fatal("the markdown field accepts completion")
	}
	if request.List || request.DirsOnly {
		t.Fatal("here what matters is a file, not a folder")
	}
	if request.Path != "/home/dev/cortz/READ" {
		t.Fatalf("the path should be relative to the project: %q", request.Path)
	}

	f.Completed(protocol.Completed{Path: "/home/dev/cortz/README", Count: 2})
	if !strings.Contains(f.warning, "2 files match") {
		t.Fatalf("the warning should talk about files, got %q", f.warning)
	}
}

// TestFormRejectsEmptyProject — the path is required, and the form stays
// open with whatever was typed.
func TestFormRejectsEmptyProject(t *testing.T) {
	f := testForm()
	f.fields[0].value = ""
	f.current = 1
	request, open := f.Key(keyboard.Confirm, "", false)
	if request != nil {
		t.Fatal("nothing gets created without a path")
	}
	if !open {
		t.Fatal("the form stays open when the path is missing")
	}
	if f.warning == "" {
		t.Fatal("the form needs to say what's missing")
	}
}

// TestFormErasesWithBackspace covers editing the text field.
func TestFormErasesWithBackspace(t *testing.T) {
	f := testForm()
	f.fields[0].value = "abc"
	f.Key(keyboard.None, "", true)
	if f.fields[0].value != "ab" {
		t.Fatalf("erasing left %q", f.fields[0].value)
	}
}

// TestFormCancelsWithEsc.
func TestFormCancelsWithEsc(t *testing.T) {
	f := testForm()
	request, open := f.Key(keyboard.Cancel, "", false)
	if request != nil || open {
		t.Fatal("esc cancels without creating anything")
	}
}

// TestFormMovesBetweenFields.
func TestFormMovesBetweenFields(t *testing.T) {
	f := testForm()
	f.Key(keyboard.NextField, "", false)
	if f.current != 1 {
		t.Fatalf("↓ should go to the next field, it's at %d", f.current)
	}
	f.Key(keyboard.PreviousField, "", false)
	if f.current != 0 {
		t.Fatalf("↑ should go back, it's at %d", f.current)
	}
	f.Key(keyboard.PreviousField, "", false)
	if f.current != len(f.fields)-1 {
		t.Fatalf("↑ on the first field wraps around, it's at %d", f.current)
	}
}

// TestFormSaysEachThingOnce — the form opens on the focused project's folder
// and says it once: the list under the field doesn't repeat the path that's
// already written in it. Only the field being written shows its hint, and
// the keys that settle the form are on the box's last line alone.
func TestFormSaysEachThingOnce(t *testing.T) {
	form := NewForm([]string{"/work/cortz-web"}, "/work/doxar-api")
	form.Pending()
	form.Completed(protocol.Completed{Options: []string{"/work/doxar-api/"}, Count: 1})
	drawn := noStyle(strings.Join(form.Draw(100), "\n"))
	if got := strings.Count(drawn, "/work/doxar-api"); got != 1 {
		t.Errorf("the folder should be written once, got %d:\n%s", got, drawn)
	}
	if got := strings.Count(drawn, "↵"); got != 1 {
		t.Errorf("the key that confirms should be said once, got %d:\n%s", got, drawn)
	}
	if !strings.Contains(drawn, "→ enters") || strings.Contains(drawn, "automatic name") || strings.Contains(drawn, "optional") {
		t.Errorf("only the field being written shows its hint:\n%s", drawn)
	}

	// The pick is still there: ↵ takes it and moves on, and the hint moves
	// with the caret.
	form.Key(keyboard.Confirm, "", false)
	drawn = noStyle(strings.Join(form.Draw(100), "\n"))
	if strings.Contains(drawn, "→ enters") || !strings.Contains(drawn, "automatic name") {
		t.Errorf("the hint should follow the field in focus:\n%s", drawn)
	}

	// Folders that merely share the prefix are a choice, and are listed.
	form = NewForm(nil, "/work/doxar")
	form.Completed(protocol.Completed{Options: []string{"/work/doxar-api/", "/work/doxar-web/"}, Count: 2})
	if drawn := noStyle(strings.Join(form.Draw(100), "\n")); !strings.Contains(drawn, "/work/doxar-web") {
		t.Errorf("a list with more than the field's own folder is shown:\n%s", drawn)
	}
}
