package tela

import (
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/teclado"
)

// TestListaEMosaicoContamAMesmaHistoria é a garantia mecânica da regra: a tela
// não decide nada, então lista e mosaico alimentados com o mesmo estado
// mostram os mesmos projetos, as mesmas células e os mesmos marcadores. É
// estruturalmente impossível as duas discordarem.
func TestListaEMosaicoContamAMesmaHistoria(t *testing.T) {
	estado := gradeDeTeste()
	foco := Foco{Projeto: 0, Celula: 0}

	mosaico := semEstilo(Desenhar(estado, foco, teclado.Navegar, 140, 30, ""))
	lista := semEstilo(DesenharLista(estado, foco, teclado.Navegar, 140, 30, ""))

	for _, projeto := range estado.Projetos {
		if !strings.Contains(strings.ToUpper(lista), strings.ToUpper(projeto.Nome)) {
			t.Errorf("a lista não mostra o projeto %q", projeto.Nome)
		}
		for _, celula := range projeto.Celulas {
			if !strings.Contains(lista, celula.Nome) {
				t.Errorf("a lista não mostra a célula %q", celula.Nome)
			}
			if !strings.Contains(lista, celula.Tipo) {
				t.Errorf("a lista não mostra o tipo da célula %q", celula.Nome)
			}
			marcador := marcadorDe(celula.Estado).simbolo
			if !strings.Contains(lista, marcador) {
				t.Errorf("a lista não mostra o marcador %q da célula %q", marcador, celula.Nome)
			}
			if !strings.Contains(mosaico, marcador) && projeto.ID == estado.Projetos[foco.Projeto].ID {
				t.Errorf("o mosaico não mostra o marcador %q da célula %q", marcador, celula.Nome)
			}
		}
	}

	// A célula focada aparece nas duas, e nas duas com o mesmo conteúdo.
	focada := estado.Projetos[0].Celulas[0]
	for _, linha := range focada.Linhas {
		if !strings.Contains(mosaico, linha) {
			t.Errorf("o mosaico não mostra a linha %q da célula focada", linha)
		}
		if !strings.Contains(lista, linha) {
			t.Errorf("a prévia da lista não mostra a linha %q da célula focada", linha)
		}
	}
}

// TestListaMostraDockerDoProjeto — o compose pertence ao projeto, e a lista diz
// isso.
func TestListaMostraDockerDoProjeto(t *testing.T) {
	estado := gradeDeTeste()
	lista := semEstilo(DesenharLista(estado, Foco{}, teclado.Navegar, 140, 30, ""))
	if !strings.Contains(lista, "docker") || !strings.Contains(lista, "4/5") {
		t.Errorf("a lista devia mostrar o estado da stack:\n%s", lista)
	}
}

// TestListaCabeNaTela — nada renderiza torto em terminal apertado.
func TestListaCabeNaTela(t *testing.T) {
	estado := gradeDeTeste()
	for _, tamanho := range [][2]int{{140, 30}, {100, 14}, {80, 10}} {
		largura, altura := tamanho[0], tamanho[1]
		linhas := strings.Split(DesenharLista(estado, Foco{Projeto: 1}, teclado.Navegar, largura, altura, ""), "\n")
		if len(linhas) != altura {
			t.Errorf("%dx%d: a lista tem %d linhas", largura, altura, len(linhas))
		}
		for _, linha := range linhas {
			if visivel := len([]rune(semEstilo(linha))); visivel > largura {
				t.Errorf("%dx%d: linha com %d colunas", largura, altura, visivel)
			}
		}
	}
}

// TestListaGolden fixa o desenho do índice com prévia.
func TestListaGolden(t *testing.T) {
	conferirGolden(t, "lista.txt", DesenharLista(gradeDeTeste(), Foco{Projeto: 0, Celula: 1}, teclado.Navegar, 120, 24, ""))
}
