package tela

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

const (
	// larguraTira é o quanto sobra de um projeto que não está em foco: some o
	// texto, nunca o sinal.
	larguraTira = 4
	// larguraMinimaDeLeitura é o piso da coluna focada. Abaixo disso a coluna
	// não serve para ler nada, e as tiras mais distantes cedem lugar.
	larguraMinimaDeLeitura = 30
	// alturaMinimaDaCelula é o mínimo para uma célula mostrar alguma coisa:
	// borda de cima, uma linha de conteúdo e borda de baixo.
	alturaMinimaDaCelula = 3
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

// Disposicao é o resultado do cálculo de layout: quanto cada coluna ocupa e que
// espaço sobrou para o miolo de cada célula visível. Desenhar e avisar o
// tamanho ao motor saem daqui, para nunca discordarem.
type Disposicao struct {
	larguras []int
	visiveis []int
	miolos   map[string]Geometria
}

// Miolos é o que a tela avisa ao motor: o tamanho útil de cada célula visível.
func (d Disposicao) Miolos() map[string]Geometria { return d.miolos }

// Dispor calcula o layout do mosaico.
func Dispor(estado protocolo.Estado, foco Foco, largura, altura int) Disposicao {
	d := Disposicao{miolos: map[string]Geometria{}}
	if len(estado.Projetos) == 0 || largura < 12 || altura < 6 {
		return d
	}
	foco = Ajustar(estado, foco)
	projeto := estado.Projetos[foco.Projeto]
	if len(projeto.Celulas) == 0 {
		return d
	}

	if foco.Cheia {
		celula := projeto.Celulas[foco.Celula]
		d.miolos[celula.ID] = MioloDaCelulaCheia(largura, altura)
		d.visiveis = []int{foco.Celula}
		return d
	}

	// Uma coluna por projeto: a focada com largura de leitura, as outras em
	// tira. Com projetos demais, as tiras mais distantes do foco saem antes de
	// a coluna focada ficar ilegível.
	d.larguras = larguraDasColunas(len(estado.Projetos), foco.Projeto, largura)

	alturaCorpo := altura - 4 // barra, borda de cima, borda de baixo, rodapé
	d.visiveis = celulasQueCabem(len(projeto.Celulas), foco.Celula, alturaCorpo)

	larguraColuna := d.larguras[foco.Projeto]
	for i, alturaCelula := range alturasDasCelulas(len(d.visiveis), alturaCorpo) {
		celula := projeto.Celulas[d.visiveis[i]]
		d.miolos[celula.ID] = Geometria{
			Colunas: max(larguraColuna-3, 1), // a borda da coluna e as duas da célula
			Linhas:  max(alturaCelula-2, 1),  // a borda de cima e a de baixo da célula
		}
	}
	return d
}

// larguraDasColunas reparte a largura entre os projetos.
func larguraDasColunas(quantos, focado, largura int) []int {
	util := largura - 1 // a borda direita do quadro
	larguras := make([]int, quantos)

	cabem := (util - larguraMinimaDeLeitura) / larguraTira
	cabem = min(max(cabem, 0), quantos-1)

	// As tiras que ficam são as mais próximas do foco, para os dois lados.
	mantidas := map[int]bool{}
	for passo := 1; len(mantidas) < cabem; passo++ {
		if i := focado - passo; i >= 0 && len(mantidas) < cabem {
			mantidas[i] = true
		}
		if i := focado + passo; i < quantos && len(mantidas) < cabem {
			mantidas[i] = true
		}
		if focado-passo < 0 && focado+passo >= quantos {
			break
		}
	}

	sobra := util
	for i := range larguras {
		if i != focado && mantidas[i] {
			larguras[i] = larguraTira
			sobra -= larguraTira
		}
	}
	larguras[focado] = sobra
	return larguras
}

// celulasQueCabem escolhe quais células da coluna focada aparecem quando não
// cabem todas, mantendo a focada sempre visível.
func celulasQueCabem(quantas, focada, alturaCorpo int) []int {
	if quantas == 0 {
		return nil
	}
	cabem := min(max(alturaCorpo/alturaMinimaDaCelula, 1), quantas)
	primeira := min(max(focada-cabem/2, 0), quantas-cabem)
	visiveis := make([]int, cabem)
	for i := range visiveis {
		visiveis[i] = primeira + i
	}
	return visiveis
}

// alturasDasCelulas reparte a altura da coluna entre as células visíveis. A
// sobra da divisão vai para as primeiras, para não deixar linha em branco.
func alturasDasCelulas(quantas, alturaCorpo int) []int {
	if quantas == 0 {
		return nil
	}
	base := alturaCorpo / quantas
	resto := alturaCorpo % quantas
	alturas := make([]int, quantas)
	for i := range alturas {
		alturas[i] = base
		if i < resto {
			alturas[i]++
		}
	}
	return alturas
}

// Desenhar monta o mosaico: todos os projetos ao mesmo tempo, cada um numa
// coluna, a focada larga e as outras em tira.
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
	if len(d.larguras) == 0 {
		return telaVazia(modo, largura, altura, erro, estado.Aviso)
	}

	alturaCorpo := altura - 4
	corpo := make([][]string, len(estado.Projetos))
	for i, p := range estado.Projetos {
		if d.larguras[i] == 0 {
			continue
		}
		if i == foco.Projeto {
			corpo[i] = colunaFocada(p, d, foco, modo, alturaCorpo)
			continue
		}
		corpo[i] = tira(p, d.larguras[i], alturaCorpo, modo)
	}

	linhas := []string{barraDeTitulo(modo, largura, contarChamados(estado), estado.Quota)}
	linhas = append(linhas, bordaDoQuadro(estado, d, foco, true))
	for l := range alturaCorpo {
		var linha strings.Builder
		for i := range estado.Projetos {
			if d.larguras[i] == 0 {
				continue
			}
			linha.WriteString(corpo[i][l])
		}
		linha.WriteString(corBorda.Render("│"))
		linhas = append(linhas, linha.String())
	}
	linhas = append(linhas, bordaDoQuadro(estado, d, foco, false))
	linhas = append(linhas, rodape(modo, largura, erro))
	return strings.Join(linhas, "\n")
}

// bordaDoQuadro desenha a linha de cima, com o nome do projeto focado, ou a de
// baixo.
func bordaDoQuadro(estado protocolo.Estado, d Disposicao, foco Foco, emCima bool) string {
	esquerdo, junta, direito := "┌", "┬", "┐"
	if !emCima {
		esquerdo, junta, direito = "└", "┴", "┘"
	}

	var linha strings.Builder
	primeira := true
	for i := range estado.Projetos {
		w := d.larguras[i]
		if w == 0 {
			continue
		}
		canto := junta
		if primeira {
			canto, primeira = esquerdo, false
		}
		linha.WriteString(corBorda.Render(canto))
		if emCima && i == foco.Projeto {
			linha.WriteString(tituloDoProjeto(estado.Projetos[i], w-1))
			continue
		}
		linha.WriteString(corBorda.Render(strings.Repeat("─", w-1)))
	}
	linha.WriteString(corBorda.Render(direito))
	return linha.String()
}

// tituloDoProjeto centraliza o nome do projeto na borda de cima da coluna.
func tituloDoProjeto(projeto protocolo.Projeto, largura int) string {
	nome := " " + strings.ToUpper(projeto.Nome) + " "
	if lipgloss.Width(nome) > largura {
		nome = " " + strings.ToUpper(cortar(projeto.Nome, max(largura-2, 1))) + " "
	}
	sobra := max(largura-lipgloss.Width(nome), 0)
	esquerda := sobra / 2
	return corBorda.Render(strings.Repeat("─", esquerda)) +
		corProjeto(projeto.Cor).Render(nome) +
		corBorda.Render(strings.Repeat("─", sobra-esquerda))
}

// colunaFocada empilha as células do projeto em foco, cada uma na sua caixa.
func colunaFocada(projeto protocolo.Projeto, d Disposicao, foco Foco, modo teclado.Modo, alturaCorpo int) []string {
	largura := d.larguras[foco.Projeto]
	var linhas []string
	for i, alturaCelula := range alturasDasCelulas(len(d.visiveis), alturaCorpo) {
		indice := d.visiveis[i]
		focada := indice == foco.Celula
		linhas = append(linhas, caixaDaCelula(projeto.Celulas[indice], modo, focada, largura-1, alturaCelula)...)
	}
	for len(linhas) < alturaCorpo {
		linhas = append(linhas, strings.Repeat(" ", largura-1))
	}
	for i, linha := range linhas[:alturaCorpo] {
		linhas[i] = corBorda.Render("│") + linha
	}
	return linhas[:alturaCorpo]
}

// tira é o projeto que não está em foco: some o texto, nunca o sinal. Mostra,
// de cima para baixo, o nome na vertical, quantas células, quantas pedem
// atenção e o estado do Docker.
func tira(projeto protocolo.Projeto, largura, alturaCorpo int, modo teclado.Modo) []string {
	conteudo := largura - 1
	var sinais []string
	for _, letra := range strings.ToUpper(projeto.Nome) {
		sinais = append(sinais, string(letra))
	}
	chamados := contarChamadosDoProjeto(projeto)
	sinais = append(sinais, "", strconv.Itoa(len(projeto.Celulas)))
	if chamados["respondeu"] > 0 {
		sinais = append(sinais, marcadorDe("respondeu").simbolo+strconv.Itoa(chamados["respondeu"]))
	}
	if chamados["aprovar"] > 0 {
		sinais = append(sinais, marcadorDe("aprovar").simbolo+strconv.Itoa(chamados["aprovar"]))
	}
	if projeto.TemCompose {
		sinais = append(sinais, "●")
		if projeto.Docker != "" {
			sinais = append(sinais, projeto.Docker)
		}
	}

	pintar := corProjeto(projeto.Cor)
	if modo == teclado.Digitar {
		pintar = corApagada
	}
	linhas := make([]string, alturaCorpo)
	for i := range linhas {
		texto := ""
		if i < len(sinais) {
			texto = sinais[i]
		}
		linhas[i] = corBorda.Render("│") + pintar.Render(centralizarEm(cortar(texto, conteudo), conteudo))
	}
	return linhas
}

// caixaDaCelula desenha uma célula: nome e marcador na borda de cima, conteúdo
// dentro. Uma borda, um significado — grossa quando o teclado é da célula.
func caixaDaCelula(celula protocolo.Celula, modo teclado.Modo, focada bool, largura, altura int) []string {
	traco := [6]string{"┌", "┐", "└", "┘", "─", "│"}
	if focada && modo == teclado.Digitar {
		traco = [6]string{"┏", "┓", "┗", "┛", "━", "┃"}
	}
	pintarQuadro := corBorda
	switch {
	case modo == teclado.Digitar && !focada:
		pintarQuadro = corApagada
	case focada:
		pintarQuadro = corBordaFocada
	}

	marcador := marcadorDe(celula.Estado)
	rotulo := " " + celula.Tipo + " · " + celula.Nome + " "
	estado := " " + marcador.simbolo + " " + strings.ToUpper(celula.Estado) + " "
	pintarEstado := marcador.cor
	if !celula.AoVivo {
		estado = " ▲ " + strconv.Itoa(celula.Rolagem) + " "
		pintarEstado = corRolagem
	}

	miolo := max(largura-2, 1)
	enfeite := miolo - lipgloss.Width(rotulo) - lipgloss.Width(estado)
	if enfeite < 0 {
		estado, pintarEstado = " "+marcador.simbolo+" ", marcador.cor
		enfeite = miolo - lipgloss.Width(rotulo) - lipgloss.Width(estado)
	}
	if enfeite < 0 {
		rotulo = cortar(rotulo, max(miolo-lipgloss.Width(estado), 1))
		enfeite = max(miolo-lipgloss.Width(rotulo)-lipgloss.Width(estado), 0)
	}

	pintarNome := corTitulo
	if !focada {
		pintarNome = corBarra
	}
	linhas := []string{
		pintarQuadro.Render(traco[0]) +
			pintarNome.Render(rotulo) +
			pintarQuadro.Render(strings.Repeat(traco[4], enfeite)) +
			pintarEstado.Render(estado) +
			pintarQuadro.Render(traco[1]),
	}
	for i := range max(altura-2, 0) {
		conteudo := ""
		if i < len(celula.Linhas) {
			conteudo = celula.Linhas[i]
		}
		linhas = append(linhas, pintarQuadro.Render(traco[5])+preencher(conteudo, miolo)+pintarQuadro.Render(traco[5]))
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
	d := Dispor(estado, foco, largura, altura)
	if len(d.larguras) == 0 {
		return 0, 0, false
	}
	foco = Ajustar(estado, foco)
	x := 0
	for i := range foco.Projeto {
		x += d.larguras[i]
	}
	y := 2 // a barra de título e a borda de cima do quadro
	projeto := estado.Projetos[foco.Projeto]
	for i, alturaCelula := range alturasDasCelulas(len(d.visiveis), altura-4) {
		if projeto.Celulas[d.visiveis[i]].ID == id {
			return x + 2, y + 1, true
		}
		y += alturaCelula
	}
	return 0, 0, false
}
