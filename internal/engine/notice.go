package engine

import (
	"os/exec"
	"strings"
)

// The notice comes from the engine, not the screen. That's why the user gets
// notified even with the screen closed — which the fork didn't do.
const noticeTitle = "Tesseract"

// announce plays the alert sound and raises the system notification. Both are
// independently toggleable, and neither waits for a response: the notifier is
// an outside program and can be slow to come up.
func (e *Engine) announce(body string) {
	e.mu.Lock()
	sound, notify, command := e.config.Sound, e.config.Notify, e.config.NotifyCommand
	e.mu.Unlock()

	if sound {
		release(soundCommand())
	}
	if notify {
		release(notificationCommand(command, noticeTitle, body))
	}
}

// soundCommand is the audible alert. WSL has no sound daemon, so whoever
// beeps is the Windows side.
func soundCommand() *exec.Cmd {
	if path, err := exec.LookPath("powershell.exe"); err == nil {
		return exec.Command(path, "-NoProfile", "-Command", "[console]::beep(880,180)")
	}
	if path, err := exec.LookPath("paplay"); err == nil {
		return exec.Command(path, "/usr/share/sounds/freedesktop/stereo/message.oga")
	}
	return nil
}

// notificationCommand builds the command that raises the system notification.
// Returns nil when the machine has no notifier at all — the lack of a
// notifier isn't an error worth showing.
//
// custom, when filled in, is a command line whose program receives the title
// and body as its last two arguments.
func notificationCommand(custom, title, body string) *exec.Cmd {
	if fields := strings.Fields(custom); len(fields) > 0 {
		return exec.Command(fields[0], append(fields[1:], title, body)...)
	}
	// wsl-notify-send bridges to the Windows toast; notify-send is the Linux
	// default when a daemon is present.
	for _, name := range []string{"wsl-notify-send.exe", "notify-send"} {
		if path, err := exec.LookPath(name); err == nil {
			return exec.Command(path, title, body)
		}
	}
	if path, err := exec.LookPath("powershell.exe"); err == nil {
		return exec.Command(path, "-NoProfile", "-Command", toastScript(title, body))
	}
	return nil
}

// toastScript is the path that always exists on WSL: the Windows PowerShell
// showing a balloon with the cell and project name.
func toastScript(title, body string) string {
	return strings.Join([]string{
		"[reflection.assembly]::LoadWithPartialName('System.Windows.Forms') | Out-Null;",
		"$notice = New-Object System.Windows.Forms.NotifyIcon;",
		"$notice.Icon = [System.Drawing.SystemIcons]::Information;",
		"$notice.BalloonTipTitle = " + asPowerShellLiteral(title) + ";",
		"$notice.BalloonTipText = " + asPowerShellLiteral(body) + ";",
		"$notice.Visible = $true;",
		"$notice.ShowBalloonTip(6000);",
		"Start-Sleep -Seconds 7;",
		"$notice.Dispose()",
	}, " ")
}

// asPowerShellLiteral writes the text as a PowerShell string literal,
// escaping whatever would close the quote.
func asPowerShellLiteral(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}

// release fires the command without waiting: the notice never blocks the
// engine.
func release(command *exec.Cmd) {
	if command == nil {
		return
	}
	if err := command.Start(); err != nil {
		return
	}
	go func() { _ = command.Wait() }()
}
