package tela

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/motor/historico"
	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

var atualizarGolden = flag.Bool("atualizar", false, "regrava os arquivos de referência do desenho")

// semEstilo tira os códigos de cor para o arquivo de referência ser legível e
// a comparação falhar por conteúdo, não por tom de cinza.
func semEstilo(desenho string) string {
	linhas := strings.Split(desenho, "\n")
	for i, linha := range linhas {
		linhas[i] = strings.TrimRight(historico.LimparCodigos(linha), " ")
	}
	return strings.Join(linhas, "\n")
}

func conferirGolden(t *testing.T, nome, desenho string) {
	t.Helper()
	caminho := filepath.Join("testdata", nome)
	limpo := semEstilo(desenho)
	if *atualizarGolden {
		if err := os.WriteFile(caminho, []byte(limpo), 0o644); err != nil {
			t.Fatalf("gravar referência: %v", err)
		}
		return
	}
	esperado, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("ler referência (rode com -atualizar para criar): %v", err)
	}
	if string(esperado) != limpo {
		t.Errorf("o desenho mudou.\n--- esperado ---\n%s\n--- veio ---\n%s", esperado, limpo)
	}
}

// gradeDeTeste é o mesmo estado usado pelo mosaico e pela lista.
func gradeDeTeste() protocolo.Estado {
	return protocolo.Estado{
		Tipos: []protocolo.TipoCelula{{Tipo: "claude", AceitaPrompt: true}, {Tipo: "bash"}},
		Projetos: []protocolo.Projeto{
			{
				ID: "p1", Nome: "doxar-api", Caminho: "/home/dev/doxar-api", Cor: 0,
				TemCompose: true, Docker: "4/5",
				Celulas: []protocolo.Celula{
					{ID: "c1", Tipo: "claude", Nome: "refatora auth", Estado: "respondeu", AoVivo: true,
						Linhas: []string{"Movi a validação de token", "pro guard.", "Qual você prefere?"}},
					{ID: "c2", Tipo: "bash", Nome: "testes", Estado: "trabalhando", AoVivo: true,
						Linhas: []string{"$ go test ./...", "ok"}},
				},
			},
			{
				ID: "p2", Nome: "cortz-web", Caminho: "/home/dev/cortz-web", Cor: 1,
				Celulas: []protocolo.Celula{
					{ID: "c3", Tipo: "claude", Nome: "fix nav", Estado: "aprovar", AoVivo: true,
						Linhas: []string{"posso mexer no Header?"}},
				},
			},
			{
				ID: "p3", Nome: "api-legado", Caminho: "/home/dev/api-legado", Cor: 2,
				Celulas: []protocolo.Celula{
					{ID: "c4", Tipo: "md", Nome: "spec-m7.md", Estado: "parada", AoVivo: true,
						Linhas: []string{"# Módulo 7"}},
				},
			},
		},
	}
}

// TestMosaicoDesenhaColunaFocadaETiras é a regra de largura: a coluna do
// projeto focado ocupa a largura de leitura, as outras viram tira.
func TestMosaicoDesenhaColunaFocadaETiras(t *testing.T) {
	estado := gradeDeTeste()
	desenho := Desenhar(estado, Foco{Projeto: 0, Celula: 0}, teclado.Navegar, 120, 24, "")
	conferirGolden(t, "mosaico-foco-0.txt", desenho)

	larguras := Dispor(estado, Foco{Projeto: 0}, 120, 24).larguras
	if larguras[1] != larguraTira || larguras[2] != larguraTira {
		t.Fatalf("as colunas não focadas deviam virar tira: %v", larguras)
	}
	if larguras[0] < 60 {
		t.Fatalf("a coluna focada precisa de largura de leitura, tem %d", larguras[0])
	}
	if soma(larguras)+1 != 120 {
		t.Fatalf("as colunas não preenchem a largura: %v", larguras)
	}
}

// TestMudarOFocoMudaQualColunaEngorda — navegar entre projetos é mover para a
// coluna do lado: ela engorda, a atual encolhe.
func TestMudarOFocoMudaQualColunaEngorda(t *testing.T) {
	estado := gradeDeTeste()
	desenho := Desenhar(estado, Foco{Projeto: 1}, teclado.Navegar, 120, 24, "")
	conferirGolden(t, "mosaico-foco-1.txt", desenho)

	larguras := Dispor(estado, Foco{Projeto: 1}, 120, 24).larguras
	if larguras[0] != larguraTira || larguras[2] != larguraTira {
		t.Fatalf("as vizinhas deviam ser tiras: %v", larguras)
	}
	if larguras[1] < 60 {
		t.Fatalf("a coluna focada devia ter engordado: %v", larguras)
	}
	if !strings.Contains(semEstilo(desenho), "CORTZ-WEB") {
		t.Error("o nome do projeto focado tem que aparecer na borda de cima")
	}
	if strings.Contains(semEstilo(desenho), "DOXAR-API ") {
		t.Error("o projeto que virou tira não mostra o nome inteiro")
	}
}

// TestTiraNuncaSome — some o texto, nunca o sinal.
func TestTiraNuncaSome(t *testing.T) {
	estado := gradeDeTeste()
	linhas := strings.Split(semEstilo(Desenhar(estado, Foco{Projeto: 1}, teclado.Navegar, 120, 24, "")), "\n")
	juntas := strings.Join(linhas, "")
	for _, sinal := range []string{"D", "O", "X", "A", "R"} {
		if !strings.Contains(juntas, sinal) {
			t.Errorf("a tira devia mostrar o nome na vertical, falta %q", sinal)
		}
	}
	if !strings.Contains(juntas, "●") {
		t.Error("a tira devia mostrar o indicador de Docker do projeto que tem compose")
	}
	if !strings.Contains(juntas, "⬤1") {
		t.Error("a tira devia mostrar quantas células pedem atenção")
	}
}

// TestMosaicoEmDigitarApagaOResto — três sinais redundantes: barra apagada,
// selo e borda grossa.
func TestMosaicoEmDigitarApagaOResto(t *testing.T) {
	estado := gradeDeTeste()
	desenho := semEstilo(Desenhar(estado, Foco{Projeto: 0, Celula: 1}, teclado.Digitar, 120, 24, ""))
	if !strings.Contains(desenho, "▓ DIGITAR ▓") {
		t.Error("falta o selo de DIGITAR na barra")
	}
	if !strings.Contains(desenho, "┏") || !strings.Contains(desenho, "┃") {
		t.Error("a borda da célula focada devia engrossar")
	}
	if !strings.Contains(desenho, "ctrl-l devolve o teclado") {
		t.Error("o rodapé devia encolher para a linha do ctrl-l")
	}
	if strings.Contains(desenho, "NAVEGAR") {
		t.Error("a barra não pode dizer NAVEGAR em modo DIGITAR")
	}
}

// TestTelaCheiaMostraSoACelulaFocada — é assim que se copia um bloco de texto
// sem pegar os vizinhos.
func TestTelaCheiaMostraSoACelulaFocada(t *testing.T) {
	estado := gradeDeTeste()
	desenho := semEstilo(Desenhar(estado, Foco{Projeto: 0, Celula: 0, Cheia: true}, teclado.Navegar, 120, 24, ""))
	if !strings.Contains(desenho, "refatora auth") {
		t.Error("a célula focada devia estar na tela")
	}
	if strings.Contains(desenho, "go test") {
		t.Error("em tela cheia, a célula vizinha não pode aparecer")
	}
	if strings.Contains(desenho, "CORTZ-WEB") {
		t.Error("em tela cheia, os outros projetos não aparecem")
	}
}

// TestGeometriaBateComODesenho — o tamanho avisado ao motor é o mesmo que a
// tela desenha, senão o processo lá dentro enxerga um terminal torto.
func TestGeometriaBateComODesenho(t *testing.T) {
	estado := gradeDeTeste()
	for _, tamanho := range [][2]int{{120, 24}, {80, 20}, {200, 50}, {60, 12}} {
		largura, altura := tamanho[0], tamanho[1]
		d := Dispor(estado, Foco{Projeto: 0}, largura, altura)
		desenho := strings.Split(Desenhar(estado, Foco{Projeto: 0}, teclado.Navegar, largura, altura, ""), "\n")
		if len(desenho) != altura {
			t.Errorf("%dx%d: o desenho tem %d linhas", largura, altura, len(desenho))
		}
		for _, linha := range desenho {
			if visivel := lipgloss.Width(semEstilo(linha)); visivel > largura {
				t.Errorf("%dx%d: linha com %d colunas: %q", largura, altura, visivel, linha)
			}
		}
		somaAlturas := 0
		for _, miolo := range d.Miolos() {
			somaAlturas += miolo.Linhas + 2
			if miolo.Colunas < 1 || miolo.Linhas < 1 {
				t.Errorf("%dx%d: miolo inválido %#v", largura, altura, miolo)
			}
		}
		if len(d.Miolos()) > 0 && somaAlturas != altura-4 {
			t.Errorf("%dx%d: as células somam %d linhas, o corpo tem %d", largura, altura, somaAlturas, altura-4)
		}
	}
}

// TestMuitosProjetosNaoEspremenAColunaFocada — a tira mais distante do foco sai
// antes de a coluna focada ficar ilegível.
func TestMuitosProjetosNaoEspremenAColunaFocada(t *testing.T) {
	estado := protocolo.Estado{}
	for i := range 30 {
		estado.Projetos = append(estado.Projetos, protocolo.Projeto{
			ID:      "p" + string(rune('a'+i)),
			Nome:    "projeto" + string(rune('a'+i)),
			Celulas: []protocolo.Celula{{ID: "c" + string(rune('a'+i)), Tipo: "bash", Nome: "shell", Estado: "trabalhando", AoVivo: true}},
		})
	}
	larguras := Dispor(estado, Foco{Projeto: 15}, 120, 24).larguras
	if larguras[15] < larguraMinimaDeLeitura {
		t.Fatalf("a coluna focada ficou com %d colunas", larguras[15])
	}
	if soma(larguras)+1 != 120 {
		t.Fatalf("as colunas não preenchem a largura: soma %d", soma(larguras))
	}
	// As tiras que sobraram são as vizinhas do foco.
	if larguras[14] == 0 || larguras[16] == 0 {
		t.Error("as tiras mantidas deviam ser as mais próximas do foco")
	}
	if larguras[0] != 0 {
		t.Error("com projetos demais, a tira mais distante do foco cede lugar")
	}
}

func soma(numeros []int) int {
	total := 0
	for _, n := range numeros {
		total += n
	}
	return total
}
