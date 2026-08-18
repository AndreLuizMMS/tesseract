// Package docker talks to the project's Docker Compose over the command line.
// Nothing here is destructive: no tearing down with volumes, no deleting
// anything.
package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ignoredDirs never enter the search for a compose file: they're heavy and
// never hold the project's stack.
var ignoredDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true, "dist": true,
	"build": true, "target": true, ".next": true, ".venv": true,
	"venv": true, "coverage": true, "tmp": true, ".cache": true,
}

// forbidden marks the file that must never be chosen on its own. Production
// isn't a development-panel thing.
var forbidden = []string{"prod", "production", "staging", "homolog"}

// readTimeout limits how long we wait for Docker to answer a query.
const readTimeout = 20 * time.Second

// actionTimeout is bigger because bringing up a service pulls an image.
const actionTimeout = 5 * time.Minute

// Service is a row of the panel.
type Service struct {
	Name   string `json:"name"`
	State  string `json:"state"`  // up, exited (1), created…
	Port   string `json:"port"`   // empty when the service publishes nothing
	Health string `json:"health"` // empty when there's no healthcheck
	Uptime string `json:"uptime"`
}

// Detect finds the project's compose file. It looks first at the root and
// then in first-level folders — because a real project keeps its stack in
// `docker/`, `infra/` and the like — and never picks a production file.
func Detect(dir string) string {
	best, bestScore := "", 0
	consider := func(path string, score int) {
		if score > bestScore {
			best, bestScore = path, score
		}
	}

	for _, entry := range entriesOf(dir) {
		path := filepath.Join(dir, entry.Name())
		if !entry.IsDir() {
			consider(path, scoreFile(entry.Name(), true))
			continue
		}
		if ignoredDirs[entry.Name()] || strings.HasPrefix(entry.Name(), ".") && entry.Name() != ".docker" {
			continue
		}
		for _, inner := range entriesOf(path) {
			if inner.IsDir() {
				continue
			}
			consider(filepath.Join(path, inner.Name()), scoreFile(inner.Name(), false))
		}
	}
	return best
}

func entriesOf(dir string) []os.DirEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	return entries
}

// scoreFile scores a candidate compose file. Zero means it doesn't qualify.
// A canonical name beats a variant, root beats subfolder, and anything that
// smells like production is worth nothing.
func scoreFile(name string, atRoot bool) int {
	lower := strings.ToLower(name)
	if !strings.Contains(lower, "compose") {
		return 0
	}
	if ext := filepath.Ext(lower); ext != ".yml" && ext != ".yaml" {
		return 0
	}
	for _, banned := range forbidden {
		if strings.Contains(lower, banned) {
			return 0
		}
	}
	// An override file only makes sense alongside another, never alone.
	if strings.Contains(lower, "override") {
		return 0
	}

	score := 10
	if filepath.Ext(lower) == ".yml" {
		score++ // tie-breaker between the two canonical names
	}
	noExt := strings.TrimSuffix(strings.TrimSuffix(lower, ".yml"), ".yaml")
	switch noExt {
	case "docker-compose":
		score += 44
	case "compose":
		score += 40
	default:
		if strings.Contains(lower, "dev") || strings.Contains(lower, "local") {
			score += 20
		}
	}
	if atRoot {
		score += 100
	}
	return score
}

// Services lists what the project's stack has, up or not.
func Services(dir, file string) ([]Service, error) {
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()

	out, err := run(ctx, dir, file, "ps", "--all", "--format", "json")
	if err != nil {
		return nil, err
	}
	return ReadServices(out)
}

// Summary is what the project's strip shows: how many services are up.
func Summary(services []Service) string {
	if len(services) == 0 {
		return "stopped"
	}
	up := 0
	for _, service := range services {
		if strings.HasPrefix(service.State, "up") {
			up++
		}
	}
	if up == 0 {
		return "stopped"
	}
	return strconv.Itoa(up) + "/" + strconv.Itoa(len(services))
}

// Act runs an action on a service, or on the whole stack when the service
// comes in empty. An unknown action is refused — that's how nothing
// destructive gets in.
func Act(dir, file, action, service string) error {
	var args []string
	switch action {
	case "up":
		args = []string{"up", "-d"}
	case "down":
		args = []string{"stop"}
	case "restart":
		args = []string{"restart"}
	case "rebuild":
		args = []string{"up", "-d", "--build"}
	default:
		return fmt.Errorf("unknown Docker action: %q", action)
	}
	if service != "" {
		args = append(args, service)
	}

	ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
	defer cancel()
	_, err := run(ctx, dir, file, args...)
	return err
}

// LogCommand is the command that a log cell runs to follow a service. It
// lives here so the panel and the cell never disagree.
func LogCommand(file, service string) (string, []string) {
	return "docker", []string{"compose", "--file", file, "logs", "--follow", "--tail", "200", service}
}

// ReadServices parses the output of `docker compose ps --format json`, which
// comes as one object per line.
func ReadServices(out []byte) ([]Service, error) {
	var services []Service
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Some versions deliver the whole array on a single line.
		if strings.HasPrefix(line, "[") {
			var batch []raw
			if err := json.Unmarshal([]byte(line), &batch); err != nil {
				return nil, err
			}
			for _, item := range batch {
				services = append(services, item.translate())
			}
			continue
		}
		var item raw
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, err
		}
		services = append(services, item.translate())
	}
	return services, scanner.Err()
}

// raw is the part of compose's output that matters.
type raw struct {
	Service    string `json:"Service"`
	State      string `json:"State"`
	Status     string `json:"Status"`
	Health     string `json:"Health"`
	ExitCode   int    `json:"ExitCode"`
	Publishers []struct {
		PublishedPort int    `json:"PublishedPort"`
		TargetPort    int    `json:"TargetPort"`
		Protocol      string `json:"Protocol"`
	} `json:"Publishers"`
}

func (b raw) translate() Service {
	return Service{
		Name:   b.Service,
		State:  readableState(b),
		Port:   publishedPort(b),
		Health: health(b),
		Uptime: uptime(b),
	}
}

func readableState(b raw) string {
	switch b.State {
	case "running":
		return "up"
	case "exited":
		return "exited (" + strconv.Itoa(b.ExitCode) + ")"
	case "":
		return "—"
	}
	return b.State
}

// publishedPort returns the first port the service exposes outward.
func publishedPort(b raw) string {
	for _, published := range b.Publishers {
		if published.PublishedPort > 0 {
			return ":" + strconv.Itoa(published.PublishedPort)
		}
	}
	return ""
}

// health reads the healthcheck, from its own field or from the status text.
func health(b raw) string {
	if b.Health != "" {
		return translateHealth(b.Health)
	}
	open := strings.Index(b.Status, "(")
	close_ := strings.Index(b.Status, ")")
	if open >= 0 && close_ > open {
		return translateHealth(b.Status[open+1 : close_])
	}
	return ""
}

func translateHealth(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "healthy":
		return "healthy"
	case "unhealthy":
		return "unhealthy"
	case "starting", "health: starting":
		return "starting"
	}
	return ""
}

// uptime pulls the up-time out of the status, without the "Up" and without
// the health.
func uptime(b raw) string {
	if !strings.HasPrefix(b.Status, "Up") {
		return ""
	}
	text := strings.TrimSpace(strings.TrimPrefix(b.Status, "Up"))
	if open := strings.Index(text, "("); open >= 0 {
		text = strings.TrimSpace(text[:open])
	}
	return shortenTime(text)
}

// shortenTime swaps "2 hours 14 minutes" for "2h14m", which is what fits in
// the column.
func shortenTime(text string) string {
	// Docker writes "Less than a second" for a container that just came up,
	// and there's no number in that to shorten.
	if strings.HasPrefix(strings.ToLower(text), "less than") {
		return "0s"
	}
	parts := strings.Fields(text)
	var short strings.Builder
	for i := 0; i+1 < len(parts); i += 2 {
		number := parts[i]
		unit := parts[i+1]
		switch {
		case strings.HasPrefix(unit, "second"):
			short.WriteString(number + "s")
		case strings.HasPrefix(unit, "minute"):
			short.WriteString(number + "m")
		case strings.HasPrefix(unit, "hour"):
			short.WriteString(number + "h")
		case strings.HasPrefix(unit, "day"):
			short.WriteString(number + "d")
		case strings.HasPrefix(unit, "week"):
			short.WriteString(number + "w")
		case strings.HasPrefix(unit, "month"):
			short.WriteString(number + "mo")
		default:
			// Format we don't know: better to return the text as it came
			// than to scramble the words.
			return text
		}
	}
	if short.Len() == 0 {
		return text
	}
	return short.String()
}

// run runs the project's docker compose and returns the output.
func run(ctx context.Context, dir, file string, args ...string) ([]byte, error) {
	all := append([]string{"compose", "--file", file}, args...)
	cmd := exec.CommandContext(ctx, "docker", all...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("docker compose %s: %s", strings.Join(args, " "), firstLine(exitErr.Stderr))
		}
		return nil, fmt.Errorf("docker compose %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func firstLine(out []byte) string {
	text := strings.TrimSpace(string(out))
	if cut := strings.Index(text, "\n"); cut > 0 {
		return text[:cut]
	}
	return text
}
