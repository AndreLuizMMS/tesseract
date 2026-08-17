// Package tema é o único lugar do Tesseract onde cor vira significado. Fora
// daqui não existe hex solto: quem desenha pede o token pelo nome, e é este
// pacote que decide o que sai no terminal.
//
// Três regras governam a paleta e nenhuma delas é estética:
//
//   - verde é POSSE DO TECLADO. Nunca é estado. O #55FFA6 (BrandPhosphor)
//     aparece no máximo uma vez por tela — na célula que está com o seu
//     teclado, e em mais nada.
//   - ciano é ESTRUTURA. Nunca é estado. Grade, cantos, numeração, rótulos.
//   - estado não usa verde nem ciano, e urgência é área preenchida, não
//     matiz: "aprovar" sai como barra sólida invertida na linha inteira,
//     enquanto "respondeu" é só um glifo.
//
// Sem brilho, sem scanline, sem aberração cromática: esses efeitos existem na
// superfície de marca (README, site, banner), nunca dentro do terminal. Sem
// ligadura, sem emoji, sem canto arredondado.
package tema

import (
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// Tokens. Fonte única de verdade — nenhum outro arquivo do projeto escreve
// hex.
// ---------------------------------------------------------------------------

// Base: os dez tons de fundo, linha e texto.
const (
	BgVoid     = "#030507" // fundo do modo DIGITAR
	BgBase     = "#070B0C" // fundo padrão
	BgSurface  = "#0C1315" // corpo da célula
	BgRaised   = "#121C1F" // cabeçalho, seleção
	LineDim    = "#16282A" // grade não focada
	LineActive = "#205047" // grade do projeto focado
	FgFaint    = "#3E534E" // desligado, atalhos
	FgMuted    = "#6C8076" // texto secundário
	FgDefault  = "#BFD1C6" // texto principal
	FgBright   = "#E8F4EC" // títulos
)

// Neon verde: posse do teclado. Nunca estado.
const (
	BrandDeep     = "#0B3322"
	BrandCore     = "#1F7A4C" // grade do projeto ativo, logo
	BrandLive     = "#35C27A" // célula focada
	BrandPhosphor = "#55FFA6" // dono do teclado — um por tela, sempre
)

// Neon ciano: estrutura. Nunca estado.
const (
	FluxDeep = "#082F31"
	FluxCore = "#128C86" // grade, cantos, rótulos
	Flux     = "#22E0D0" // glifo, numeração, segunda dimensão
)

// Estados: nenhum verde, nenhum ciano.
const (
	StateWorking = "#6C8076" // ▸ trabalhando
	StateRead    = "#7DB7E8" // ⬤ respondeu
	StateBlock   = "#FFB454" // ⏵ aprovar
	StateDead    = "#FF3B47" // ✖ caiu
	StateOff     = "#3E534E" // ○ parada
	StateOrphan  = "#C77DFF" // ⚠ órfã
)

// As quatro cores da ANSI 16 que não têm papel próprio na paleta. Existem
// para o punhado de lugares que precisa de mais matizes distintas do que os
// papéis oferecem — a faixa de cor dos projetos, por exemplo — sem cair no
// verde nem no ciano, que têm dono.
const (
	CorAnsiVermelho = "#C22F38" // ANSI 1
	CorAnsiAmarelo  = "#C9A227" // ANSI 3
	CorAnsiAzul     = "#3E7FA8" // ANSI 4
	CorAnsiMagenta  = "#8B4FC4" // ANSI 5
)

// Glifo de um caractere da marca. O símbolo cheio 7×5 mora no README.
const Glifo = "⧉"

// ---------------------------------------------------------------------------
// Perfil de cor: 24 bits, 16 cores ou nenhuma.
// ---------------------------------------------------------------------------

// Perfil é quanta cor o terminal aceita neste momento.
type Perfil int

const (
	// CorTotal: 24 bits, os hex saem como estão.
	CorTotal Perfil = iota
	// Cores16: só a ANSI 16, cada token cai no índice de ansi16.
	Cores16
	// SemCor: NO_COLOR=1 ou TERM=dumb. Só negrito, reverso e sublinhado.
	SemCor
)

// ansi16 é o destino de cada token quando o terminal só tem 16 cores.
//
// Dois pares colidem de propósito, porque a ANSI 16 não tem tom intermediário:
// StateWorking e StateOff caem os dois no 8. É por isso que o glifo (▸ contra
// ○) é obrigatório: o alfabeto de estados tem que ler sem cor nenhuma.
var ansi16 = map[string]string{
	BgVoid:     "0",
	BgBase:     "0",
	BgSurface:  "0",
	BgRaised:   "8",
	LineDim:    "8",
	LineActive: "2",
	FgFaint:    "8",
	FgMuted:    "8",
	FgDefault:  "7",
	FgBright:   "15",

	BrandDeep:     "2",
	BrandCore:     "2",
	BrandLive:     "2",
	BrandPhosphor: "10",

	FluxDeep: "6",
	FluxCore: "6",
	Flux:     "14",

	StateRead:   "12",
	StateBlock:  "11",
	StateDead:   "9",
	StateOrphan: "13",

	CorAnsiVermelho: "1",
	CorAnsiAmarelo:  "3",
	CorAnsiAzul:     "4",
	CorAnsiMagenta:  "5",
}

// Atual é o perfil em vigor. É variável para o teste poder fixá-lo.
var Atual = Detectar()

// Detectar lê o ambiente e decide o perfil. NO_COLOR vence tudo — basta a
// variável existir, o valor não importa (no-color.org).
func Detectar() Perfil {
	if _, tem := os.LookupEnv("NO_COLOR"); tem {
		return SemCor
	}
	term := os.Getenv("TERM")
	if term == "dumb" {
		return SemCor
	}
	if strings.Contains(term, "16color") {
		return Cores16
	}
	// Fora esses dois casos declarados, o tema entrega a cor cheia e deixa o
	// lipgloss rebaixar para o que o terminal aguenta — ele já sabe fazer
	// isso, e adivinhar aqui só criaria uma segunda verdade.
	return CorTotal
}

// Pintar devolve o estilo com frente e fundo já resolvidos para o perfil
// corrente. Fundo vazio significa "não pinta fundo".
func Pintar(frente, fundo string) lipgloss.Style {
	e := lipgloss.NewStyle()
	switch Atual {
	case SemCor:
		return e
	case Cores16:
		if c, ok := ansi16[frente]; ok {
			e = e.Foreground(lipgloss.Color(c))
		}
		if c, ok := ansi16[fundo]; ok {
			e = e.Background(lipgloss.Color(c))
		}
	default:
		if frente != "" {
			e = e.Foreground(lipgloss.Color(frente))
		}
		if fundo != "" {
			e = e.Background(lipgloss.Color(fundo))
		}
	}
	return e
}

// FundoDaTela e FrenteDaTela são o fundo e o texto que o aplicativo impõe ao
// terminal enquanto está aberto. O fundo é o BgVoid — o mais fundo da paleta —
// porque a grade só lê bem quando o preto atrás dela é mesmo preto. Com
// NO_COLOR nada é imposto: devolvem nil, e o terminal fica como estava.
func FundoDaTela() color.Color {
	if Atual == SemCor {
		return nil
	}
	return cor(BgVoid)
}

func FrenteDaTela() color.Color {
	if Atual == SemCor {
		return nil
	}
	return cor(FgDefault)
}

// Sobre acrescenta um fundo a um estilo que já veio pintado. Sem cor, devolve
// o estilo como está — fundo pintado não sobrevive a NO_COLOR.
func Sobre(e lipgloss.Style, fundo string) lipgloss.Style {
	switch Atual {
	case SemCor:
		return e
	case Cores16:
		return e.Background(lipgloss.Color(ansi16[fundo]))
	default:
		return e.Background(lipgloss.Color(fundo))
	}
}

// ---------------------------------------------------------------------------
// Alfabeto de estados.
// ---------------------------------------------------------------------------

// Estado é a situação da célula, do jeito que ela aparece no marcador.
type Estado string

const (
	Trabalhando Estado = "trabalhando"
	Respondeu   Estado = "respondeu"
	Aprovar     Estado = "aprovar"
	Caiu        Estado = "caiu"
	Parada      Estado = "parada"
	Orfa        Estado = "orfa"
)

// Marcador é como um estado se mostra: um glifo, um rótulo, uma cor e um
// estilo. Invertido marca o estado que ocupa a linha inteira em vez de virar
// só um sinalzinho.
type Marcador struct {
	Glifo     string
	Rotulo    string
	Cor       string
	Invertido bool
	// Apagado desenha o marcador em faint. Existe porque StateWorking e
	// StateOff colidem no mesmo índice da ANSI 16.
	Apagado bool
}

// >>> mapa-de-estados
// Nenhuma cor deste mapa pode ser verde ou ciano — scripts/check-theme.sh
// falha o build se alguém tentar. Verde é posse do teclado, ciano é estrutura.
var mapa = map[Estado]Marcador{
	Trabalhando: {Glifo: "▸", Rotulo: "TRABALHANDO", Cor: StateWorking},
	Respondeu:   {Glifo: "⬤", Rotulo: "RESPONDEU", Cor: StateRead},
	Aprovar:     {Glifo: "⏵", Rotulo: "APROVAR", Cor: StateBlock, Invertido: true},
	Caiu:        {Glifo: "✖", Rotulo: "CAIU", Cor: StateDead},
	Parada:      {Glifo: "○", Rotulo: "PARADA", Cor: StateOff, Apagado: true},
	Orfa:        {Glifo: "⚠", Rotulo: "ÓRFÃ", Cor: StateOrphan},
}

// <<< mapa-de-estados

// Do devolve o marcador do estado. Estado desconhecido cai em Parada, que é o
// estado mais inofensivo: nada acontecendo.
func Do(e Estado) Marcador {
	if m, ok := mapa[e]; ok {
		return m
	}
	return mapa[Parada]
}

// Estilo é como o marcador se pinta. Invertido vira barra sólida: fundo na
// cor do estado, texto no fundo mais fundo. Sem cor, inverte de verdade — o
// reverso do terminal não precisa de paleta.
func (m Marcador) Estilo() lipgloss.Style {
	if m.Invertido {
		if Atual == SemCor {
			return lipgloss.NewStyle().Reverse(true).Bold(true)
		}
		return Pintar(BgVoid, m.Cor).Bold(true)
	}
	e := Pintar(m.Cor, "")
	if m.Apagado {
		e = e.Faint(true)
	}
	return e
}

// Linha desenha o marcador. O estado que bloqueia ocupa a largura inteira,
// porque urgência aqui é área preenchida, não matiz; os outros são só o glifo
// e o rótulo. Largura menor ou igual a zero devolve a versão curta.
func (m Marcador) Linha(largura int) string {
	texto := m.Glifo + " " + m.Rotulo
	if m.Invertido && largura > 0 {
		return m.Estilo().Width(largura).Render(" " + texto + " ")
	}
	return m.Estilo().Render(texto)
}

// ---------------------------------------------------------------------------
// Os dois modos.
// ---------------------------------------------------------------------------

// Modo é quem é o dono do teclado. Nunca há dois donos ao mesmo tempo.
type Modo int

const (
	// Navegar: toda tecla é do aplicativo. Borda simples.
	Navegar Modo = iota
	// Digitar: toda tecla é da célula. Borda dupla, fundo mais fundo e selo.
	Digitar
)

// SeloDigitar é o aviso invertido que só aparece em DIGITAR.
const SeloDigitar = "▓ DIGITAR ▓"

// Borda do modo. Sem canto arredondado em lugar nenhum.
func (m Modo) Borda() lipgloss.Border {
	if m == Digitar {
		return lipgloss.DoubleBorder()
	}
	return lipgloss.NormalBorder()
}

// Fundo do modo. DIGITAR escurece a tela inteira: é o primeiro sinal de que o
// aplicativo ficou mudo.
func (m Modo) Fundo() string {
	if m == Digitar {
		return BgVoid
	}
	return BgBase
}

// Selo devolve o selo do modo já pintado. Em NAVEGAR não há selo.
func (m Modo) Selo() string {
	if m != Digitar {
		return ""
	}
	if Atual == SemCor {
		return lipgloss.NewStyle().Reverse(true).Bold(true).Render(" " + SeloDigitar + " ")
	}
	return Pintar(BgVoid, BrandPhosphor).Bold(true).Render(" " + SeloDigitar + " ")
}

// Celula devolve o estilo da borda da célula. O verde phosphor sai daqui e só
// daqui: é a célula que está com o teclado, e existe uma por tela.
func Celula(m Modo, focada bool) lipgloss.Style {
	e := lipgloss.NewStyle().Border(m.Borda())
	if Atual == SemCor {
		// Sem cor, o modo e o foco continuam legíveis: borda dupla contra
		// borda simples, e negrito na célula que tem o teclado.
		return e.Bold(focada)
	}
	switch {
	case focada && m == Digitar:
		return e.BorderForeground(cor(BrandPhosphor)).Bold(true)
	case focada:
		return e.BorderForeground(cor(BrandLive))
	default:
		return e.BorderForeground(cor(LineDim))
	}
}

// Grade devolve o estilo da linha de projeto: verde escuro quando é o projeto
// focado, apagado quando não é.
func Grade(focado bool) lipgloss.Style {
	if focado {
		return Pintar(LineActive, "")
	}
	return Pintar(LineDim, "")
}

// Estrutura é tudo que é moldura e numeração: ciano, sempre.
func Estrutura() lipgloss.Style { return Pintar(FluxCore, "") }

// Numeracao é a segunda dimensão da grade — índice de projeto, contador de
// célula.
func Numeracao() lipgloss.Style { return Pintar(Flux, "") }

// cor resolve um token para o perfil corrente, para os lugares que pedem uma
// cor solta em vez de um estilo — a borda, por exemplo. Não deve ser chamada
// com o perfil SemCor: quem chama decide antes se pinta.
func cor(token string) color.Color {
	if Atual == Cores16 {
		return lipgloss.Color(ansi16[token])
	}
	return lipgloss.Color(token)
}
