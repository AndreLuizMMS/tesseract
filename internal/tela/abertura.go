package tela

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/andreluiz/tesseract/internal/tema"
)

// A abertura é o único lugar do Tesseract onde existe movimento por prazer.
// Ela dura cerca de um segundo e roda enquanto o motor é procurado, então o
// tempo dela é tempo que já ia passar — nenhuma espera é inventada.
//
// A marca não aparece pronta: ela se desenha. O quadrado de trás primeiro, o
// da frente por cima, cada um traço a traço, com a ponta da caneta acesa em
// fósforo. A tessera acende no fim, o símbolo inteiro estoura num quadro e
// assenta. Só então o nome abre, letra a letra.
//
// Nada disso é brilho, scanline ou glitch — o terminal não emite luz e a
// regra da marca continua valendo. O que se move aqui é a ordem em que as
// coisas existem, não a textura delas.
const (
	passoDoTraco    = 22 * time.Millisecond
	passoDaIgnicao  = 90 * time.Millisecond
	passoDoEstouro  = 130 * time.Millisecond
	pausaDaMarca    = 260 * time.Millisecond
	passoDaLetra    = 62 * time.Millisecond
	pausaDoNome     = 200 * time.Millisecond
	passoDaContagem = 170 * time.Millisecond
)

// caminhoDoTraco é a ordem em que o desenho nasce: o quadrado de trás inteiro,
// depois o da frente por cima. Cada par é linha e coluna dentro do símbolo
// 7×5. O percurso de cada quadrado é o de uma caneta que não levanta: topo da
// esquerda para a direita, desce a direita, volta pela base, sobe a esquerda.
var caminhoDoTraco = [][2]int{
	// quadrado de trás
	{1, 1}, {1, 2}, {1, 3}, {1, 4}, {1, 5}, {1, 6},
	{2, 6}, {3, 6},
	{4, 6}, {4, 5}, {4, 4}, {4, 3}, {4, 2}, {4, 1},
	{3, 1}, {2, 1},
	// quadrado da frente
	{0, 0}, {0, 1}, {0, 2}, {0, 3}, {0, 4}, {0, 5},
	{1, 5}, {2, 5},
	{3, 5}, {3, 4}, {3, 3}, {3, 2}, {3, 1}, {3, 0},
	{2, 0}, {1, 0},
}

// posicaoDaTessera é onde a marca acende.
var posicaoDaTessera = [2]int{2, 3}

// alturaDaAbertura é o bloco inteiro: as cinco linhas do símbolo, uma em
// branco e as três linhas da contagem.
const alturaDaAbertura = 9

// LarguraNecessaria é a largura, em colunas, da linha mais comprida que a
// abertura chega a desenhar. Abaixo disso o terminal quebra a linha (wrap),
// o cursor-up fixo da animação erra a conta e sobra lixo na tela — por isso
// quem decide animar precisa checar contra isto antes.
func LarguraNecessaria() int {
	assinatura := []string{"", tema.Nome, tema.Tagline, "", tema.Versao}
	maior := 0
	for i := range tema.Simbolo {
		lado := ""
		if i < len(assinatura) {
			lado = assinatura[i]
		}
		if largura := len([]rune("   " + tema.Simbolo[i] + "    " + lado)); largura > maior {
			maior = largura
		}
	}
	return maior
}

// Abertura é a animação de partida. Ela pinta sempre o bloco inteiro e o
// reescreve por cima de si mesmo, então não deixa rastro na rolagem.
type Abertura struct {
	saida  io.Writer
	animar bool
	// aceso guarda quais posições do símbolo já foram desenhadas.
	aceso map[[2]int]bool
	// nome é quanto do wordmark já abriu, em letras.
	nome int
	// assinado liga a tagline e a versão.
	assinado bool
	// estouro pinta o símbolo inteiro em fósforo por um quadro.
	estouro bool
	// contagem são as linhas do motor, que chegam depois da conexão.
	contagem []string
	// desenhado diz se o bloco já ocupa a tela e pode ser reescrito por cima.
	desenhado bool
}

// NovaAbertura prepara a animação. Com animar falso ela vira o banner estático
// de sempre — é o que acontece sem terminal de verdade, ou com
// TESSERACT_SEM_ABERTURA no ambiente.
func NovaAbertura(saida io.Writer, animar bool) *Abertura {
	return &Abertura{saida: saida, animar: animar, aceso: map[[2]int]bool{}}
}

// Montar desenha a marca se montando e o nome abrindo. Volta assim que o
// bloco está inteiro na tela.
func (a *Abertura) Montar() {
	if !a.animar {
		a.acenderTudo()
		a.pintar()
		return
	}

	// O traço: a ponta da caneta é o último ponto de cada quadro.
	for i, ponto := range caminhoDoTraco {
		a.aceso[ponto] = true
		a.pintarComPonta(&caminhoDoTraco[i])
		time.Sleep(passoDoTraco)
	}

	// A ignição: a tessera acende sozinha, e o símbolo inteiro estoura num
	// quadro só antes de assentar.
	a.aceso[posicaoDaTessera] = true
	for range 3 {
		a.pintarComPonta(&posicaoDaTessera)
		time.Sleep(passoDaIgnicao)
	}
	a.estouro = true
	a.pintar()
	time.Sleep(passoDoEstouro)
	a.estouro = false
	a.pintar()

	// A marca fechada segura sozinha por um instante antes do nome entrar. É a
	// pausa que separa as duas metades da abertura — sem ela, o wordmark
	// atropela o desenho que acabou de nascer.
	time.Sleep(pausaDaMarca)

	// O nome: uma letra por vez, a última sempre acesa.
	for a.nome < len([]rune(tema.Nome)) {
		a.nome++
		a.pintar()
		time.Sleep(passoDaLetra)
	}
	a.assinado = true
	a.pintar()
	time.Sleep(pausaDoNome)
}

// Contar fecha a abertura com o que o motor devolveu, uma linha por vez.
func (a *Abertura) Contar(linhas []string) {
	if !a.animar {
		a.contagem = linhas
		a.pintar()
		return
	}
	for _, linha := range linhas {
		a.contagem = append(a.contagem, linha)
		a.pintar()
		time.Sleep(passoDaContagem)
	}
}

// acenderTudo põe o bloco no estado final, sem animação nenhuma.
func (a *Abertura) acenderTudo() {
	for _, ponto := range caminhoDoTraco {
		a.aceso[ponto] = true
	}
	a.aceso[posicaoDaTessera] = true
	a.nome = len([]rune(tema.Nome))
	a.assinado = true
}

func (a *Abertura) pintar() { a.pintarComPonta(nil) }

// pintarComPonta escreve o bloco inteiro de uma vez. A ponta, quando existe, é
// a posição que acabou de nascer e sai em fósforo — é ela que dá a sensação de
// caneta correndo pelo desenho.
func (a *Abertura) pintarComPonta(ponta *[2]int) {
	var quadro strings.Builder
	if a.desenhado {
		// Sobe até o topo do bloco para reescrever por cima. Sem isto a
		// animação viraria uma cascata de cópias na rolagem.
		quadro.WriteString("\x1b[" + strconv.Itoa(alturaDaAbertura) + "A")
	}
	a.desenhado = true

	for _, linha := range a.linhasDoBloco(ponta) {
		quadro.WriteString("\x1b[2K" + linha + "\n")
	}
	_, _ = io.WriteString(a.saida, quadro.String())
}

// linhasDoBloco monta as nove linhas do quadro atual.
func (a *Abertura) linhasDoBloco(ponta *[2]int) []string {
	assinatura := []string{"", a.wordmark(), a.tagline(), "", a.versao()}
	linhas := make([]string, 0, alturaDaAbertura)

	for i := range tema.Simbolo {
		lado := ""
		if i < len(assinatura) {
			lado = assinatura[i]
		}
		linhas = append(linhas, strings.TrimRight("   "+a.linhaDoSimbolo(i, ponta)+"    "+lado, " "))
	}

	linhas = append(linhas, "")
	for i := range 3 {
		if i < len(a.contagem) {
			linhas = append(linhas, "   "+a.contagem[i])
			continue
		}
		linhas = append(linhas, "")
	}
	return linhas
}

// linhaDoSimbolo pinta uma linha do desenho no estado atual: o que já nasceu
// aparece na cor do quadrado a que pertence, o que ainda não nasceu é espaço.
func (a *Abertura) linhaDoSimbolo(linha int, ponta *[2]int) string {
	var saida strings.Builder
	fosforo := tema.Pintar(tema.BrandPhosphor, "").Bold(true)
	for coluna, r := range []rune(tema.Simbolo[linha]) {
		posicao := [2]int{linha, coluna}
		pintar, tem := tema.CorDoSimbolo(linha, coluna)
		switch {
		case !tem:
			saida.WriteRune(r)
		case !a.aceso[posicao]:
			saida.WriteRune(' ')
		case a.estouro || (ponta != nil && *ponta == posicao):
			saida.WriteString(fosforo.Render(string(r)))
		default:
			saida.WriteString(pintar.Render(string(r)))
		}
	}
	return saida.String()
}

// wordmark abre o nome letra a letra, com a que acabou de chegar acesa.
func (a *Abertura) wordmark() string {
	letras := []rune(tema.Nome)
	if a.nome <= 0 {
		return ""
	}
	aberto, acesa := string(letras[:max(a.nome-1, 0)]), ""
	if a.nome <= len(letras) {
		acesa = string(letras[a.nome-1])
	}
	claro := tema.Pintar(tema.FgBright, "").Bold(true)
	if a.nome >= len(letras) {
		return claro.Render(string(letras))
	}
	return claro.Render(aberto) + tema.Pintar(tema.BrandPhosphor, "").Bold(true).Render(acesa)
}

func (a *Abertura) tagline() string {
	if !a.assinado {
		return ""
	}
	return tema.Pintar(tema.FgMuted, "").Render(tema.Tagline)
}

func (a *Abertura) versao() string {
	if !a.assinado {
		return ""
	}
	return tema.Pintar(tema.FgFaint, "").Render(tema.Versao)
}

// LinhaDoMotor formata uma linha da contagem: a seta em ciano, porque ela é
// estrutura, e o texto no tom de sempre.
func LinhaDoMotor(texto string) string {
	return tema.Pintar(tema.FluxCore, "").Render(">") + " " + tema.Pintar(tema.FgMuted, "").Render(texto)
}
