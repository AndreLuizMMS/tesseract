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

// TestUptimeDeContainerRecemSubido — "Less than a second" não pode virar
// palavra embaralhada na coluna.
func TestUptimeDeContainerRecemSubido(t *testing.T) {
	saida := `{"Service":"web","State":"running","Status":"Up Less than a second","ExitCode":0}
{"Service":"api","State":"running","Status":"Up About a minute","ExitCode":0}`
	servicos, err := LerServicos([]byte(saida))
	if err != nil {
		t.Fatalf("ler: %v", err)
	}
	if servicos[0].Uptime != "0s" {
		t.Fatalf("recém-subido devia virar \"0s\", veio %q", servicos[0].Uptime)
	}
	if servicos[1].Uptime != "About a minute" {
		t.Fatalf("formato desconhecido volta como veio, veio %q", servicos[1].Uptime)
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

// TestDetectarSegueAOrdem — na raiz, o nome canônico ganha da variante.
func TestDetectarSegueAOrdem(t *testing.T) {
	dir := t.TempDir()
	if Detectar(dir) != "" {
		t.Fatal("projeto sem compose não pode ganhar painel")
	}

	criar := func(caminho string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
			t.Fatalf("preparar: %v", err)
		}
		if err := os.WriteFile(caminho, []byte("services: {}\n"), 0o644); err != nil {
			t.Fatalf("preparar: %v", err)
		}
	}

	criar(filepath.Join(dir, "compose.yaml"))
	if achado := Detectar(dir); achado != filepath.Join(dir, "compose.yaml") {
		t.Fatalf("achou %q", achado)
	}
	criar(filepath.Join(dir, "docker-compose.yml"))
	if achado := Detectar(dir); achado != filepath.Join(dir, "docker-compose.yml") {
		t.Fatalf("o nome canônico devia ganhar, achou %q", achado)
	}
}

// TestDetectarAchaEmSubpasta é o caso do mundo real: a stack mora em `docker/`.
func TestDetectarAchaEmSubpasta(t *testing.T) {
	dir := t.TempDir()
	dentro := filepath.Join(dir, "docker", "compose.yml")
	if err := os.MkdirAll(filepath.Dir(dentro), 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if err := os.WriteFile(dentro, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if achado := Detectar(dir); achado != dentro {
		t.Fatalf("compose em subpasta devia ser achado, veio %q", achado)
	}
}

// TestDetectarNuncaEscolheProducao — o painel é de desenvolvimento.
func TestDetectarNuncaEscolheProducao(t *testing.T) {
	dir := t.TempDir()
	pasta := filepath.Join(dir, "docker")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	for _, nome := range []string{"compose.prod.yml", "docker-compose-prod.yml", "compose.production.yaml", "compose.staging.yml"} {
		if err := os.WriteFile(filepath.Join(pasta, nome), []byte("services: {}\n"), 0o644); err != nil {
			t.Fatalf("preparar: %v", err)
		}
	}
	if achado := Detectar(dir); achado != "" {
		t.Fatalf("só havia arquivo de produção, mas escolheu %q", achado)
	}

	bom := filepath.Join(pasta, "compose.yml")
	if err := os.WriteFile(bom, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if achado := Detectar(dir); achado != bom {
		t.Fatalf("com o de desenvolvimento ao lado, devia escolher ele: %q", achado)
	}
}

// TestDetectarPrefereDevEntreVariantes — `docker-compose-dev.yml` ao lado do de
// produção é o arranjo comum.
func TestDetectarPrefereDevEntreVariantes(t *testing.T) {
	dir := t.TempDir()
	for _, nome := range []string{"docker-compose-prod.yml", "docker-compose-dev.yml"} {
		if err := os.WriteFile(filepath.Join(dir, nome), []byte("services: {}\n"), 0o644); err != nil {
			t.Fatalf("preparar: %v", err)
		}
	}
	if achado := Detectar(dir); achado != filepath.Join(dir, "docker-compose-dev.yml") {
		t.Fatalf("devia escolher o de desenvolvimento, veio %q", achado)
	}
}

// TestDetectarPrefereARaiz — compose na raiz ganha do que está em subpasta.
func TestDetectarPrefereARaiz(t *testing.T) {
	dir := t.TempDir()
	raiz := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(raiz, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	dentro := filepath.Join(dir, "infra", "docker-compose.yml")
	if err := os.MkdirAll(filepath.Dir(dentro), 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if err := os.WriteFile(dentro, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if achado := Detectar(dir); achado != raiz {
		t.Fatalf("a raiz devia ganhar, veio %q", achado)
	}
}

// TestDetectarIgnoraPastaPesada — nada de vasculhar node_modules.
func TestDetectarIgnoraPastaPesada(t *testing.T) {
	dir := t.TempDir()
	dentro := filepath.Join(dir, "node_modules", "algum-pacote-docker-compose.yml")
	if err := os.MkdirAll(filepath.Dir(dentro), 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if err := os.WriteFile(dentro, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if achado := Detectar(dir); achado != "" {
		t.Fatalf("não podia olhar dentro de node_modules, achou %q", achado)
	}
}

// TestDetectarNaoVaiFundoDemais — dois níveis abaixo já é longe demais.
func TestDetectarNaoVaiFundoDemais(t *testing.T) {
	dir := t.TempDir()
	fundo := filepath.Join(dir, "infra", "local", "docker-compose.yml")
	if err := os.MkdirAll(filepath.Dir(fundo), 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if err := os.WriteFile(fundo, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if achado := Detectar(dir); achado != "" {
		t.Fatalf("compose a dois níveis não conta, achou %q", achado)
	}
}

// TestDetectarIgnoraOverrideSozinho — sobreposição só vale acompanhada.
func TestDetectarIgnoraOverrideSozinho(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.override.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if achado := Detectar(dir); achado != "" {
		t.Fatalf("override sozinho não é stack, achou %q", achado)
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
