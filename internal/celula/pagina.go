package celula

import (
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"

	"github.com/andreluiz/tesseract/internal/tema"
)

// A aba de markdown desenha o arquivo como uma página, não como saída de
// terminal: medida de leitura fixa, margem dos dois lados, título com peso e
// espaço em volta dos blocos. Ler documentação longa numa tela larga é o motivo
// de o Tesseract existir.
const (
	// medidaDeLeitura é a largura de texto que se lê sem cansar. Acima disso a
	// linha fica comprida demais e o olho perde a volta.
	medidaDeLeitura = 88
	// margemDaPagina é o respiro entre a borda da célula e o texto.
	margemDaPagina = 2
)

// estiloDaPagina é o tema do markdown renderizado.
var estiloDaPagina = montarEstiloDaPagina()

func montarEstiloDaPagina() ansi.StyleConfig {
	estilo := styles.DarkStyleConfig

	texto := func(s string) *string { return &s }
	sim := func() *bool { verdade := true; return &verdade }
	nao := func() *bool { falso := false; return &falso }
	numero := func(n uint) *uint { return &n }

	// A margem é aplicada aqui fora, junto com a centralização da página.
	estilo.Document.Margin = numero(0)
	estilo.Document.BlockPrefix = ""
	estilo.Document.BlockSuffix = "\n"
	estilo.Document.Color = texto(tema.FgDefault)

	// Título: faixa sólida ocupando a linha, como capítulo de livro. A faixa é
	// ciano porque título é estrutura do documento, não estado.
	estilo.H1.Prefix = "  "
	estilo.H1.Suffix = "  "
	estilo.H1.Color = texto(tema.Flux)
	estilo.H1.BackgroundColor = texto(tema.FluxDeep)
	estilo.H1.Bold = sim()
	estilo.H1.Upper = sim()
	estilo.H1.BlockPrefix = "\n"
	estilo.H1.BlockSuffix = "\n"

	// Seções: barra à esquerda em vez dos sustenidos do markdown. A hierarquia
	// desce pelo brilho, não pelo matiz — ciano forte, ciano fraco, cinza.
	estilo.H2.Prefix = "▌ "
	estilo.H2.Color = texto(tema.Flux)
	estilo.H2.Bold = sim()
	estilo.H2.BlockPrefix = "\n"
	estilo.H2.BlockSuffix = "\n"

	estilo.H3.Prefix = "▏ "
	estilo.H3.Color = texto(tema.FluxCore)
	estilo.H3.Bold = sim()
	estilo.H3.BlockPrefix = "\n"

	estilo.H4.Prefix = "· "
	estilo.H4.Color = texto(tema.FgBright)
	estilo.H4.Bold = sim()
	estilo.H5.Prefix = "· "
	estilo.H5.Color = texto(tema.FgMuted)
	estilo.H6.Prefix = "· "
	estilo.H6.Color = texto(tema.FgFaint)
	estilo.H6.Bold = nao()

	// Citação com filete à esquerda, em itálico apagado.
	estilo.BlockQuote.IndentToken = texto("┃ ")
	estilo.BlockQuote.Color = texto(tema.FgMuted)
	estilo.BlockQuote.Italic = sim()

	// Régua: uma linha inteira, não oito traços.
	estilo.HorizontalRule.Color = texto(tema.LineDim)
	estilo.HorizontalRule.Format = "\n" + strings.Repeat("─", medidaDeLeitura-4) + "\n"

	// Listas com marcador redondo e espaço para respirar.
	estilo.Item.BlockPrefix = "• "
	estilo.Item.Color = texto(tema.FgDefault)
	estilo.Enumeration.BlockPrefix = ". "
	estilo.Enumeration.Color = texto(tema.FluxCore)

	// Código: fundo próprio, como caixa de código de livro técnico.
	estilo.Code.Color = texto(tema.FluxCore)
	estilo.Code.BackgroundColor = texto(tema.BgRaised)
	estilo.Code.Prefix = " "
	estilo.Code.Suffix = " "
	estilo.CodeBlock.Margin = numero(2)
	// Sem tema de terceiro: o realce de sintaxe usa a mesma paleta do resto.
	estilo.CodeBlock.Theme = ""
	estilo.CodeBlock.Chroma = realceDoCodigo()

	// Links legíveis, sem virar ruído.
	estilo.Link.Color = texto(tema.Flux)
	estilo.Link.Underline = sim()
	estilo.LinkText.Color = texto(tema.FluxCore)
	estilo.LinkText.Bold = sim()

	// Tabela com filete fino.
	estilo.Table.CenterSeparator = texto("┼")
	estilo.Table.ColumnSeparator = texto("│")
	estilo.Table.RowSeparator = texto("─")

	estilo.Emph.Italic = sim()
	estilo.Strong.Bold = sim()
	estilo.Strong.Color = texto(tema.FgBright)
	return estilo
}

// realceDoCodigo é o realce de sintaxe dentro do bloco de código, na paleta do
// Tesseract. É a mesma leitura do tema do editor: string em verde escuro,
// função em azul, palavra-chave em roxo, tipo em amarelo, número em laranja.
// Verde phosphor não entra — um bloco de código repetiria a cor de posse do
// teclado dezenas de vezes.
func realceDoCodigo() *ansi.Chroma {
	cor := func(c string) ansi.StylePrimitive { return ansi.StylePrimitive{Color: &c} }
	negrito := func(c string) ansi.StylePrimitive {
		verdade := true
		return ansi.StylePrimitive{Color: &c, Bold: &verdade}
	}
	italico := func(c string) ansi.StylePrimitive {
		verdade := true
		return ansi.StylePrimitive{Color: &c, Italic: &verdade}
	}
	fundo := func(c string) ansi.StylePrimitive { return ansi.StylePrimitive{BackgroundColor: &c} }
	return &ansi.Chroma{
		Text:                cor(tema.FgDefault),
		Error:               cor(tema.StateDead),
		Comment:             italico(tema.FgFaint),
		CommentPreproc:      cor(tema.Flux),
		Keyword:             cor(tema.StateOrphan),
		KeywordReserved:     cor(tema.StateOrphan),
		KeywordNamespace:    cor(tema.StateOrphan),
		KeywordType:         cor(tema.CorAnsiAmarelo),
		Operator:            cor(tema.FgMuted),
		Punctuation:         cor(tema.FgMuted),
		Name:                cor(tema.FgDefault),
		NameBuiltin:         cor(tema.CorAnsiMagenta),
		NameTag:             cor(tema.StateOrphan),
		NameAttribute:       cor(tema.Flux),
		NameClass:           negrito(tema.CorAnsiAmarelo),
		NameConstant:        cor(tema.StateBlock),
		NameDecorator:       cor(tema.Flux),
		NameException:       cor(tema.StateDead),
		NameFunction:        cor(tema.StateRead),
		NameOther:           cor(tema.FgDefault),
		Literal:             cor(tema.StateBlock),
		LiteralNumber:       cor(tema.StateBlock),
		LiteralDate:         cor(tema.StateBlock),
		LiteralString:       cor(tema.BrandCore),
		LiteralStringEscape: cor(tema.Flux),
		GenericDeleted:      cor(tema.StateDead),
		GenericEmph:         italico(tema.FgDefault),
		GenericInserted:     cor(tema.BrandCore),
		GenericStrong:       negrito(tema.FgBright),
		GenericSubheading:   cor(tema.FluxCore),
		Background:          fundo(tema.BgSurface),
	}
}

// renderizarPagina desenha o markdown como página: medida de leitura para o
// texto, centralizada na largura que a célula tem, e espaço extra quando o
// documento tem código largo — código quebrado no meio vira ruído ilegível.
func renderizarPagina(bruto string, colunas int) []string {
	largura := larguraDaPagina(colunas, bruto)
	// Código mais largo do que a página é cortado, nunca quebrado: diagrama e
	// tela de terminal partidos no meio viram ruído ilegível.
	bruto = cortarCodigoLargo(bruto, largura-4)
	desenhista, err := glamour.NewTermRenderer(
		glamour.WithStyles(estiloDaPagina),
		glamour.WithWordWrap(largura),
		glamour.WithEmoji(),
	)
	if err != nil {
		return strings.Split(bruto, "\n")
	}
	saida, err := desenhista.Render(bruto)
	if err != nil {
		return strings.Split(bruto, "\n")
	}

	recuo := strings.Repeat(" ", max((colunas-largura)/2, 0))
	linhas := strings.Split(strings.Trim(saida, "\n"), "\n")
	for i, linha := range linhas {
		linhas[i] = recuo + linha
	}
	return linhas
}

// larguraDaPagina é a medida de leitura que cabe na célula, esticada até onde
// for preciso para o código do documento não quebrar.
func larguraDaPagina(colunas int, bruto string) int {
	desejada := max(medidaDeLeitura, larguraDoCodigo(bruto)+2)
	return max(min(colunas-2*margemDaPagina, desejada), 20)
}

// cortarCodigoLargo encurta as linhas de dentro dos blocos de código que não
// cabem na página, marcando o corte.
func cortarCodigoLargo(bruto string, largura int) string {
	if largura < 10 {
		return bruto
	}
	linhas := strings.Split(bruto, "\n")
	dentro := false
	for i, linha := range linhas {
		if strings.HasPrefix(strings.TrimSpace(linha), "```") {
			dentro = !dentro
			continue
		}
		if !dentro {
			continue
		}
		if runas := []rune(linha); len(runas) > largura {
			linhas[i] = string(runas[:largura-1]) + "›"
		}
	}
	return strings.Join(linhas, "\n")
}

// larguraDoCodigo é a maior linha dentro de bloco de código do documento.
func larguraDoCodigo(bruto string) int {
	maior, dentro := 0, false
	for _, linha := range strings.Split(bruto, "\n") {
		if strings.HasPrefix(strings.TrimSpace(linha), "```") {
			dentro = !dentro
			continue
		}
		if !dentro {
			continue
		}
		if comprimento := len([]rune(linha)); comprimento > maior {
			maior = comprimento
		}
	}
	return maior
}
