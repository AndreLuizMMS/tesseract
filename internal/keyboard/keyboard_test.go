package keyboard

import (
	"strings"
	"testing"
)

// TestOneKeyOneMeaning is the mechanical guarantee of the rule: if this
// test breaks, someone gave two meanings to the same key in the same mode —
// which is exactly what rotted the fork.
func TestOneKeyOneMeaning(t *testing.T) {
	for _, mode := range Modes() {
		seen := map[string]Action{}
		for _, shortcut := range Shortcuts(mode) {
			if previous, repeated := seen[shortcut.Key]; repeated {
				t.Errorf("mode %s: key %q means %q and also %q", mode, shortcut.Key, previous, shortcut.Action)
			}
			seen[shortcut.Key] = shortcut.Action
		}
	}
}

// TestTypeOnlyReservesCtrlL — in TYPE every key belongs to the cell, with
// no exception besides the one that returns the keyboard.
func TestTypeOnlyReservesCtrlL(t *testing.T) {
	shortcutList := Shortcuts(Type)
	if len(shortcutList) != 1 {
		t.Fatalf("TYPE reserves %d keys, and it can only reserve one: %#v", len(shortcutList), shortcutList)
	}
	if shortcutList[0].Key != "ctrl+l" || shortcutList[0].Action != ExitType {
		t.Fatalf("the one key reserved in TYPE must be ctrl-l: %#v", shortcutList[0])
	}
	for _, key := range []string{"q", "D", "tab", "esc", "enter", "up", "down", "left", "right", "/", "?"} {
		if action := Lookup(Type, key); action != None {
			t.Errorf("in TYPE the key %q had to go to the cell, but it means %q", key, action)
		}
	}
}

// TestEveryKeyHasHelp — a key with no help is a key nobody discovers.
func TestEveryKeyHasHelp(t *testing.T) {
	for _, mode := range Modes() {
		for _, shortcut := range Shortcuts(mode) {
			if shortcut.Help == "" {
				t.Errorf("mode %s: key %q has no help text", mode, shortcut.Key)
			}
			if shortcut.Action == None {
				t.Errorf("mode %s: key %q does nothing", mode, shortcut.Key)
			}
		}
	}
}

// TestNoLetterMovesThroughTheGrid — movement is directional, period. It's
// where the feeling of an unpredictable key came from in the fork.
func TestNoLetterMovesThroughTheGrid(t *testing.T) {
	moves := map[Action]bool{
		CellPrevious: true, CellNext: true,
		CellUp: true, CellDown: true,
	}
	arrows := map[string]bool{"up": true, "down": true, "left": true, "right": true}
	for _, shortcut := range Shortcuts(Browse) {
		if moves[shortcut.Action] && !arrows[shortcut.Key] {
			t.Errorf("key %q moves through the grid without being an arrow", shortcut.Key)
		}
	}
	for _, letter := range []string{"j", "k", "h", "l", "J", "K", "H", "L"} {
		if action := Lookup(Browse, letter); action != None {
			t.Errorf("letter %q can't mean anything in BROWSE, but it means %q", letter, action)
		}
	}
}

// TestHelpGroupsTheFamilies — the help shows one line per idea, not nine
// identical lines for the nine numbers.
func TestHelpGroupsTheFamilies(t *testing.T) {
	lines := HelpLines(Browse)
	count := map[string]int{}
	for _, line := range lines {
		count[line.Keys]++
		if line.Help == "" {
			t.Errorf("the help line for %q is empty", line.Keys)
		}
	}
	for keys, how := range count {
		if how > 1 {
			t.Errorf("the help repeats line %q %d times", keys, how)
		}
	}
	if count["1…9 project N"] != 1 {
		t.Error("the numbers should show up as a single help line")
	}
	if count["↑↓←→ cell"] != 1 {
		t.Error("the four arrows should show up as a single help line")
	}
}

// TestDockerPanelHasItsOwnKeyboard — the panel doesn't borrow meaning from
// the global map without declaring it, and uppercase is always the stack
// version.
func TestDockerPanelHasItsOwnKeyboard(t *testing.T) {
	pairs := map[string]string{"u": "U", "s": "S", "r": "R"}
	stackWide := map[Action]bool{UpStack: true, StopStack: true, RestartStack: true}
	for lower, upper := range pairs {
		if Lookup(DockerPanel, lower) == None {
			t.Errorf("the panel should declare the key %q", lower)
		}
		action := Lookup(DockerPanel, upper)
		if !stackWide[action] {
			t.Errorf("uppercase %q had to be the stack version, but it's %q", upper, action)
		}
	}
	if Lookup(DockerPanel, "esc") != Back {
		t.Error("esc must return the keyboard to the application")
	}
	// No destructive action lives here.
	for _, shortcut := range Shortcuts(DockerPanel) {
		for _, forbidden := range []string{"erase", "remove", "volume", "destroy"} {
			if contains(shortcut.Help, forbidden) {
				t.Errorf("the Docker panel can't have a destructive action: %q", shortcut.Help)
			}
		}
	}
}

func contains(text, piece string) bool {
	return strings.Contains(text, piece)
}

// TestFooterHasShortText — a key that shows up in the footer needs a label
// that fits in it.
func TestFooterHasShortText(t *testing.T) {
	for _, mode := range Modes() {
		for _, shortcut := range Shortcuts(mode) {
			if shortcut.Footer && shortcut.Short == "" {
				t.Errorf("mode %s: key %q is in the footer with no short label", mode, shortcut.Key)
			}
		}
	}
}
