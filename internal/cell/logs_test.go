package cell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// prepareStack sets up a project with a real compose file and makes sure the
// stack starts down. Without Docker on the machine, the test is skipped.
func prepareStack(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("no docker on this machine")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker isn't responding")
	}

	dir := t.TempDir()
	compose := "services:\n  web:\n    image: nginx:alpine\n    command: [\"sh\", \"-c\", \"while true; do echo tesseract-log; sleep 1; done\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	t.Cleanup(func() {
		// Stop, never bring down with volumes: the rule holds even in tests.
		_ = exec.Command("docker", "compose", "--file", filepath.Join(dir, "docker-compose.yml"), "stop").Run()
		_ = exec.Command("docker", "compose", "--file", filepath.Join(dir, "docker-compose.yml"), "rm", "-f").Run()
	})
	return dir
}

// TestLogsStaysStoppedAndReattachesWhenTheServiceComesUp is the behavior
// recovery depends on: the stack is down, the cell waits, and when the
// service comes up it starts showing the log on its own.
func TestLogsStaysStoppedAndReattachesWhenTheServiceComesUp(t *testing.T) {
	dir := prepareStack(t)

	item, err := New("logs")
	if err != nil {
		t.Fatalf("manufacture: %v", err)
	}
	if err := item.Spawn(Config{ID: "c1", Directory: dir, Target: "web", Columns: 60, Lines: 12}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer item.Kill()

	if !waitFor(t, 10*time.Second, func() bool { return item.State() == Stopped }) {
		t.Fatalf("a down service should leave the cell stopped, it's %q", item.State())
	}

	up := exec.Command("docker", "compose", "--file", filepath.Join(dir, "docker-compose.yml"), "up", "-d")
	up.Dir = dir
	if output, err := up.CombinedOutput(); err != nil {
		t.Skipf("couldn't bring up the test stack: %v\n%s", err, output)
	}

	if !waitFor(t, 30*time.Second, func() bool {
		return strings.Contains(screenOf(item), "tesseract-log")
	}) {
		t.Fatalf("the cell didn't reattach on its own when the service came up:\nstate %q\n%s", item.State(), screenOf(item))
	}
	if item.State() != Working {
		t.Fatalf("with the log running, the cell is %q", item.State())
	}
}

// TestLogsWithoutComposeFailsClearly — a project without compose has nothing
// to follow.
func TestLogsWithoutComposeFailsClearly(t *testing.T) {
	item, _ := New("logs")
	err := item.Spawn(Config{ID: "c1", Directory: t.TempDir(), Target: "web", Columns: 60, Lines: 12})
	if err == nil {
		t.Fatal("without compose, the logs cell can't spawn")
	}
	if !strings.Contains(err.Error(), "compose") {
		t.Fatalf("unclear error message: %v", err)
	}
}

// TestLogsWithoutServiceFailsClearly — log of which service?
func TestLogsWithoutServiceFailsClearly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	item, _ := New("logs")
	if err := item.Spawn(Config{ID: "c1", Directory: dir, Columns: 60, Lines: 12}); err == nil {
		t.Fatal("without a service, the logs cell can't spawn")
	}
}

// TestLogsIsReadOnly — a keystroke typed into a logs cell goes nowhere.
func TestLogsIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services:\n  web:\n    image: nginx\n"), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	item, _ := New("logs")
	if err := item.Spawn(Config{ID: "c1", Directory: dir, Target: "web", Columns: 60, Lines: 12}); err != nil {
		t.Skipf("no docker to bring the cell up: %v", err)
	}
	defer item.Kill()
	if err := item.Key(Keystroke{Code: 'x', Text: "x"}); err != nil {
		t.Fatalf("a logs cell ignores a keystroke, it doesn't return an error: %v", err)
	}
}
