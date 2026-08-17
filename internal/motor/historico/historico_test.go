package historico

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// TestDescartaComecoMantemFim escreve muito além do teto e confere que o
// arquivo perdeu o começo e guardou o fim.
func TestDescartaComecoMantemFim(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "celula.log")
	const teto int64 = 4096
	h, err := Abrir(caminho, teto)
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	defer h.Fechar()

	if _, err := h.Write([]byte("PRIMEIRA LINHA DE TODAS\n")); err != nil {
		t.Fatalf("escrever: %v", err)
	}
	recheio := bytes.Repeat([]byte("x"), 512)
	for range 40 {
		if _, err := h.Write(append(recheio, '\n')); err != nil {
			t.Fatalf("escrever: %v", err)
		}
	}
	if _, err := h.Write([]byte("ULTIMA LINHA DE TODAS\n")); err != nil {
		t.Fatalf("escrever: %v", err)
	}

	if tam := h.Tamanho(); tam > teto {
		t.Fatalf("histórico ficou com %d bytes, acima do teto de %d", tam, teto)
	}

	if achados, err := h.Buscar("PRIMEIRA"); err != nil {
		t.Fatalf("buscar: %v", err)
	} else if len(achados) != 0 {
		t.Fatalf("o começo devia ter sido descartado, mas ainda está lá: %#v", achados)
	}
	achados, err := h.Buscar("ULTIMA LINHA")
	if err != nil {
		t.Fatalf("buscar: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("o fim tinha que estar preservado, achei %d ocorrências", len(achados))
	}
}

// TestBuscaDevolveLinhaCerta confere o número da linha e a limpeza dos códigos
// de terminal.
func TestBuscaDevolveLinhaCerta(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "celula.log")
	h, err := Abrir(caminho, TetoPadrao)
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	defer h.Fechar()

	if _, err := h.Write([]byte("um\ndois\n\x1b[32mtres tesseract\x1b[0m\nquatro\ncinco TESSERACT\n")); err != nil {
		t.Fatalf("escrever: %v", err)
	}

	achados, err := h.Buscar("tesseract")
	if err != nil {
		t.Fatalf("buscar: %v", err)
	}
	if len(achados) != 2 {
		t.Fatalf("esperava 2 ocorrências, veio %d: %#v", len(achados), achados)
	}
	if achados[0].Linha != 3 || achados[0].Texto != "tres tesseract" {
		t.Fatalf("primeira ocorrência errada: %#v", achados[0])
	}
	if achados[1].Linha != 5 || achados[1].Texto != "cinco TESSERACT" {
		t.Fatalf("segunda ocorrência errada: %#v", achados[1])
	}
}

// TestCaudaReproduzOFim é o que permite a célula renascer com a tela anterior.
func TestCaudaReproduzOFim(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "celula.log")
	h, err := Abrir(caminho, TetoPadrao)
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	defer h.Fechar()

	if _, err := h.Write([]byte("comeco\nfim da tela\n")); err != nil {
		t.Fatalf("escrever: %v", err)
	}
	cauda, err := h.Cauda(12)
	if err != nil {
		t.Fatalf("cauda: %v", err)
	}
	if string(cauda) != "fim da tela\n" {
		t.Fatalf("cauda veio %q", cauda)
	}
}

// TestContinuaOArquivoQueJaExiste garante que reabrir não apaga o passado.
func TestContinuaOArquivoQueJaExiste(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "celula.log")
	h, err := Abrir(caminho, TetoPadrao)
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	if _, err := h.Write([]byte("antes da queda\n")); err != nil {
		t.Fatalf("escrever: %v", err)
	}
	h.Fechar()

	h2, err := Abrir(caminho, TetoPadrao)
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	defer h2.Fechar()
	if _, err := h2.Write([]byte("depois da queda\n")); err != nil {
		t.Fatalf("escrever: %v", err)
	}
	achados, err := h2.Buscar("queda")
	if err != nil {
		t.Fatalf("buscar: %v", err)
	}
	if len(achados) != 2 {
		t.Fatalf("esperava as duas linhas, veio %#v", achados)
	}
}

func TestLimparCodigos(t *testing.T) {
	casos := map[string]string{
		"\x1b[1;32m$ ls\x1b[0m":      "$ ls",
		"\x1b]0;titulo\x07prompt":    "prompt",
		"linha\rsobrescrita":         "linhasobrescrita",
		"acentuação preservada":      "acentuação preservada",
		"\x1b[2J\x1b[Hlimpou a tela": "limpou a tela",
	}
	for bruto, esperado := range casos {
		if veio := LimparCodigos(bruto); veio != esperado {
			t.Errorf("LimparCodigos(%q) = %q, esperado %q", bruto, veio, esperado)
		}
	}
	if strings.Contains(LimparCodigos("\x1b[31mvermelho"), "\x1b") {
		t.Error("sobrou código de terminal no texto limpo")
	}
}

// TestBuscaEmArquivoGrande cobre o caso da fase de acabamento: o histórico
// passou do teto, foi cortado, e a busca continua achando o que ficou — com o
// número de linha certo no que sobrou.
func TestBuscaEmArquivoGrande(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "celula.log")
	const teto int64 = 256 << 10
	h, err := Abrir(caminho, teto)
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	defer h.Fechar()

	// Muito volume, com uma agulha no começo (que vai ser descartada) e outra
	// no fim (que tem que sobreviver).
	if _, err := h.Write([]byte("AGULHA-DO-COMECO\n")); err != nil {
		t.Fatalf("escrever: %v", err)
	}
	linha := strings.Repeat("saída comum de um build ", 8) + "\n"
	for i := range 4000 {
		if i == 2000 {
			if _, err := h.Write([]byte("AGULHA-DO-MEIO no meio do volume\n")); err != nil {
				t.Fatalf("escrever: %v", err)
			}
		}
		if _, err := h.Write([]byte(linha)); err != nil {
			t.Fatalf("escrever: %v", err)
		}
	}
	if _, err := h.Write([]byte("AGULHA-DO-FIM\n")); err != nil {
		t.Fatalf("escrever: %v", err)
	}

	if h.Tamanho() > teto {
		t.Fatalf("o histórico passou do teto: %d", h.Tamanho())
	}

	if achados, err := h.Buscar("AGULHA-DO-COMECO"); err != nil {
		t.Fatalf("buscar: %v", err)
	} else if len(achados) != 0 {
		t.Fatalf("o começo foi descartado, não podia aparecer: %#v", achados)
	}

	achados, err := h.Buscar("AGULHA-DO-FIM")
	if err != nil {
		t.Fatalf("buscar: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("a agulha do fim tinha que ser achada uma vez, veio %d", len(achados))
	}

	// O número da linha bate com a contagem do arquivo como ele está agora.
	total, err := h.Linhas()
	if err != nil {
		t.Fatalf("contar linhas: %v", err)
	}
	if achados[0].Linha < 1 || achados[0].Linha > total {
		t.Fatalf("linha %d fora do arquivo de %d linhas", achados[0].Linha, total)
	}
	if achados[0].Linha != total {
		t.Fatalf("a última linha escrita devia ser a última do arquivo: %d de %d", achados[0].Linha, total)
	}
}

// TestBuscaAtravessandoOCorte — o termo que estava na fronteira do descarte não
// aparece pela metade.
func TestBuscaAtravessandoOCorte(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "celula.log")
	const teto int64 = 8 << 10
	h, err := Abrir(caminho, teto)
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	defer h.Fechar()

	for range 20 {
		if _, err := h.Write([]byte(strings.Repeat("x", 500) + "PALAVRA-CORTADA\n")); err != nil {
			t.Fatalf("escrever: %v", err)
		}
	}
	achados, err := h.Buscar("PALAVRA-CORTADA")
	if err != nil {
		t.Fatalf("buscar: %v", err)
	}
	if len(achados) == 0 {
		t.Fatal("as ocorrências que sobreviveram ao corte tinham que ser achadas")
	}
	for _, achado := range achados {
		if !strings.Contains(achado.Texto, "PALAVRA-CORTADA") {
			t.Fatalf("ocorrência sem o termo: %#v", achado)
		}
	}
}
