package tela

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
	"github.com/andreluiz/tesseract/internal/tema"
)

// Nenhuma cor é escolhida aqui: todas vêm de internal/tema, que é o único
// lugar do projeto onde existe hex. Em DIGITAR o aplicativo fica mudo e mostra
// que está mudo: barra e tiras apagam, o selo aparece e a borda da célula
// focada engrossa.
var (
	corApagada = tema.Pintar(tema.FgFaint, "")
	corBarra   = tema.Pintar(tema.FgDefault, "")
	corSelo    = tema.Pintar(tema.BgVoid, tema.StateBlock).Bold(true)
	corTitulo  = tema.Pintar(tema.FgBright, "").Bold(true)
	// A grade das células é ciano: estrutura. É o segundo quadrado do símbolo
	// desenhado na tela inteira, e é metade da identidade — em cinza ela some
	// e o Tesseract vira mais um painel escuro.
	corBorda       = tema.Pintar(tema.FluxCore, "")
	corBordaFocada = tema.Pintar(tema.BrandLive, "").Bold(true)
	corRolagem     = tema.Pintar(tema.StateBlock, "")
	// Em DIGITAR a célula que tem o teclado fica verde phosphor: é o quarto
	// sinal do modo, e o único aceso na tela. É o único lugar do aplicativo
	// que usa essa cor — uma vez por tela, sempre.
	corDigitando = tema.Pintar(tema.BrandPhosphor, "")
	corErro      = tema.Pintar(tema.StateDead, "")
	corQuota     = tema.Pintar(tema.FgMuted, "")
	corQuotaAlta = tema.Pintar(tema.StateBlock, "")
	// corGradeAtiva é a tira do projeto que está com o foco.
	corGradeAtiva = tema.Pintar(tema.LineActive, "")
)

// corProjeto é o nome do projeto na tira: caixa alta em fg.bright, o topo da
// hierarquia de texto. Uma cor por projeto virava arco-íris e brigava com o
// único sinal que precisa gritar — a célula que está com o teclado. Quem
// separa um projeto do outro é a tira, o nome e a posição, não o matiz.
func corProjeto(int) lipgloss.Style {
	return tema.Pintar(tema.FgBright, "")
}

type marcador struct {
	simbolo string
	estado  tema.Estado
	// bloqueia marca o estado que não anda sem você. Ele não muda de matiz
	// para chamar atenção: muda de área, virando barra sólida invertida — e é
	// o único que pisca.
	bloqueia bool
}

// marcadores traduz o estado da célula no símbolo e no estado do tema. As
// cores saem todas de tema.Do — nenhum estado é verde ou ciano, porque esses
// dois significam posse do teclado e estrutura.
var marcadores = map[string]marcador{
	"trabalhando": deTema(tema.Trabalhando),
	"respondeu":   deTema(tema.Respondeu),
	"aprovar":     deTema(tema.Aprovar),
	"caiu":        deTema(tema.Caiu),
	"parada":      deTema(tema.Parada),
	"orfa":        deTema(tema.Orfa),
}

func deTema(estado tema.Estado) marcador {
	m := tema.Do(estado)
	return marcador{simbolo: m.Glifo, estado: estado, bloqueia: m.Invertido}
}

// cor pergunta ao tema a cada quadro, em vez de guardar o estilo pronto: a
// barra que pisca muda de aparência entre um desenho e o outro, e um estilo
// congelado no início do programa nunca piscaria.
func (m marcador) cor() lipgloss.Style { return tema.Do(m.estado).Estilo() }

// chip transforma uma cor de texto no selo preenchido do estado da célula: o
// mesmo fundo elevado para todo estado, negrito, só a cor do texto muda.
func chip(cor lipgloss.Style) lipgloss.Style {
	return tema.Sobre(cor, tema.BgRaised).Bold(true)
}

// selo é como o marcador aparece na borda da célula. O estado que bloqueia já
// vem invertido do tema e não ganha o fundo do chip: ele é a área preenchida.
func (m marcador) selo() lipgloss.Style {
	if m.bloqueia {
		return m.cor()
	}
	return chip(m.cor())
}

func marcadorDe(estado string) marcador {
	if achado, existe := marcadores[estado]; existe {
		return achado
	}
	return marcadores["parada"]
}

// corMarca é o glifo da marca na barra. Em DIGITAR ele apaga junto com o
// resto: o aplicativo está mudo, e a marca também.
func corMarca(modo teclado.Modo) lipgloss.Style {
	if modo == teclado.Digitar {
		return corApagada
	}
	return tema.Pintar(tema.Flux, "")
}

// MioloDaCelulaCheia é o espaço útil de uma célula ocupando a tela toda:
// desconta a barra de título, o rodapé e as bordas da caixa.
func MioloDaCelulaCheia(largura, altura int) Geometria {
	return Geometria{Colunas: max(largura-2, 1), Linhas: max(altura-alturaDaBarra-3, 1)}
}

// DesenharCheia põe uma célula na tela inteira. É como se copia um bloco de
// texto sem pegar os vizinhos, e é o que a tecla de tela cheia faz.
func DesenharCheia(projeto protocolo.Projeto, celula protocolo.Celula, modo teclado.Modo, largura, altura int, erro string) string {
	linhas := barraDeTitulo(modo, largura, contarChamadosDoProjeto(projeto), nil)
	linhas = append(linhas, caixaDaCelula(celula, modo, true, largura, altura-alturaDaBarra-1)...)
	linhas = append(linhas, rodape(modo, largura, erro))
	return strings.Join(linhas, "\n")
}

// alturaDaBarra é quanto o cabeçalho ocupa: a faixa da marca e a linha de
// estado. Toda conta de layout desconta isto, em vez de repetir o número.
const alturaDaBarra = 2

// barraDeTitulo é o cabeçalho: uma faixa com a marca no meio de uma régua, e
// abaixo dela quem chama você e de quem é o teclado. É o primeiro dos três
// sinais redundantes do modo.
func barraDeTitulo(modo teclado.Modo, largura int, chamados map[string]int, quota *protocolo.Quota) []string {
	return []string{
		faixaDaMarca(modo, largura),
		linhaDeEstado(modo, largura, chamados, quota),
	}
}

// faixaDaMarca é a régua que atravessa a tela com o nome no meio. A régua é
// ciano porque é estrutura, e o nome vem espaçado — a marca escreve assim, e
// no terminal o espaço entre as letras é o que existe de tipografia.
func faixaDaMarca(modo teclado.Modo, largura int) string {
	// A régua é ciano, como toda a grade: ela é a moldura de cima da tela, e
	// usar aqui o verde da tira do projeto focado diria "posse do teclado" onde
	// não há teclado nenhum.
	nome := "T E S S E R A C T"
	pintarNome, pintarRegua := corTitulo, corBorda
	traco := "─"
	if modo == teclado.Digitar {
		// Em DIGITAR o aplicativo está mudo, e a faixa apaga junto com o resto.
		nome, pintarNome, pintarRegua = "t e s s e r a c t", corApagada, corApagada
	}

	miolo := " " + corMarca(modo).Render(tema.Glifo) + "  " + pintarNome.Render(nome) + " "
	sobra := largura - lipgloss.Width(miolo)
	if sobra < 4 {
		return preencher(centralizarEm(miolo, largura), largura)
	}
	esquerda := sobra / 2
	return pintarRegua.Render(strings.Repeat(traco, esquerda)) + miolo +
		pintarRegua.Render(strings.Repeat(traco, sobra-esquerda))
}

// linhaDeEstado é a segunda linha do cabeçalho: quem chama de um lado, o
// estado da janela do outro.
func linhaDeEstado(modo teclado.Modo, largura int, chamados map[string]int, quota *protocolo.Quota) string {
	pintarSinais := corBarra
	if modo == teclado.Digitar {
		pintarSinais = corApagada
	}

	// À esquerda, quem está chamando você — a única informação da barra que
	// pede ação.
	var sinais []string
	for _, estado := range []string{"respondeu", "aprovar"} {
		if chamados[estado] > 0 {
			sinais = append(sinais, marcadorDe(estado).simbolo+" "+strconv.Itoa(chamados[estado]))
		}
	}
	esquerda := ""
	if len(sinais) > 0 {
		esquerda = " " + pintarSinais.Render(strings.Join(sinais, "   "))
	}

	// À direita, o estado da janela: a quota e de quem é o teclado. O canto
	// superior direito é onde o olho procura isso, e é lá que mora o único
	// selo invertido que pode estar visível de cada vez.
	direita := pintarSinais.Render(modo.String())
	if modo == teclado.Digitar {
		direita = corSelo.Render(" ▓ DIGITAR ▓ ")
	} else if quota != nil {
		pintar := corQuota
		if quota.Percentual >= 80 {
			pintar = corQuotaAlta
		}
		direita = pintar.Render("⏳ "+strconv.Itoa(quota.Percentual)+"% "+quota.Vira) + "   " + direita
	}
	return tresPartes(esquerda, "", direita, largura)
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
	linhas := barraDeTitulo(modo, largura, nil, nil)
	recado := "nenhuma célula na tela — n cria a primeira"
	if aviso != "" {
		recado = aviso
	}
	// A grade vazia é o único momento em que a marca cabe inteira na tela sem
	// roubar espaço de trabalho. Todo estado vazio termina numa tecla.
	miolo := append(tema.SimboloPintado(), "", corApagada.Render(recado))

	corpo := max(altura-alturaDaBarra-1, 1)
	topo := max((corpo-len(miolo))/2, 0)
	for i := range corpo {
		if i >= topo && i-topo < len(miolo) {
			linhas = append(linhas, preencher(centralizarEm(miolo[i-topo], largura), largura))
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

// apagar tira a cor do conteúdo e devolve a linha inteira em cinza. É o que
// faz só a célula focada ficar acesa: com muitas seções na tela, a cor vira
// ruído se todas gritam ao mesmo tempo.
func apagar(texto string) string {
	var saida strings.Builder
	dentroDoCodigo := false
	for _, r := range texto {
		if r == 0x1b {
			dentroDoCodigo = true
			continue
		}
		if dentroDoCodigo {
			if r >= 0x40 && r <= 0x7e && r != '[' {
				dentroDoCodigo = false
			}
			continue
		}
		saida.WriteRune(r)
	}
	if saida.Len() == 0 {
		return ""
	}
	return corApagada.Render(saida.String())
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

// rotuloDaCelula é o que aparece na borda de cima: o tipo e o nome. Quando a
// célula tem abas, as abas viram o rótulo — a ativa em destaque.
func rotuloDaCelula(celula protocolo.Celula) string {
	if len(celula.Abas) == 0 {
		return celula.Tipo + " · " + celula.Nome
	}
	var abas []string
	for _, aba := range celula.Abas {
		if aba == celula.Aba {
			abas = append(abas, corSelo.Render(" "+aba+" "))
			continue
		}
		abas = append(abas, corApagada.Render(" "+aba+" "))
	}
	return strings.Join(abas, "") + " " + celula.Nome
}
