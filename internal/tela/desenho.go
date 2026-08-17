package tela

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

// Em DIGITAR o aplicativo fica mudo e mostra que está mudo: barra e tiras
// apagam, o selo aparece e a borda da célula focada engrossa.
var (
	corApagada     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	corBarra       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	corSelo        = lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(lipgloss.Color("214")).Bold(true)
	corTitulo      = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
	corBorda       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	corBordaFocada = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	corRolagem     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	corErro        = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	corQuota       = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	corQuotaAlta   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

// paletaDeProjetos dá a cada coluna uma cor própria, para o olho achar o
// projeto sem ler o nome.
var paletaDeProjetos = []lipgloss.Style{
	lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("141")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("80")),
}

func corProjeto(cor int) lipgloss.Style {
	if cor < 0 {
		cor = 0
	}
	return paletaDeProjetos[cor%len(paletaDeProjetos)]
}

type marcador struct {
	simbolo string
	cor     lipgloss.Style
}

// marcadores traduz o estado da célula no símbolo e na cor do marcador.
var marcadores = map[string]marcador{
	"trabalhando": {"▸", lipgloss.NewStyle().Foreground(lipgloss.Color("39"))},
	"respondeu":   {"⬤", lipgloss.NewStyle().Foreground(lipgloss.Color("42"))},
	"aprovar":     {"⏵", lipgloss.NewStyle().Foreground(lipgloss.Color("214"))},
	"caiu":        {"✖", lipgloss.NewStyle().Foreground(lipgloss.Color("203"))},
	"parada":      {"○", lipgloss.NewStyle().Foreground(lipgloss.Color("244"))},
	"orfa":        {"⚠", lipgloss.NewStyle().Foreground(lipgloss.Color("208"))},
}

func marcadorDe(estado string) marcador {
	if achado, existe := marcadores[estado]; existe {
		return achado
	}
	return marcadores["parada"]
}

// MioloDaCelulaCheia é o espaço útil de uma célula ocupando a tela toda:
// desconta a barra de título, o rodapé e as bordas da caixa.
func MioloDaCelulaCheia(largura, altura int) Geometria {
	return Geometria{Colunas: max(largura-2, 1), Linhas: max(altura-4, 1)}
}

// DesenharCheia põe uma célula na tela inteira. É como se copia um bloco de
// texto sem pegar os vizinhos, e é o que a tecla de tela cheia faz.
func DesenharCheia(projeto protocolo.Projeto, celula protocolo.Celula, modo teclado.Modo, largura, altura int, erro string) string {
	linhas := []string{barraDeTitulo(modo, largura, contarChamadosDoProjeto(projeto), nil)}
	linhas = append(linhas, caixaDaCelula(celula, modo, true, largura, altura-2)...)
	linhas = append(linhas, rodape(modo, largura, erro))
	return strings.Join(linhas, "\n")
}

// barraDeTitulo é o primeiro dos três sinais redundantes do modo.
func barraDeTitulo(modo teclado.Modo, largura int, chamados map[string]int, quota *protocolo.Quota) string {
	nome := "TESSERACT"
	pintarNome, pintarSinais := corTitulo, corBarra
	if modo == teclado.Digitar {
		nome, pintarNome, pintarSinais = "tesseract", corApagada, corApagada
	}

	esquerda := " " + pintarNome.Render(nome)
	var sinais []string
	for _, estado := range []string{"respondeu", "aprovar"} {
		if chamados[estado] > 0 {
			sinais = append(sinais, marcadorDe(estado).simbolo+" "+strconv.Itoa(chamados[estado]))
		}
	}
	if len(sinais) > 0 {
		esquerda += "   " + pintarSinais.Render(strings.Join(sinais, "   "))
	}

	meio := pintarSinais.Render(modo.String())
	if modo == teclado.Digitar {
		meio = corSelo.Render(" ▓ DIGITAR ▓ ")
	}

	direita := ""
	if quota != nil && modo != teclado.Digitar {
		pintar := corQuota
		if quota.Percentual >= 80 {
			pintar = corQuotaAlta
		}
		direita = pintar.Render("⏳ " + strconv.Itoa(quota.Percentual) + "% " + quota.Vira)
	}
	return tresPartes(esquerda, meio, direita, largura)
}

// rodape mostra o que dá para fazer agora. Em DIGITAR encolhe para uma linha
// só, que é o terceiro sinal redundante do modo.
func rodape(modo teclado.Modo, largura int, erro string) string {
	if erro != "" {
		return preencher(corErro.Render(" "+cortar(erro, largura-1)), largura)
	}
	if modo == teclado.Digitar {
		return preencher(corApagada.Render(" ctrl-l devolve o teclado"), largura)
	}
	var dicas []string
	for _, atalho := range teclado.Atalhos(modo) {
		if !atalho.Rodape {
			continue
		}
		dicas = append(dicas, corBarra.Render(atalho.Curto))
	}
	return preencher(" "+strings.Join(dicas, "   "), largura)
}

// telaVazia é o que aparece quando não há nenhuma célula na grade.
func telaVazia(modo teclado.Modo, largura, altura int, erro, aviso string) string {
	linhas := []string{barraDeTitulo(modo, largura, nil, nil)}
	recado := "nenhuma célula na tela — n cria a primeira"
	if aviso != "" {
		recado = aviso
	}
	corpo := max(altura-2, 1)
	for i := range corpo {
		if i == corpo/2 {
			linhas = append(linhas, preencher(centralizarEm(corApagada.Render(recado), largura), largura))
			continue
		}
		linhas = append(linhas, strings.Repeat(" ", largura))
	}
	return strings.Join(append(linhas, rodape(modo, largura, erro)), "\n")
}

// contarChamados soma, na grade inteira, quantas células estão pedindo atenção.
func contarChamados(estado protocolo.Estado) map[string]int {
	contas := map[string]int{}
	for _, projeto := range estado.Projetos {
		for _, celula := range projeto.Celulas {
			contas[celula.Estado]++
		}
	}
	return contas
}

func contarChamadosDoProjeto(projeto protocolo.Projeto) map[string]int {
	contas := map[string]int{}
	for _, celula := range projeto.Celulas {
		contas[celula.Estado]++
	}
	return contas
}

// tresPartes espalha esquerda, meio e direita numa linha só.
func tresPartes(esquerda, meio, direita string, largura int) string {
	le, lm, ld := lipgloss.Width(esquerda), lipgloss.Width(meio), lipgloss.Width(direita)
	if le+lm+ld+2 > largura {
		return preencher(esquerda+" "+meio, largura)
	}
	antes := (largura-lm)/2 - le
	depois := largura - le - antes - lm - ld - 1
	if antes < 1 {
		antes = 1
	}
	if depois < 1 {
		depois = 1
	}
	linha := esquerda + strings.Repeat(" ", antes) + meio + strings.Repeat(" ", depois) + direita
	return preencher(linha, largura)
}

// preencher completa a linha com espaços até a largura, cortando o que sobrar.
func preencher(texto string, largura int) string {
	atual := lipgloss.Width(texto)
	switch {
	case atual == largura:
		return texto
	case atual < largura:
		return texto + strings.Repeat(" ", largura-atual)
	}
	return cortar(texto, largura)
}

// centralizarEm põe o texto no meio da largura.
func centralizarEm(texto string, largura int) string {
	sobra := largura - lipgloss.Width(texto)
	if sobra <= 0 {
		return texto
	}
	return strings.Repeat(" ", sobra/2) + texto + strings.Repeat(" ", sobra-sobra/2)
}

// cortar corta o texto na largura visível, sem quebrar os códigos de cor no
// meio nem deixar cor vazando para o resto da linha.
func cortar(texto string, largura int) string {
	if largura <= 0 {
		return ""
	}
	var saida strings.Builder
	visivel, tinhaCor := 0, false
	dentroDoCodigo := false
	for _, r := range texto {
		if r == 0x1b {
			dentroDoCodigo, tinhaCor = true, true
		}
		if dentroDoCodigo {
			saida.WriteRune(r)
			if r >= 0x40 && r <= 0x7e && r != '[' {
				dentroDoCodigo = false
			}
			continue
		}
		if visivel >= largura {
			if tinhaCor {
				saida.WriteString("\x1b[0m")
			}
			return saida.String()
		}
		saida.WriteRune(r)
		visivel++
	}
	return saida.String()
}

// lar é o diretório da conta, lido uma vez só.
var casaDoUsuario = sync.OnceValue(func() string {
	casa, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return casa
})

func lar() string { return casaDoUsuario() }
