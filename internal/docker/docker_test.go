package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func readExample(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("examples", name))
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	return content
}

// TestReadServices covers the four cases that show up in real life: a
// service up and healthy, up with no healthcheck, dead with an exit code,
// and coming up with no published port.
func TestReadServices(t *testing.T) {
	services, err := ReadServices(readExample(t, "ps.ndjson"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(services) != 4 {
		t.Fatalf("expected 4 services, got %d", len(services))
	}

	want := []Service{
		{Name: "api", State: "up", Port: ":3000", Health: "healthy", Uptime: "2h"},
		{Name: "redis", State: "up", Port: ":6379", Health: "", Uptime: "2h14m"},
		{Name: "worker", State: "exited (1)", Port: "", Health: "", Uptime: ""},
		{Name: "minio", State: "up", Port: "", Health: "starting", Uptime: "10s"},
	}
	for i, w := range want {
		if services[i] != w {
			t.Errorf("service %d came as %#v, expected %#v", i, services[i], w)
		}
	}
}

// TestReadServicesInArray covers compose versions that deliver a single
// array.
func TestReadServicesInArray(t *testing.T) {
	services, err := ReadServices(readExample(t, "ps-array.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(services) != 1 || services[0].Name != "solo" || services[0].Port != ":8080" {
		t.Fatalf("array wasn't parsed correctly: %#v", services)
	}
}

// TestReadServicesEmpty — a stack never brought up returns nothing, no
// error.
func TestReadServicesEmpty(t *testing.T) {
	services, err := ReadServices([]byte("\n\n"))
	if err != nil {
		t.Fatalf("empty output isn't an error: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected nothing, got %#v", services)
	}
}

// TestUptimeOfFreshlyStartedContainer — "Less than a second" can't turn
// into a scrambled word in the column.
func TestUptimeOfFreshlyStartedContainer(t *testing.T) {
	out := `{"Service":"web","State":"running","Status":"Up Less than a second","ExitCode":0}
{"Service":"api","State":"running","Status":"Up About a minute","ExitCode":0}`
	services, err := ReadServices([]byte(out))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if services[0].Uptime != "0s" {
		t.Fatalf("just-started should become \"0s\", got %q", services[0].Uptime)
	}
	if services[1].Uptime != "About a minute" {
		t.Fatalf("unknown format should come back as-is, got %q", services[1].Uptime)
	}
}

func TestSummary(t *testing.T) {
	services, _ := ReadServices(readExample(t, "ps.ndjson"))
	if got := Summary(services); got != "3/4" {
		t.Fatalf("summary came %q, expected \"3/4\"", got)
	}
	if got := Summary(nil); got != "stopped" {
		t.Fatalf("empty stack is \"stopped\", got %q", got)
	}
	stopped := []Service{{Name: "a", State: "exited (0)"}}
	if got := Summary(stopped); got != "stopped" {
		t.Fatalf("everything stopped is \"stopped\", got %q", got)
	}
}

// TestDetectFollowsOrder — at the root, the canonical name beats a variant.
func TestDetectFollowsOrder(t *testing.T) {
	dir := t.TempDir()
	if Detect(dir) != "" {
		t.Fatal("a project with no compose can't win the panel")
	}

	create := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(path, []byte("services: {}\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	create(filepath.Join(dir, "compose.yaml"))
	if found := Detect(dir); found != filepath.Join(dir, "compose.yaml") {
		t.Fatalf("found %q", found)
	}
	create(filepath.Join(dir, "docker-compose.yml"))
	if found := Detect(dir); found != filepath.Join(dir, "docker-compose.yml") {
		t.Fatalf("the canonical name should win, found %q", found)
	}
}

// TestDetectFindsInSubfolder is the real-world case: the stack lives in
// `docker/`.
func TestDetectFindsInSubfolder(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "docker", "compose.yml")
	if err := os.MkdirAll(filepath.Dir(inner), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(inner, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if found := Detect(dir); found != inner {
		t.Fatalf("a compose file in a subfolder should be found, got %q", found)
	}
}

// TestDetectNeverChoosesProduction — the panel is for development.
func TestDetectNeverChoosesProduction(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "docker")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	for _, name := range []string{"compose.prod.yml", "docker-compose-prod.yml", "compose.production.yaml", "compose.staging.yml"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte("services: {}\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	if found := Detect(dir); found != "" {
		t.Fatalf("only a production file existed, but it chose %q", found)
	}

	good := filepath.Join(folder, "compose.yml")
	if err := os.WriteFile(good, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if found := Detect(dir); found != good {
		t.Fatalf("with a development one alongside, it should pick it: %q", found)
	}
}

// TestDetectPrefersDevAmongVariants — `docker-compose-dev.yml` next to the
// production one is the common arrangement.
func TestDetectPrefersDevAmongVariants(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"docker-compose-prod.yml", "docker-compose-dev.yml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("services: {}\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	if found := Detect(dir); found != filepath.Join(dir, "docker-compose-dev.yml") {
		t.Fatalf("should pick the development one, got %q", found)
	}
}

// TestDetectPrefersRoot — compose at the root beats one in a subfolder.
func TestDetectPrefersRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(root, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	inner := filepath.Join(dir, "infra", "docker-compose.yml")
	if err := os.MkdirAll(filepath.Dir(inner), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(inner, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if found := Detect(dir); found != root {
		t.Fatalf("the root should win, got %q", found)
	}
}

// TestDetectIgnoresHeavyFolder — no rummaging through node_modules.
func TestDetectIgnoresHeavyFolder(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "node_modules", "some-package-docker-compose.yml")
	if err := os.MkdirAll(filepath.Dir(inner), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(inner, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if found := Detect(dir); found != "" {
		t.Fatalf("shouldn't have looked inside node_modules, found %q", found)
	}
}

// TestDetectDoesNotGoTooDeep — two levels down is already too deep.
func TestDetectDoesNotGoTooDeep(t *testing.T) {
	dir := t.TempDir()
	deep := filepath.Join(dir, "infra", "local", "docker-compose.yml")
	if err := os.MkdirAll(filepath.Dir(deep), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(deep, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if found := Detect(dir); found != "" {
		t.Fatalf("compose two levels down doesn't count, found %q", found)
	}
}

// TestDetectIgnoresOverrideAlone — an override only counts alongside
// another file.
func TestDetectIgnoresOverrideAlone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.override.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if found := Detect(dir); found != "" {
		t.Fatalf("an override alone isn't a stack, found %q", found)
	}
}

// TestNothingDestructive — the list of actions is closed, and anything that
// tears down a volume isn't in it.
func TestNothingDestructive(t *testing.T) {
	for _, forbidden := range []string{"down", "rm", "delete", "kill", "down -v", "prune"} {
		if err := Act(t.TempDir(), "docker-compose.yml", forbidden, ""); err == nil {
			t.Errorf("the action %q must not be accepted", forbidden)
		}
	}
}

// TestLogCommandDoesNotFollowAnotherService guarantees the log cell follows
// exactly the requested service.
func TestLogCommandDoesNotFollowAnotherService(t *testing.T) {
	program, args := LogCommand("/dev/cortz/docker-compose.yml", "worker")
	if program != "docker" {
		t.Fatalf("program came as %q", program)
	}
	joined := ""
	for _, arg := range args {
		joined += arg + " "
	}
	want := "compose --file /dev/cortz/docker-compose.yml logs --follow --tail 200 worker "
	if joined != want {
		t.Fatalf("command came %q, expected %q", joined, want)
	}
}
