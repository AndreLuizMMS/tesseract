package engine

import (
	"os"
	"os/exec"
	"strings"
	"time"
)

// loginTimeout limits reading the shell's environment: if the user's rc file
// hangs, the engine comes up anyway.
const loginTimeout = 5 * time.Second

// InheritLoginPath puts into the engine the PATH the user has in their
// terminal.
//
// Without this, the engine running as a service only sees systemd's short
// PATH — and the agents installed on the account (claude, cursor-agent) and
// the tools they call (node, pnpm) simply wouldn't exist. Runs once, at
// startup, and fails silently: a short PATH is worse, not fatal.
func InheritLoginPath() {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	command := exec.Command(shell, "-lic", "printf '%s' \"$PATH\"")
	command.Env = append(os.Environ(), "TESSERACT_READING_ENVIRONMENT=1")
	ready := make(chan []byte, 1)
	go func() {
		output, err := command.Output()
		if err != nil {
			ready <- nil
			return
		}
		ready <- output
	}()

	select {
	case output := <-ready:
		if path := lastLine(string(output)); strings.Contains(path, "/") {
			os.Setenv("PATH", path)
		}
	case <-time.After(loginTimeout):
		if command.Process != nil {
			_ = command.Process.Kill()
		}
	}
}

// lastLine is what's left after whatever the user's rc file chose to print.
func lastLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
