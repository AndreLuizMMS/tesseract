package tela

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/tema"
)

// corSelecao é o trecho marcado em vídeo invertido. A cor que o agente escreveu
// some enquanto o trecho está marcado — o que importa ali é onde a marca começa
// e onde termina.
var corSelecao = tema.Pintar(tema.BgVoid, tema.FgDefault)

// Selecao é o trecho marcado com o mouse dentro de uma célula, da âncora (onde
// o botão desceu) até o ponto de agora. As coordenadas são do miolo da célula,
// não da tela: a caixa pode mudar de lugar que o trecho continua o mesmo.
//
// ponytail: a marca vale sobre a janela que está à vista. Rolar o histórico
// antes de arrastar já resolve o caso comum; marcar mais do que cabe na tela
// pede guardar a posição no histórico, e isso é motor, não tela.
type Selecao struct {
	Celula     string
	AncoraX    int
	AncoraY    int
	AtualX     int
	AtualY     int
	Arrastando bool
}

// Vazia é verdadeira quando o mouse desceu e subiu no mesmo lugar: isso é um
// clique, que só escolhe a célula, e não uma marca.
func (s Selecao) Vazia() bool {
	return s.AncoraX == s.AtualX && s.AncoraY == s.AtualY
}

// ordenada põe âncora e ponto atual na ordem da leitura, para arrastar de baixo
// para cima valer tanto quanto de cima para baixo.
func (s Selecao) ordenada() (x1, y1, x2, y2 int) {
	if s.AncoraY < s.AtualY || (s.AncoraY == s.AtualY && s.AncoraX <= s.AtualX) {
		return s.AncoraX, s.AncoraY, s.AtualX, s.AtualY
	}
	return s.AtualX, s.AtualY, s.AncoraX, s.AncoraY
}

// faixa diz quais colunas da linha estão marcadas. Como no terminal, a primeira
// linha vai da âncora até o fim e a última começa na margem — é assim que um
// parágrafo sai inteiro.
func (s Selecao) faixa(linha, colunas int) (de, ate int, tem bool) {
	x1, y1, x2, y2 := s.ordenada()
	if linha < y1 || linha > y2 || colunas <= 0 {
		return 0, 0, false
	}
	de, ate = 0, colunas
	if linha == y1 {
		de = max(x1, 0)
	}
	if linha == y2 {
		ate = min(x2+1, colunas)
	}
	if ate <= de {
		return 0, 0, false
	}
	return de, ate, true
}

// Texto é o que vai para a área de transferência: só as letras, sem cor e sem
// os espaços que sobram no fim de cada linha.
func (s Selecao) Texto(linhas []string, colunas int) string {
	_, y1, _, y2 := s.ordenada()
	var trechos []string
	for linha := y1; linha <= y2; linha++ {
		de, ate, tem := s.faixa(linha, colunas)
		if !tem {
			continue
		}
		texto := ""
		if linha >= 0 && linha < len(linhas) {
			texto = ansi.Cut(ansi.Strip(linhas[linha]), de, ate)
		}
		trechos = append(trechos, strings.TrimRight(texto, " "))
	}
	// Arrastar além da última linha da célula não vale linha em branco no fim.
	return strings.TrimRight(strings.Join(trechos, "\n"), "\n")
}

// Pintar devolve as linhas com o trecho marcado em destaque. As linhas em
// branco dentro da marca também acendem: arrastar por uma área vazia tem que
// mostrar que ela foi marcada.
func (s Selecao) Pintar(linhas []string, colunas int) []string {
	_, _, _, y2 := s.ordenada()
	pintadas := make([]string, max(len(linhas), y2+1))
	copy(pintadas, linhas)

	for i := range pintadas {
		de, ate, tem := s.faixa(i, colunas)
		if !tem {
			continue
		}
		marcado := ansi.Strip(ansi.Cut(pintadas[i], de, ate))
		marcado += strings.Repeat(" ", max(ate-de-lipgloss.Width(marcado), 0))
		pintadas[i] = ansi.Cut(pintadas[i], 0, de) + corSelecao.Render(marcado) + ansi.Cut(pintadas[i], ate, colunas)
	}
	return pintadas
}

// comSelecao devolve o retrato com o trecho marcado pintado na célula dele. A
// marca é da tela e morre aqui: o motor nunca fica sabendo dela.
func comSelecao(estado protocolo.Estado, s Selecao, colunas int) protocolo.Estado {
	for i, projeto := range estado.Projetos {
		for j, celula := range projeto.Celulas {
			if celula.ID != s.Celula {
				continue
			}
			celula.Linhas = s.Pintar(celula.Linhas, colunas)
			celulas := append([]protocolo.Celula(nil), projeto.Celulas...)
			celulas[j] = celula
			projetos := append([]protocolo.Projeto(nil), estado.Projetos...)
			projetos[i].Celulas = celulas
			estado.Projetos = projetos
			return estado
		}
	}
	return estado
}
