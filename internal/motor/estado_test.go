package motor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSalvarEhAtomico confere que a gravação não deixa rastro pela metade e que
// o arquivo final é sempre legível.
func TestSalvarEhAtomico(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "estado.json")

	estado := EstadoDesejado{
		Tela: "mosaico",
		Projetos: []ProjetoSalvo{{
			ID: "p1", Caminho: "/dev/cortz",
			Celulas: []CelulaSalva{{ID: "c1", Tipo: "bash", Nome: "testes"}},
		}},
	}
	for range 5 {
		if err := Salvar(caminho, estado); err != nil {
			t.Fatalf("salvar: %v", err)
		}
	}

	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ler diretório: %v", err)
	}
	for _, entrada := range entradas {
		if entrada.Name() != "estado.json" {
			t.Fatalf("sobrou arquivo temporário depois de salvar: %s", entrada.Name())
		}
	}

	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("ler estado: %v", err)
	}
	var conferencia EstadoDesejado
	if err := json.Unmarshal(conteudo, &conferencia); err != nil {
		t.Fatalf("estado gravado ficou ilegível: %v", err)
	}

	volta, err := Carregar(caminho)
	if err != nil {
		t.Fatalf("carregar: %v", err)
	}
	if len(volta.Projetos) != 1 || volta.Projetos[0].Celulas[0].Nome != "testes" || volta.Tela != "mosaico" {
		t.Fatalf("estado voltou diferente: %#v", volta)
	}
}

// TestCarregarArquivoIlegivelPreservaOriginal cobre o pior caso: o arquivo
// virou lixo. Nada é perdido em silêncio.
func TestCarregarArquivoIlegivelPreservaOriginal(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "estado.json")
	lixo := []byte("{isso não é json")
	if err := os.WriteFile(caminho, lixo, 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	estado, err := Carregar(caminho)
	if err == nil {
		t.Fatal("carregar arquivo corrompido tinha que devolver erro")
	}
	if len(estado.Projetos) != 0 {
		t.Fatalf("nada devia ter sido carregado: %#v", estado)
	}

	preservado, erroLeitura := os.ReadFile(caminho + ".corrompido")
	if erroLeitura != nil {
		t.Fatalf("o original não foi preservado: %v", erroLeitura)
	}
	if string(preservado) != string(lixo) {
		t.Fatalf("a preservação mudou o conteúdo: %q", preservado)
	}
	if !strings.Contains(err.Error(), ".corrompido") {
		t.Fatalf("o erro precisa dizer onde o original ficou: %v", err)
	}
}

// TestCarregarParcialAproveitaOQueDa é a queda mais comum: um projeto quebrado
// no meio de vários bons.
func TestCarregarParcialAproveitaOQueDa(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "estado.json")
	misturado := `{"tela":"lista","projetos":[
		{"id":"p1","caminho":"/dev/bom","celulas":[{"id":"c1","tipo":"bash","nome":"testes"}]},
		{"id":"p2","caminho":42},
		{"id":"p3","caminho":"/dev/outro","celulas":[]}
	]}`
	if err := os.WriteFile(caminho, []byte(misturado), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	estado, err := Carregar(caminho)
	if err == nil {
		t.Fatal("descartar um projeto tinha que devolver erro")
	}
	if len(estado.Projetos) != 2 {
		t.Fatalf("os projetos bons deviam ter sobrado, veio %#v", estado.Projetos)
	}
	if estado.Tela != "lista" {
		t.Fatalf("a tela escolhida se perdeu: %q", estado.Tela)
	}
	if _, err := os.Stat(caminho + ".corrompido"); err != nil {
		t.Fatalf("o original não foi preservado: %v", err)
	}
}

// TestCarregarSemArquivo é a primeira execução da vida: grade vazia, sem erro.
func TestCarregarSemArquivo(t *testing.T) {
	estado, err := Carregar(filepath.Join(t.TempDir(), "estado.json"))
	if err != nil {
		t.Fatalf("primeira execução não pode falhar: %v", err)
	}
	if len(estado.Projetos) != 0 {
		t.Fatalf("grade devia nascer vazia: %#v", estado)
	}
}

// TestPreservarNaoSobrescreve garante que duas corrupções seguidas não apagam a
// primeira evidência.
func TestPreservarNaoSobrescreve(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "estado.json")
	for i := range 2 {
		if err := os.WriteFile(caminho, []byte("lixo"), 0o644); err != nil {
			t.Fatalf("preparar: %v", err)
		}
		if _, err := Carregar(caminho); err == nil {
			t.Fatalf("rodada %d: esperava erro", i)
		}
	}
	if _, err := os.Stat(caminho + ".corrompido"); err != nil {
		t.Fatalf("primeira preservação sumiu: %v", err)
	}
	if _, err := os.Stat(caminho + ".corrompido-1"); err != nil {
		t.Fatalf("segunda preservação não foi criada: %v", err)
	}
}
