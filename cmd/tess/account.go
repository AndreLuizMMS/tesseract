package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/engine"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/screen"
	"github.com/andreluiz/tesseract/internal/theme"
)

// statusWait bounds asking Claude Code who is logged into a folder.
const statusWait = 15 * time.Second

// accountName is what goes after ~/.claude-: it becomes a folder name.
var accountName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,30}$`)

// claudeLogin is the part of `claude auth status` that says who's in.
type claudeLogin struct {
	LoggedIn     bool   `json:"loggedIn"`
	Email        string `json:"email"`
	Subscription string `json:"subscriptionType"`
}

// account runs `ts account` and `ts account add`.
func account(args []string) error {
	if len(args) == 0 {
		return listAccounts()
	}
	if args[0] != "add" || len(args) > 2 {
		return errors.New("usage: ts account [add [name]]")
	}
	name := ""
	if len(args) == 2 {
		name = args[1]
	}
	return addAccount(name)
}

// listAccounts shows each Claude account, primary first, with who is
// logged into it.
func listAccounts() error {
	config, err := engine.LoadConfig(engine.ConfigPath())
	if err != nil {
		return err
	}
	folders := engine.ExpandAccounts(config.ClaudeAccounts)
	if len(folders) == 0 {
		folders = []string{cell.NativeAccount()}
	}
	for i, folder := range folders {
		role := ""
		if i == 0 {
			role = " · primary"
		}
		fmt.Printf("%d  %s  %s%s\n", i+1, shortPath(folder), describeLogin(claudeProgram(config), folder), role)
	}
	if len(folders) < 2 {
		fmt.Println("one account: `ts account add` sets up the one a conversation moves to when this one runs out")
	}
	return nil
}

// addAccount sets up another Claude account and puts it on the list: the
// folder, the login, everything it shares with the primary, the config, and
// the engine told about it.
func addAccount(name string) error {
	config, err := engine.LoadConfig(engine.ConfigPath())
	if err != nil {
		return err
	}
	program := claudeProgram(config)
	listed := engine.ExpandAccounts(config.ClaudeAccounts)
	if name == "" {
		name = freeAccountName(listed)
	}
	if !accountName.MatchString(name) {
		return fmt.Errorf("account name %q: lowercase letters, digits and dashes only", name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	folder := filepath.Join(home, ".claude-"+name)

	primary, _ := claudeStatus(program, cell.NativeAccount())
	login, _ := claudeStatus(program, folder)
	if !login.LoggedIn {
		fmt.Printf("%s logging in the account that lives in %s\n", theme.Glyph, shortPath(folder))
		fmt.Println("  the browser opens on claude.ai: sign in with the OTHER account.")
		fmt.Println("  already signed in there as the primary? open the link Claude prints in a private window.")
		fmt.Println()
		if err := os.MkdirAll(folder, 0o700); err != nil {
			return err
		}
		signIn := exec.Command(program, "auth", "login", "--claudeai")
		signIn.Env = accountEnvironment(folder)
		signIn.Stdin, signIn.Stdout, signIn.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := signIn.Run(); err != nil {
			return fmt.Errorf("the login didn't finish (%w). `ts account add %s` tries again", err, name)
		}
		if login, err = claudeStatus(program, folder); err != nil || !login.LoggedIn {
			return fmt.Errorf("Claude Code says nobody is logged into %s. `ts account add %s` tries again", shortPath(folder), name)
		}
	}
	// The same account twice is no failover: both run out at once.
	if primary.Email != "" && strings.EqualFold(login.Email, primary.Email) {
		logout := exec.Command(program, "auth", "logout")
		logout.Env = accountEnvironment(folder)
		_ = logout.Run()
		return fmt.Errorf("%s is the primary account, so it was logged out of %s. `ts account add %s` again, signed in with the other one — a private window keeps the primary out of the way", login.Email, shortPath(folder), name)
	}

	if err := cell.SetUpClaudeAccount(folder); err != nil {
		return fmt.Errorf("setting up %s: %w", shortPath(folder), err)
	}
	accounts, err := addToConfig(engine.ConfigPath(), listed, folder)
	if err != nil {
		return err
	}

	fmt.Printf("%s %s · %s, sharing the primary's instructions, settings, skills, plugins and MCP servers\n", theme.Glyph, shortPath(folder), describeLogin(program, folder))
	fmt.Printf("  new Claude cells run on %s while it has quota, and a conversation that runs out moves on down the list: %s\n", shortPath(accounts[0]), strings.Join(shortPaths(accounts), " → "))
	fmt.Println("  engine:", tellEngine())
	return nil
}

// freeAccountName is the first number from 2 on whose folder isn't listed.
func freeAccountName(listed []string) string {
	home, _ := os.UserHomeDir()
	for n := 2; ; n++ {
		name := strconv.Itoa(n)
		taken := false
		for _, folder := range listed {
			taken = taken || folder == filepath.Join(home, ".claude-"+name)
		}
		if !taken {
			return name
		}
	}
}

// addToConfig puts folder at the end of the account list, with Claude
// Code's own folder at the front when the list didn't exist yet, and keeps
// every other key the file has. It returns the list as it ends up.
func addToConfig(path string, listed []string, folder string) ([]string, error) {
	if len(listed) == 0 {
		listed = []string{cell.NativeAccount()}
	}
	accounts := listed
	found := false
	for _, existing := range listed {
		found = found || existing == folder
	}
	if !found {
		accounts = append(append([]string(nil), listed...), folder)
	}

	raw := map[string]json.RawMessage{}
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(content) > 0 {
		if err := json.Unmarshal(content, &raw); err != nil {
			return nil, fmt.Errorf("%s doesn't parse, so it wasn't touched: %w", path, err)
		}
	}
	list, err := json.Marshal(shortPaths(accounts))
	if err != nil {
		return nil, err
	}
	raw["claudeAccounts"] = list
	written, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return accounts, os.WriteFile(path, append(written, '\n'), 0o644)
}

// tellEngine has a running engine read the accounts again. One that isn't
// running reads them when it starts.
func tellEngine() string {
	client, err := screen.Connect(engine.SocketPath())
	if err != nil {
		return "not running; it reads the accounts when it starts"
	}
	defer client.Close()
	if err := client.Send(protocol.TypeAccounts, struct{}{}); err != nil {
		return err.Error()
	}
	summary, err := waitSummary(client)
	if err != nil {
		return err.Error()
	}
	return summary
}

// claudeStatus asks Claude Code who is logged into the folder.
func claudeStatus(program, folder string) (claudeLogin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), statusWait)
	defer cancel()
	status := exec.CommandContext(ctx, program, "auth", "status", "--json")
	status.Env = accountEnvironment(folder)
	// Logged out exits with an error and still prints the JSON.
	output, _ := status.Output()
	var login claudeLogin
	if err := json.Unmarshal(output, &login); err != nil {
		return claudeLogin{}, fmt.Errorf("claude auth status: %w", err)
	}
	return login, nil
}

// describeLogin is who is logged into the folder, in a few words.
func describeLogin(program, folder string) string {
	login, err := claudeStatus(program, folder)
	switch {
	case err != nil:
		return "unknown (" + err.Error() + ")"
	case !login.LoggedIn:
		return "not logged in"
	case login.Subscription != "":
		return login.Email + " (" + login.Subscription + ")"
	default:
		return login.Email
	}
}

// accountEnvironment points Claude Code at the folder. Claude Code's own
// folder gets no variable, the same way a cell runs on it.
func accountEnvironment(folder string) []string {
	if folder == cell.NativeAccount() {
		return os.Environ()
	}
	return append(os.Environ(), "CLAUDE_CONFIG_DIR="+folder)
}

// claudeProgram is the program the cells start Claude Code with.
func claudeProgram(config engine.Config) string {
	if program := config.Agents["claude"].Program; program != "" {
		return program
	}
	return "claude"
}

// shortPath writes a folder under the home as ~/…, the way the config
// shows it.
func shortPath(folder string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return folder
	}
	if rest, ok := strings.CutPrefix(folder, home+"/"); ok {
		return "~/" + rest
	}
	return folder
}

func shortPaths(folders []string) []string {
	short := make([]string, len(folders))
	for i, folder := range folders {
		short[i] = shortPath(folder)
	}
	return short
}
