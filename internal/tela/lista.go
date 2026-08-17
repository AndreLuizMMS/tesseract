package tela

import (
	"strings"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

// larguraDoIndice é a coluna da esquerda da lista: os projetos e suas células.
const larguraDoIndice = 30

// MioloDaPrevia é o espaço que sobra para a prévia ao vivo da célula
// selecionada, à direita do índice.
func MioloDaPrevia(largura, altura int) Geometria {
	return Geometria{
		Colunas: max(largura-larguraDoIndice-3, 1),
		Linhas:  max(altura-alturaDaBarra-3, 1),
	}
}

// DesenharLista é o índice: o mesmo conteúdo do mosaico, sem vídeo, com prévia
// ao vivo da célula selecionada à direita. Serve para varrer muitos projetos e
// para terminal de pouca altura.
func DesenharLista(estado protocolo.Estado, foco Foco, modo teclado.Modo, largura, altura int, erro string) string {
	if len(estado.Projetos) == 0 {
		return telaVazia(modo, largura, altura, erro, estado.Aviso)
	}
	foco = Ajustar(estado, foco)
	corpo := altura - alturaDaBarra - 3

	indice := linhasDoIndice(estado, foco, corpo)
	previa := MioloDaPrevia(largura, altura)

	var celula protocolo.Celula
	if celulas := estado.Projetos[foco.Projeto].Celulas; len(celulas) > 0 {
		celula = celulas[foco.Celula]
	}

	linhas := barraDeTitulo(modo, largura, contarChamados(estado), estado.Quota)
	linhas = append(linhas, corBorda.Render("┌"+strings.Repeat("─", larguraDoIndice)+"┬"+strings.Repeat("─", previa.Colunas)+"┐"))
	cabecalho := corTitulo.Render(" " + celula.Tipo + " · " + celula.Nome)
	for l := range corpo {
		esquerda := ""
		if l < len(indice) {
			esquerda = indice[l]
		}
		direita := ""
		switch {
		case l == 0:
			direita = cabecalho
		case l >= 2 && l-2 < len(celula.Linhas):
			direita = celula.Linhas[l-2]
		}
		linhas = append(linhas,
			corBorda.Render("│")+preencher(esquerda, larguraDoIndice)+
				corBorda.Render("│")+preencher(direita, previa.Colunas)+
				corBorda.Render("│"))
	}
	linhas = append(linhas, corBorda.Render("└"+strings.Repeat("─", larguraDoIndice)+"┴"+strings.Repeat("─", previa.Colunas)+"┘"))
	linhas = append(linhas, rodape(modo, largura, erro))
	return strings.Join(linhas, "\n")
}

// linhasDoIndice monta a coluna da esquerda: projeto, células e Docker.
func linhasDoIndice(estado protocolo.Estado, foco Foco, altura int) []string {
	var linhas []string
	var linhaDoFoco int
	for i, projeto := range estado.Projetos {
		if i > 0 {
			linhas = append(linhas, "")
		}
		caminho := encurtarCaminho(projeto.Caminho, larguraDoIndice-len(projeto.Nome)-3)
		linhas = append(linhas, corProjeto(projeto.Cor).Render(" "+strings.ToUpper(projeto.Nome))+" "+corApagada.Render(caminho))
		for j, celula := range projeto.Celulas {
			marca := "  "
			if i == foco.Projeto && j == foco.Celula {
				marca = corBordaFocada.Render("▸ ")
				linhaDoFoco = len(linhas)
			}
			marcador := marcadorDe(celula.Estado)
			linhas = append(linhas, marca+marcador.cor().Render(marcador.simbolo)+" "+
				corBarra.Render(preencher(celula.Tipo, 7))+corApagada.Render(cortar(celula.Nome, larguraDoIndice-13)))
		}
		if projeto.TemCompose {
			estadoDocker := projeto.Docker
			if estadoDocker == "" {
				estadoDocker = "parado"
			}
			linhas = append(linhas, "  "+corApagada.Render("● "+preencher("docker", 7)+estadoDocker))
		}
	}
	return janela(linhas, linhaDoFoco, altura)
}

// janela corta a lista para caber na altura, mantendo o foco visível.
func janela(linhas []string, foco, altura int) []string {
	if len(linhas) <= altura {
		return linhas
	}
	primeira := min(max(foco-altura/2, 0), len(linhas)-altura)
	return linhas[primeira : primeira+altura]
}

// encurtarCaminho troca a casa por ~ e corta pela esquerda quando não cabe.
func encurtarCaminho(caminho string, largura int) string {
	if largura < 4 {
		return ""
	}
	curto := caminho
	if casa := lar(); casa != "" && strings.HasPrefix(curto, casa) {
		curto = "~" + strings.TrimPrefix(curto, casa)
	}
	if len(curto) <= largura {
		return curto
	}
	// Corta numa barra, para o pedaço que sobra continuar sendo um caminho.
	sobra := curto[len(curto)-largura+1:]
	if barra := strings.Index(sobra, "/"); barra >= 0 {
		sobra = sobra[barra:]
	}
	return "…" + sobra
}
