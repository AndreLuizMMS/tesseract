package tela

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

const (
	// larguraMinimaDeCelula é o menos que uma célula precisa para o conteúdo
	// dela ainda dizer alguma coisa. A fileira enche até aqui antes de descer
	// para a linha de baixo.
	larguraMinimaDeCelula = 34
	// larguraApertadaDeCelula é até onde a célula encolhe quando a alternativa
	// é a fileira descer para fora da tela. Aperta, mas ainda se lê.
	larguraApertadaDeCelula = 24
	// alturaMinimaDeCelula é borda de cima, três linhas de conteúdo e borda de
	// baixo.
	alturaMinimaDeCelula = 5
	// larguraMinimaDeColuna é o menos que uma coluna de projetos precisa para
	// ainda caber duas células nela.
	larguraMinimaDeColuna = 2 * larguraApertadaDeCelula
	// alturaMinimaDeProjeto é a divisória mais a célula mais baixa que ainda
	// diz alguma coisa. Quando os projetos não cabem nisso, a tela abre outra
	// coluna em vez de deixar célula fora do quadro.
	alturaMinimaDeProjeto = alturaMinimaDeCelula + 1
)

// Foco é onde o usuário está agora.
type Foco struct {
	Projeto int
	Celula  int
	Cheia   bool
}

// Geometria é a área útil que a tela reservou para o miolo de uma célula. O
// motor usa isso para dar ao processo lá dentro um terminal do tamanho certo.
type Geometria struct {
	Colunas int
	Linhas  int
}

// faixa é uma fileira de células do mesmo projeto, lado a lado. Um projeto com
// muitas células ocupa várias faixas; um projeto com uma célula ocupa a largura
// inteira. É isso que faz todas as células caberem na tela sem nenhuma virar
// tira.
type faixa struct {
	projeto int
	celulas []int
	// abre marca a primeira faixa do projeto, que é a que leva o nome dele.
	abre bool
	// altura é quanto ela recebeu na divisão da tela.
	altura int
}

// coluna é uma fatia vertical da tela com os projetos que couberam nela. A
// tela é uma coluna só enquanto os projetos couberem na altura; quando não
// cabem, o mosaico abre colunas em vez de espremer faixa ou jogar célula para
// fora.
type coluna struct {
	faixas  []faixa
	x       int
	largura int
	// escondidas conta as células desta coluna que não couberam na altura.
	escondidas int
}

// Disposicao é o resultado do cálculo de layout. Desenhar e avisar o tamanho ao
// motor saem daqui, para nunca discordarem.
type Disposicao struct {
	colunas []coluna
	miolos  map[string]Geometria
	origens map[string][2]int
	// escondidas conta as células que não couberam na tela.
	escondidas int
}

// todasAsFaixas junta as fileiras de todas as colunas, na ordem em que os
// projetos aparecem.
func (d Disposicao) todasAsFaixas() []faixa {
	var faixas []faixa
	for _, c := range d.colunas {
		faixas = append(faixas, c.faixas...)
	}
	return faixas
}

// Miolos é o que a tela avisa ao motor: o tamanho útil de cada célula visível.
func (d Disposicao) Miolos() map[string]Geometria { return d.miolos }

// Dispor calcula o layout do mosaico: todas as células de todos os projetos ao
// mesmo tempo, ocupando o máximo de espaço, com o projeto como divisão.
func Dispor(estado protocolo.Estado, foco Foco, largura, altura int) Disposicao {
	d := Disposicao{miolos: map[string]Geometria{}, origens: map[string][2]int{}}
	if len(estado.Projetos) == 0 || largura < 12 || altura < 6 {
		return d
	}
	foco = Ajustar(estado, foco)
	projetoFocado := estado.Projetos[foco.Projeto]
	if len(projetoFocado.Celulas) == 0 {
		return d
	}

	if foco.Cheia {
		celula := projetoFocado.Celulas[foco.Celula]
		miolo := MioloDaCelulaCheia(largura, altura)
		d.miolos[celula.ID] = miolo
		d.origens[celula.ID] = [2]int{1, 2}
		return d
	}

	corpo := altura - alturaDaBarra - 1 // o cabeçalho e o rodapé
	d.colunas = repartirColunas(estado, foco, largura, corpo)

	for _, c := range d.colunas {
		d.escondidas += c.escondidas
		y := alturaDaBarra // o cabeçalho
		for _, f := range c.faixas {
			if f.abre {
				y++
			}
			projeto := estado.Projetos[f.projeto]
			x := c.x
			for i, larguraCelula := range repartirLargura(len(f.celulas), c.largura) {
				celula := projeto.Celulas[f.celulas[i]]
				d.miolos[celula.ID] = Geometria{
					Colunas: max(larguraCelula-2, 1),
					Linhas:  max(f.altura-2, 1),
				}
				d.origens[celula.ID] = [2]int{x + 1, y + 1}
				x += larguraCelula
			}
			y += f.altura
		}
	}
	return d
}

// repartirColunas divide os projetos em colunas e planeja as fileiras de cada
// uma. É o que faz o mosaico crescer para o lado antes de crescer para baixo.
func repartirColunas(estado protocolo.Estado, foco Foco, largura, corpo int) []coluna {
	quantas := quantasColunas(len(estado.Projetos), largura, corpo)
	porColuna := (len(estado.Projetos) + quantas - 1) / quantas
	quantas = (len(estado.Projetos) + porColuna - 1) / porColuna
	// uma coluna de respiro entre uma coluna de projetos e a outra, senão a
	// divisória de uma parece continuar na outra.
	larguras := repartirLargura(quantas, largura-(quantas-1))

	colunas := make([]coluna, 0, quantas)
	x := 0
	for inicio := 0; inicio < len(estado.Projetos); inicio += porColuna {
		c := coluna{x: x, largura: larguras[len(colunas)]}
		todas := planejarFaixas(estado, inicio, min(inicio+porColuna, len(estado.Projetos)), c.largura, corpo)
		visiveis := janelaDeFaixas(todas, faixaDoFoco(todas, foco), corpo)
		c.escondidas = celulasDeFora(todas, visiveis)
		c.faixas = repartirAltura(visiveis, corpo)
		colunas = append(colunas, c)
		x += c.largura + 1
	}
	return colunas
}

// quantasColunas decide em quantas colunas a tela se divide: uma só enquanto
// os projetos couberem na altura com folga, mais quando a alternativa seria
// faixa espremida ou célula fora da tela.
func quantasColunas(projetos, largura, corpo int) int {
	if projetos <= 1 {
		return 1
	}
	cabemNaAltura := max(corpo/alturaMinimaDeProjeto, 1)
	precisa := (projetos + cabemNaAltura - 1) / cabemNaAltura
	return min(precisa, max(largura/larguraMinimaDeColuna, 1), projetos)
}

// planejarFaixas quebra cada projeto em fileiras de células que cabem na
// largura, equilibradas para não sobrar uma fileira com uma célula só. Quando
// as fileiras não cabem na altura que o projeto tem, as células apertam em vez
// de descer para fora da tela.
func planejarFaixas(estado protocolo.Estado, inicio, fim, largura, corpo int) []faixa {
	alturaDoProjeto := max(corpo/max(fim-inicio, 1)-1, alturaMinimaDeCelula)
	fileirasQueCabem := max(alturaDoProjeto/alturaMinimaDeCelula, 1)
	var faixas []faixa
	for iProjeto := inicio; iProjeto < fim; iProjeto++ {
		projeto := estado.Projetos[iProjeto]
		total := len(projeto.Celulas)
		if total == 0 {
			continue
		}
		fileiras := min(fileirasQuadradas(total), fileirasQueCabem)
		cabemNaLargura := min(max(largura/larguraMinimaDeCelula, 1), (total+fileiras-1)/fileiras)
		porFileira := equilibrar(total, cabemNaLargura)
		if (total+porFileira-1)/porFileira > fileirasQueCabem {
			apertadas := min((total+fileirasQueCabem-1)/fileirasQueCabem, max(largura/larguraApertadaDeCelula, 1))
			porFileira = equilibrar(total, max(apertadas, porFileira))
		}
		for inicio := 0; inicio < total; inicio += porFileira {
			fim := min(inicio+porFileira, total)
			indices := make([]int, 0, fim-inicio)
			for i := inicio; i < fim; i++ {
				indices = append(indices, i)
			}
			faixas = append(faixas, faixa{projeto: iProjeto, celulas: indices, abre: inicio == 0})
		}
	}
	return faixas
}

// fileirasQuadradas diz em quantas fileiras as células de um projeto ficam com
// a grade mais quadrada: quatro células viram 2x2 em vez de uma tira de quatro,
// duas continuam lado a lado. É o arredondamento da raiz quadrada do total.
func fileirasQuadradas(total int) int {
	fileiras := 1
	for fileiras*(fileiras+1) < total {
		fileiras++
	}
	return fileiras
}

// equilibrar diz quantas células põr por fileira para nenhuma fileira ficar
// muito mais vazia que a outra, respeitando o teto de quantas cabem.
func equilibrar(total, cabem int) int {
	fileiras := (total + cabem - 1) / cabem
	return (total + fileiras - 1) / fileiras
}

// faixaDoFoco acha em que fileira está a célula focada.
func faixaDoFoco(faixas []faixa, foco Foco) int {
	for i, f := range faixas {
		if f.projeto != foco.Projeto {
			continue
		}
		for _, celula := range f.celulas {
			if celula == foco.Celula {
				return i
			}
		}
	}
	return 0
}

// janelaDeFaixas escolhe quais fileiras aparecem quando não cabem todas,
// mantendo a do foco sempre visível.
func janelaDeFaixas(faixas []faixa, focada, corpo int) []faixa {
	cabem := max(corpo/(alturaMinimaDeCelula+1), 1)
	if cabem >= len(faixas) {
		return faixas
	}
	primeira := min(max(focada-cabem/2, 0), len(faixas)-cabem)
	janela := append([]faixa{}, faixas[primeira:primeira+cabem]...)
	// A primeira fileira da janela sempre diz de que projeto ela é, mesmo que o
	// nome do projeto tenha ficado acima do corte.
	janela[0].abre = true
	return janela
}

func celulasDeFora(todas, visiveis []faixa) int {
	contar := func(faixas []faixa) int {
		total := 0
		for _, f := range faixas {
			total += len(f.celulas)
		}
		return total
	}
	return contar(todas) - contar(visiveis)
}

// repartirAltura divide a altura entre as fileiras, descontando as linhas de
// nome de projeto. A sobra da divisão vai para as primeiras.
func repartirAltura(faixas []faixa, corpo int) []faixa {
	if len(faixas) == 0 {
		return nil
	}
	cabecalhos := 0
	for _, f := range faixas {
		if f.abre {
			cabecalhos++
		}
	}
	disponivel := max(corpo-cabecalhos, len(faixas))
	base := disponivel / len(faixas)
	resto := disponivel % len(faixas)

	repartidas := append([]faixa{}, faixas...)
	for i := range repartidas {
		repartidas[i].altura = base
		if i < resto {
			repartidas[i].altura++
		}
	}
	return repartidas
}

// repartirLargura divide a largura entre as células de uma fileira.
func repartirLargura(quantas, largura int) []int {
	if quantas == 0 {
		return nil
	}
	base := largura / quantas
	resto := largura % quantas
	larguras := make([]int, quantas)
	for i := range larguras {
		larguras[i] = base
		if i < resto {
			larguras[i]++
		}
	}
	return larguras
}

// Desenhar monta o mosaico: todas as células abertas ao mesmo tempo, agrupadas
// por projeto, cada uma ocupando o máximo de espaço que a tela permite.
func Desenhar(estado protocolo.Estado, foco Foco, modo teclado.Modo, largura, altura int, erro string) string {
	if len(estado.Projetos) == 0 {
		return telaVazia(modo, largura, altura, erro, estado.Aviso)
	}
	foco = Ajustar(estado, foco)
	projeto := estado.Projetos[foco.Projeto]
	if foco.Cheia && len(projeto.Celulas) > 0 {
		return DesenharCheia(projeto, projeto.Celulas[foco.Celula], modo, largura, altura, erro)
	}

	d := Dispor(estado, foco, largura, altura)
	if len(d.colunas) == 0 {
		return telaVazia(modo, largura, altura, erro, estado.Aviso)
	}

	corpo := altura - alturaDaBarra - 1
	blocos := make([][]string, len(d.colunas))
	for i, c := range d.colunas {
		blocos[i] = blocoDaColuna(estado, c, foco, modo)
	}

	linhas := barraDeTitulo(modo, largura, contarChamados(estado), estado.Quota)
	for l := range corpo {
		var linha strings.Builder
		for i, bloco := range blocos {
			if i > 0 {
				linha.WriteString(" ")
			}
			if l < len(bloco) {
				linha.WriteString(bloco[l])
				continue
			}
			linha.WriteString(strings.Repeat(" ", d.colunas[i].largura))
		}
		linhas = append(linhas, linha.String())
	}
	if d.escondidas > 0 && len(linhas) > 1 {
		linhas[len(linhas)-1] = marcarEscondidas(linhas[len(linhas)-1], d.escondidas, largura)
	}
	for len(linhas) < altura-1 {
		linhas = append(linhas, strings.Repeat(" ", largura))
	}
	linhas = linhas[:altura-1]
	return strings.Join(append(linhas, rodape(modo, largura, erro)), "\n")
}

// blocoDaColuna desenha uma coluna inteira: cada projeto com sua divisória e
// as fileiras de células dele, todas na largura da coluna.
func blocoDaColuna(estado protocolo.Estado, c coluna, foco Foco, modo teclado.Modo) []string {
	var linhas []string
	for _, f := range c.faixas {
		projeto := estado.Projetos[f.projeto]
		if f.abre {
			linhas = append(linhas, divisoriaDoProjeto(projeto, c.largura, modo, f.projeto == foco.Projeto))
		}
		linhas = append(linhas, fileiraDeCelulas(projeto, f, foco, modo, c.largura)...)
	}
	return linhas
}

// divisoriaDoProjeto é a linha que separa um projeto do outro: o nome, o
// caminho e o estado da stack.
func divisoriaDoProjeto(projeto protocolo.Projeto, largura int, modo teclado.Modo, focado bool) string {
	pintarNome := corProjeto(projeto.Cor).Bold(focado)
	pintarTraco := corBorda
	if focado {
		pintarTraco = corGradeAtiva
	}
	if !focado {
		pintarNome = corApagada
	}
	if modo == teclado.Digitar {
		pintarNome, pintarTraco = corApagada, corApagada
	}

	marca := "──"
	if focado {
		marca = "━━"
	}
	esquerda := marca + " " + strings.ToUpper(projeto.Nome) + " "

	var sinais []string
	chamados := contarChamadosDoProjeto(projeto)
	for _, estado := range []string{"respondeu", "aprovar"} {
		if chamados[estado] > 0 {
			sinais = append(sinais, marcadorDe(estado).simbolo+strconv.Itoa(chamados[estado]))
		}
	}
	if projeto.TemCompose {
		docker := projeto.Docker
		if docker == "" {
			docker = "parado"
		}
		sinais = append(sinais, "● "+docker)
	}
	direita := ""
	if len(sinais) > 0 {
		direita = " " + strings.Join(sinais, "  ") + " "
	}

	caminho := " " + encurtarCaminho(projeto.Caminho, max(largura/3, 10)) + " "
	if lipgloss.Width(esquerda)+lipgloss.Width(caminho)+lipgloss.Width(direita)+4 > largura {
		caminho = ""
	}

	enfeite := largura - lipgloss.Width(esquerda) - lipgloss.Width(caminho) - lipgloss.Width(direita)
	if enfeite < 0 {
		esquerda = cortar(esquerda, max(largura-lipgloss.Width(direita), 1))
		caminho, enfeite = "", max(largura-lipgloss.Width(esquerda)-lipgloss.Width(direita), 0)
	}

	return pintarTraco.Render(marca) + pintarNome.Render(strings.TrimPrefix(esquerda, marca)) +
		corApagada.Render(caminho) +
		pintarTraco.Render(strings.Repeat("─", enfeite)) +
		pintarTraco.Render(direita)
}

// fileiraDeCelulas desenha uma fileira: as células lado a lado, cada uma com
// sua caixa.
func fileiraDeCelulas(projeto protocolo.Projeto, f faixa, foco Foco, modo teclado.Modo, largura int) []string {
	larguras := repartirLargura(len(f.celulas), largura)
	caixas := make([][]string, len(f.celulas))
	for i, indice := range f.celulas {
		focada := f.projeto == foco.Projeto && indice == foco.Celula
		caixas[i] = caixaDaCelula(projeto.Celulas[indice], modo, focada, larguras[i], f.altura)
	}

	linhas := make([]string, f.altura)
	for l := range linhas {
		var linha strings.Builder
		for i, caixa := range caixas {
			if l < len(caixa) {
				linha.WriteString(caixa[l])
				continue
			}
			linha.WriteString(strings.Repeat(" ", larguras[i]))
		}
		linhas[l] = linha.String()
	}
	return linhas
}

// marcarEscondidas avisa, no rodapé do mosaico, quantas células ficaram fora da
// tela. Some o desenho, nunca a contagem.
func marcarEscondidas(linha string, quantas, largura int) string {
	aviso := corRolagem.Render(" +" + strconv.Itoa(quantas) + " célula(s) fora da tela ")
	corte := max(largura-lipgloss.Width(aviso), 0)
	return cortar(linha, corte) + aviso
}

// caixaDaCelula desenha uma célula: nome e marcador na borda de cima, conteúdo
// dentro. Uma borda, um significado — grossa e verde quando o teclado é dela.
func caixaDaCelula(celula protocolo.Celula, modo teclado.Modo, focada bool, largura, altura int) []string {
	traco := [6]string{"┌", "┐", "└", "┘", "─", "│"}
	if focada && modo == teclado.Digitar {
		traco = [6]string{"┏", "┓", "┗", "┛", "━", "┃"}
	}
	// Célula sem o foco é célula apagada por inteiro: miolo e borda. A cor da
	// estrutura fica para a que está com o cursor.
	pintarQuadro := corApagada
	switch {
	case focada && modo == teclado.Digitar:
		pintarQuadro = corDigitando
	case focada:
		pintarQuadro = corBordaFocada
	}

	marcador := marcadorDe(celula.Estado)
	rotulo := " " + rotuloDaCelula(celula) + " "
	estado := " " + marcador.simbolo + " " + strings.ToUpper(celula.Estado) + " "
	pintarEstado := marcador.selo()
	if !celula.AoVivo {
		estado = " ▲ " + strconv.Itoa(celula.Rolagem) + " "
		pintarEstado = chip(corRolagem)
	}

	miolo := max(largura-2, 1)
	enfeite := miolo - lipgloss.Width(rotulo) - lipgloss.Width(estado)
	if enfeite < 0 {
		estado, pintarEstado = " "+marcador.simbolo+" ", marcador.cor()
		enfeite = miolo - lipgloss.Width(rotulo) - lipgloss.Width(estado)
	}
	if enfeite < 0 {
		rotulo = cortar(rotulo, max(miolo-lipgloss.Width(estado), 1))
		enfeite = max(miolo-lipgloss.Width(rotulo)-lipgloss.Width(estado), 0)
	}

	pintarNome := corTitulo
	switch {
	case focada && modo == teclado.Digitar:
		pintarNome = corDigitando.Bold(true)
	case !focada:
		pintarNome = corBarra
	}

	// Urgência é área preenchida, não matiz: a célula que trava o trabalho
	// vira uma barra sólida invertida na linha inteira do cabeçalho. Só as
	// laterais ficam de fora, e a célula que está com o teclado escapa —
	// lá o verde manda.
	pintarTopo := pintarQuadro
	if marcador.bloqueia && celula.AoVivo && !(focada && modo == teclado.Digitar) {
		barra := marcador.cor()
		pintarTopo, pintarNome, pintarEstado = barra, barra, barra
	}

	linhas := []string{
		pintarTopo.Render(traco[0]) +
			pintarNome.Render(rotulo) +
			pintarTopo.Render(strings.Repeat(traco[4], enfeite)) +
			pintarEstado.Render(estado) +
			pintarTopo.Render(traco[1]),
	}
	// A lateral é a mesma em todas as linhas da caixa: pintar uma vez e repetir
	// custa muito menos do que pedir o mesmo desenho a cada linha, e o miolo de
	// uma célula tem dezenas delas.
	lateral := pintarQuadro.Render(traco[5])
	for i := range max(altura-2, 0) {
		conteudo := ""
		if i < len(celula.Linhas) {
			conteudo = celula.Linhas[i]
			if !focada {
				conteudo = apagar(conteudo)
			}
		}
		linhas = append(linhas, lateral+preencher(conteudo, miolo)+lateral)
	}
	if altura >= 2 {
		linhas = append(linhas, pintarQuadro.Render(traco[2]+strings.Repeat(traco[4], miolo)+traco[3]))
	}
	return linhas
}

// Ajustar prende o foco dentro do que existe.
func Ajustar(estado protocolo.Estado, foco Foco) Foco {
	if len(estado.Projetos) == 0 {
		return Foco{}
	}
	foco.Projeto = min(max(foco.Projeto, 0), len(estado.Projetos)-1)
	celulas := len(estado.Projetos[foco.Projeto].Celulas)
	if celulas == 0 {
		foco.Celula = 0
		return foco
	}
	foco.Celula = min(max(foco.Celula, 0), celulas-1)
	return foco
}

// OrigemNoMosaico diz em que linha e coluna da tela começa o miolo de uma
// célula. É o que põe o cursor no lugar certo dentro da caixa.
func OrigemNoMosaico(estado protocolo.Estado, foco Foco, largura, altura int, id string) (int, int, bool) {
	origem, tem := Dispor(estado, foco, largura, altura).origens[id]
	return origem[0], origem[1], tem
}
