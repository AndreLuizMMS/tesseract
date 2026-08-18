package cell

import "os"

func init() {
	Register(Descriptor{Type: "bash", Order: 30}, func() Cell { return &Bash{} })
}

// Bash is a shell running in the project's root.
type Bash struct {
	Process
}

func (b *Bash) Spawn(cfg Config) error {
	return b.start(cfg, userShell(), nil, terminalEnvironment())
}

// userShell respects the account's choice and falls back to bash when there
// is none.
func userShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/bash"
}

// terminalEnvironment is the engine's environment with the terminal
// declared, because the internal screen emulates a colored xterm.
//
// TESSERACT=1 is the Tesseract introducing itself to whatever runs inside
// it. It can't paint an agent's interface — what the cell shows is what the
// process wrote, and nothing else. What it can do is say "you're in here",
// and let whoever cares dress accordingly: an agent's statusline, a shell
// prompt, a project script.
func terminalEnvironment() []string {
	environment := make([]string, 0, len(os.Environ())+3)
	for _, pair := range os.Environ() {
		switch {
		case len(pair) > 5 && pair[:5] == "TERM=":
		case len(pair) > 10 && pair[:10] == "COLORTERM=":
		case len(pair) > 10 && pair[:10] == "TESSERACT=":
		default:
			environment = append(environment, pair)
		}
	}
	return append(environment, "TERM=xterm-256color", "COLORTERM=truecolor", "TESSERACT=1")
}
