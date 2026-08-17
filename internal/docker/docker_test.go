package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func lerExemplo(t *testing.T, nome string) []byte {
	t.Helper()
	conteudo, err := os.ReadFile(filepath.Join("exemplos", nome))
	if err != nil {
		t.Fatalf("ler exemplo: %v", err)
	}
	return conteudo
}

// TestLerServicos cobre os quatro casos que aparecem na vida real: serviço de
// pé e saudável, de pé sem healthcheck, morto com código de saída, e subindo
// sem porta publicada.
func TestLerServicos(t *testing.T) {
	servicos, err := LerServicos(lerExemplo(t, "ps.ndjson"))
	if err != nil {
		t.Fatalf("ler: %v", err)
	}
	if len(servicos) != 4 {
		t.Fatalf("esperava 4 serviços, veio %d", len(servicos))
	}

	esperado := []Servico{
		{Nome: "api", Estado: "up", Porta: ":3000", Saude: "saudável", Uptime: "2h"},
		{Nome: "redis", Estado: "up", Porta: ":6379", Saude: "", Uptime: "2h14m"},
		{Nome: "worker", Estado: "exited (1)", Porta: "", Saude: "", Uptime: ""},
		{Nome: "minio", Estado: "up", Porta: "", Saude: "subindo", Uptime: "10s"},
	}
	for i, quero := range esperado {
		if servicos[i] != quero {
			t.Errorf("serviço %d veio %#v, esperado %#v", i, servicos[i], quero)
		}
	}
}

// TestLerServicosEmArray cobre as versões do compose que entregam um array só.
func TestLerServicosEmArray(t *testing.T) {
	servicos, err := LerServicos(lerExemplo(t, "ps-array.json"))
	if err != nil {
		t.Fatalf("ler: %v", err)
	}
	if len(servicos) != 1 || servicos[0].Nome != "solo" || servicos[0].Porta != ":8080" {
		t.Fatalf("array não foi entendido: %#v", servicos)
	}
}

// TestLerServicosVazio — stack nunca subida devolve nada, sem erro.
func TestLerServicosVazio(t *testing.T) {
	servicos, err := LerServicos([]byte("\n\n"))
	if err != nil {
		t.Fatalf("saída vazia não é erro: %v", err)
	}
	if len(servicos) != 0 {
		t.Fatalf("esperava nada, veio %#v", servicos)
	}
}

func TestResumo(t *testing.T) {
	servicos, _ := LerServicos(lerExemplo(t, "ps.ndjson"))
	if veio := Resumo(servicos); veio != "3/4" {
		t.Fatalf("resumo veio %q, esperado \"3/4\"", veio)
	}
	if veio := Resumo(nil); veio != "parado" {
		t.Fatalf("stack vazia é \"parado\", veio %q", veio)
	}
	parados := []Servico{{Nome: "a", Estado: "exited (0)"}}
	if veio := Resumo(parados); veio != "parado" {
		t.Fatalf("tudo parado é \"parado\", veio %q", veio)
	}
}

// TestDetectarSegueAOrdem — só a raiz do projeto, e na ordem declarada.
func TestDetectarSegueAOrdem(t *testing.T) {
	dir := t.TempDir()
	if Detectar(dir) != "" {
		t.Fatal("projeto sem compose não pode ganhar painel")
	}

	criar := func(nome string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, nome), []byte("services: {}\n"), 0o644); err != nil {
			t.Fatalf("preparar: %v", err)
		}
	}

	criar("compose.yaml")
	if achado := Detectar(dir); achado != filepath.Join(dir, "compose.yaml") {
		t.Fatalf("achou %q", achado)
	}
	criar("compose.yml")
	if achado := Detectar(dir); achado != filepath.Join(dir, "compose.yml") {
		t.Fatalf("achou %q", achado)
	}
	criar("docker-compose.yaml")
	if achado := Detectar(dir); achado != filepath.Join(dir, "docker-compose.yaml") {
		t.Fatalf("achou %q", achado)
	}
	criar("docker-compose.yml")
	if achado := Detectar(dir); achado != filepath.Join(dir, "docker-compose.yml") {
		t.Fatalf("achou %q", achado)
	}
}

// TestDetectarNaoBuscaRecursivo — compose numa subpasta não conta.
func TestDetectarNaoBuscaRecursivo(t *testing.T) {
	dir := t.TempDir()
	fundo := filepath.Join(dir, "infra", "local")
	if err := os.MkdirAll(fundo, 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fundo, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if achado := Detectar(dir); achado != "" {
		t.Fatalf("compose fora da raiz não conta, mas achou %q", achado)
	}
}

// TestDetectarIgnoraDiretorio — uma pasta chamada docker-compose.yml não é um
// arquivo de compose.
func TestDetectarIgnoraDiretorio(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "docker-compose.yml"), 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if achado := Detectar(dir); achado != "" {
		t.Fatalf("diretório não é compose, mas achou %q", achado)
	}
}

// TestNadaDestrutivo — a lista de ações é fechada, e o que derruba volume não
// está nela.
func TestNadaDestrutivo(t *testing.T) {
	for _, proibida := range []string{"down", "rm", "apaga", "kill", "down -v", "prune"} {
		if err := Agir(t.TempDir(), "docker-compose.yml", proibida, ""); err == nil {
			t.Errorf("a ação %q não pode ser aceita", proibida)
		}
	}
}

// TestComandoDeLogNaoSegueOutroServico garante que a célula de log acompanha
// exatamente o serviço pedido.
func TestComandoDeLogNaoSegueOutroServico(t *testing.T) {
	programa, argumentos := ComandoDeLog("/dev/cortz/docker-compose.yml", "worker")
	if programa != "docker" {
		t.Fatalf("programa veio %q", programa)
	}
	juntos := ""
	for _, arg := range argumentos {
		juntos += arg + " "
	}
	esperado := "compose --file /dev/cortz/docker-compose.yml logs --follow --tail 200 worker "
	if juntos != esperado {
		t.Fatalf("comando veio %q, esperado %q", juntos, esperado)
	}
}
