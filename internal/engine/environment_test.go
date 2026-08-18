package engine

import (
	"os"
	"strings"
	"testing"
)

// TestInheritLoginPath — the engine needs to see the tools the user has in
// their terminal, not just the service's short PATH.
func TestInheritLoginPath(t *testing.T) {
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })

	os.Setenv("PATH", "/just/that")
	InheritLoginPath()

	after := os.Getenv("PATH")
	if after == "/just/that" {
		t.Skip("this machine's shell returned no PATH at all")
	}
	if !strings.Contains(after, "/just/that") {
		t.Fatalf("the service's PATH should still be in there: %q", after)
	}
	if len(strings.Split(after, ":")) < 3 {
		t.Fatalf("the inherited PATH came too short: %q", after)
	}
}

// TestLastLine — the user's rc file may print things beforehand.
func TestLastLine(t *testing.T) {
	if got := lastLine("rc file banner\n/usr/bin:/bin\n"); got != "/usr/bin:/bin" {
		t.Fatalf("got %q", got)
	}
	if got := lastLine("  /usr/bin  "); got != "/usr/bin" {
		t.Fatalf("got %q", got)
	}
}
