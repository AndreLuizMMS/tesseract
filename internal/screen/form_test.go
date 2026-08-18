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

// TestFormStartsAtHome — the path already points at home, since that's
// where every project starts from.
func TestFormStartsAtHome(t *testing.T) {
	f := testForm()
	value := f.fields[0].value
	if value != homeWithSlash() {
		t.Fatalf("the field should start at home, got %q", value)
	}
	if !strings.HasSuffix(value, "/") {
		t.Fatalf("the path should end in a slash for tab completion from inside it: %q", value)
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

// TestFormCompletesFolderPath — on the project field, tab only looks at
// folders.
func TestFormCompletesFolderPath(t *testing.T) {
	f := testForm()
	typeText(f, "dev/cor")

	request, wants := f.WantsComplete(keyboard.Complete)
	if !wants {
		t.Fatal("the project field accepts completion")
	}
	if !request.DirsOnly || !strings.HasSuffix(request.Path, "dev/cor") {
		t.Fatalf("the completion request came out wrong: %#v", request)
	}

	f.Completed(protocol.Completed{Path: "/home/andreluiz/dev/cortz-", Count: 3})
	if f.fields[0].value != "/home/andreluiz/dev/cortz-" {
		t.Fatalf("the field didn't get the completion: %q", f.fields[0].value)
	}
	if !strings.Contains(f.warning, "3 folders match") {
		t.Fatalf("the warning should count how many match, got %q", f.warning)
	}
}

// TestFormCompletesFileInsideTheProject — on the markdown field, tab looks
// inside the project being created.
func TestFormCompletesFileInsideTheProject(t *testing.T) {
	f := testForm()
	f.fields[0].value = "/home/dev/cortz"
	f.current = 2
	typeText(f, "READ")

	request, wants := f.WantsComplete(keyboard.Complete)
	if !wants {
		t.Fatal("the markdown field accepts completion")
	}
	if request.DirsOnly {
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
