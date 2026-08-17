package celula

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// prepararStack monta um projeto com compose de verdade e garante que a stack
// começa parada. Sem Docker na máquina, o teste é pulado.
func prepararStack(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("sem docker nesta máquina")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("o docker não está respondendo")
	}

	dir := t.TempDir()
	compose := "services:\n  web:\n    image: nginx:alpine\n    command: [\"sh\", \"-c\", \"while true; do echo tesseract-log; sleep 1; done\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	t.Cleanup(func() {
		// Parar, nunca derrubar com volume: a regra vale até no teste.
		_ = exec.Command("docker", "compose", "--file", filepath.Join(dir, "docker-compose.yml"), "stop").Run()
		_ = exec.Command("docker", "compose", "--file", filepath.Join(dir, "docker-compose.yml"), "rm", "-f").Run()
	})
	return dir
}

// TestLogsFicaParadaEEngataQuandoOServicoSobe é o comportamento que a
// recuperação depende: a stack está parada, a célula espera, e quando o serviço
// sobe ela começa a mostrar o log sozinha.
func TestLogsFicaParadaEEngataQuandoOServicoSobe(t *testing.T) {
	dir := prepararStack(t)

	celula, err := Nova("logs")
	if err != nil {
		t.Fatalf("fabricar: %v", err)
	}
	if err := celula.Nascer(Config{ID: "c1", Diretorio: dir, Alvo: "web", Colunas: 60, Linhas: 12}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer celula.Matar()

	if !esperarPor(t, 10*time.Second, func() bool { return celula.Estado() == Parada }) {
		t.Fatalf("serviço parado devia deixar a célula parada, está %q", celula.Estado())
	}

	subir := exec.Command("docker", "compose", "--file", filepath.Join(dir, "docker-compose.yml"), "up", "-d")
	subir.Dir = dir
	if saida, err := subir.CombinedOutput(); err != nil {
		t.Skipf("não consegui subir a stack de teste: %v\n%s", err, saida)
	}

	if !esperarPor(t, 30*time.Second, func() bool {
		return strings.Contains(telaDe(celula), "tesseract-log")
	}) {
		t.Fatalf("a célula não engatou sozinha quando o serviço subiu:\nestado %q\n%s", celula.Estado(), telaDe(celula))
	}
	if celula.Estado() != Trabalhando {
		t.Fatalf("com o log correndo, a célula está %q", celula.Estado())
	}
}

// TestLogsSemComposeFalhaClaro — projeto sem compose não tem o que acompanhar.
func TestLogsSemComposeFalhaClaro(t *testing.T) {
	celula, _ := Nova("logs")
	err := celula.Nascer(Config{ID: "c1", Diretorio: t.TempDir(), Alvo: "web", Colunas: 60, Linhas: 12})
	if err == nil {
		t.Fatal("sem compose, a célula de log não pode nascer")
	}
	if !strings.Contains(err.Error(), "compose") {
		t.Fatalf("mensagem pouco clara: %v", err)
	}
}

// TestLogsSemServicoFalhaClaro — log de qual serviço?
func TestLogsSemServicoFalhaClaro(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	celula, _ := Nova("logs")
	if err := celula.Nascer(Config{ID: "c1", Diretorio: dir, Colunas: 60, Linhas: 12}); err == nil {
		t.Fatal("sem serviço, a célula de log não pode nascer")
	}
}

// TestLogsSoLeitura — tecla digitada numa célula de log não vai para lugar
// nenhum.
func TestLogsSoLeitura(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services:\n  web:\n    image: nginx\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	celula, _ := Nova("logs")
	if err := celula.Nascer(Config{ID: "c1", Diretorio: dir, Alvo: "web", Colunas: 60, Linhas: 12}); err != nil {
		t.Skipf("sem docker para subir a célula: %v", err)
	}
	defer celula.Matar()
	if err := celula.Tecla(Toque{Codigo: 'x', Texto: "x"}); err != nil {
		t.Fatalf("célula de log ignora tecla, não devolve erro: %v", err)
	}
}
