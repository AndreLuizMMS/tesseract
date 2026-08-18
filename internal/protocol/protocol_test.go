package protocol

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

// testCase is one contract message with a well-filled example value.
type testCase struct {
	name  string
	typ   string
	value any
	empty func() any
}

func testCases() []testCase {
	return []testCase{
		{"key", TypeKey, Key{Cell: "c1", Code: 'l', Text: "l"}, func() any { return new(Key) }},
		{"pasted key", TypeKey, Key{Cell: "c1", Paste: "ls -la\n"}, func() any { return new(Key) }},
		{"size", TypeSize, Size{Cell: "c1", Cols: 120, Rows: 40}, func() any { return new(Size) }},
		{"scroll", TypeScroll, Scroll{Cell: "c1", Delta: -3, Live: true}, func() any { return new(Scroll) }},
		{"create", TypeCreate, Create{Path: "/dev/x", Type: "bash", Name: "test", Target: "api", Prompt: "hi"}, func() any { return new(Create) }},
		{"kill", TypeKill, Kill{Cell: "c1"}, func() any { return new(Kill) }},
		{"rename", TypeRename, Rename{Cell: "c1"}, func() any { return new(Rename) }},
		{"resume", TypeResume, Resume{Cell: "c1"}, func() any { return new(Resume) }},
		{"tab", TypeTab, Tab{Cell: "c1", Step: 1}, func() any { return new(Tab) }},
		{"prompt", TypePrompt, Prompt{Cell: "c1", Text: "cover the mobile menu"}, func() any { return new(Prompt) }},
		{"complete", TypeComplete, Complete{Path: "~/dev/cor", DirsOnly: true}, func() any { return new(Complete) }},
		{"completed", TypeCompleted, Completed{Path: "/home/a/dev/cortz", Count: 3}, func() any { return new(Completed) }},
		{"screen", TypeScreen, Screen{Screen: "list"}, func() any { return new(Screen) }},
		{"search", TypeSearch, Search{Cell: "c1", Term: "error"}, func() any { return new(Search) }},
		{"matches", TypeMatches, Matches{Cell: "c1", Term: "error", Lines: []Match{{Line: 12, Text: "error: x"}}}, func() any { return new(Matches) }},
		{"services", TypeServices, Services{
			Project: "p1", File: "/dev/cortz/docker-compose.yml", Action: "up", Service: "api",
			List: []Service{{Name: "api", State: "up", Port: ":3000", Health: "healthy", Uptime: "2h14m"}},
		}, func() any { return new(Services) }},
		{"editor", TypeEditor, Editor{Project: "p1"}, func() any { return new(Editor) }},
		{"go to line", TypeGoToLine, GoToLine{Cell: "c1", Line: 42}, func() any { return new(GoToLine) }},
		{"docker", TypeDocker, Docker{Project: "p1", Action: "up", Service: "api"}, func() any { return new(Docker) }},
		{"status", TypeStatus, Summary{}, func() any { return new(Summary) }},
		{"stop", TypeStop, Summary{}, func() any { return new(Summary) }},
		{"summary", TypeSummary, Summary{Text: "engine running · 1 project"}, func() any { return new(Summary) }},
		{"error", TypeError, Error{Message: "path does not exist"}, func() any { return new(Error) }},
		{"state", TypeState, State{
			Notice: "previous state preserved",
			Screen: "grid",
			Quota:  &Quota{Percent: 59, Rollover: "2:47"},
			Types:  []CellType{{Type: "md", TargetLabel: "MD", CompletesPath: true}},
			Projects: []Project{{
				ID: "p1", Path: "/dev/cortz", Name: "cortz", Color: 2,
				HasCompose: true, Docker: "4/5",
				Cells: []Cell{{
					ID: "c1", Type: "bash", Name: "test", State: "working",
					Lines: []string{"$ pnpm test", "  ok"}, CursorX: 2, CursorY: 1,
					Scroll: 4, Live: false,
					Tabs: []string{"claude", "cursor", "bash"}, Tab: "claude",
				}},
			}},
		}, func() any { return new(State) }},
	}
}

// TestRoundTrip guarantees every contract message crosses the socket and
// comes back the same. It's what keeps the screen and the engine from
// silently disagreeing.
func TestRoundTrip(t *testing.T) {
	for _, c := range testCases() {
		t.Run(c.name, func(t *testing.T) {
			env, err := Pack(c.typ, c.value)
			if err != nil {
				t.Fatalf("pack: %v", err)
			}

			var wire bytes.Buffer
			if err := json.NewEncoder(&wire).Encode(env); err != nil {
				t.Fatalf("write to wire: %v", err)
			}
			if bytes.Count(wire.Bytes(), []byte("\n")) != 1 {
				t.Fatalf("message must take up exactly one line, got %q", wire.String())
			}

			var arrived Message
			if err := json.NewDecoder(&wire).Decode(&arrived); err != nil {
				t.Fatalf("read from wire: %v", err)
			}
			if arrived.Type != c.typ {
				t.Fatalf("type became %q, expected %q", arrived.Type, c.typ)
			}

			dest := c.empty()
			if err := json.Unmarshal(arrived.Data, dest); err != nil {
				t.Fatalf("unpack: %v", err)
			}
			back := reflect.ValueOf(dest).Elem().Interface()
			if !reflect.DeepEqual(back, c.value) {
				t.Fatalf("value changed on the trip:\nwent  %#v\ncame back %#v", c.value, back)
			}
		})
	}
}

// TestEveryTypeHasCase is the mechanical guarantee that nobody adds a
// message to the contract without proving it comes back the same.
func TestEveryTypeHasCase(t *testing.T) {
	all := []string{
		TypeKey, TypeSize, TypeScroll, TypeCreate, TypeKill, TypeRename,
		TypeResume, TypeTab, TypePrompt, TypeComplete, TypeScreen, TypeSearch,
		TypeDocker, TypeEditor, TypeGoToLine, TypeStatus, TypeStop, TypeState, TypeCompleted,
		TypeMatches, TypeServices, TypeSummary, TypeError,
	}
	covered := map[string]bool{}
	for _, c := range testCases() {
		covered[c.typ] = true
	}
	for _, typ := range all {
		if !covered[typ] {
			t.Errorf("message %q has no round-trip case", typ)
		}
	}
}

// TestUnpackTyped covers the generic shortcut used by the engine and the
// screen.
func TestUnpackTyped(t *testing.T) {
	env, err := Pack(TypeKey, Key{Cell: "c9", Code: 'c', Mod: 4})
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	key, err := Unpack[Key](env)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if key.Cell != "c9" || key.Code != 'c' || key.Mod != 4 {
		t.Fatalf("key came back wrong: %#v", key)
	}
}
