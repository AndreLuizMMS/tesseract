package celula

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"

	"github.com/andreluiz/tesseract/internal/motor/historico"
)

// telaLimpaDe é a tela da célula sem os códigos de cor — o markdown renderizado
// vem cheio deles, e o que interessa no teste é o texto.
func telaLimpaDe(c Celula) string {
	var limpas []string
	for _, linha := range c.Desenhar().Linhas {
		limpas = append(limpas, historico.LimparCodigos(linha))
	}
	return strings.Join(limpas, "\n")
}

// projetoComDocumentos monta um projeto com markdown espalhado, inclusive em
// pasta que não deve ser varrida.
func projetoComDocumentos(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	arquivos := map[string]string{
		"README.md":                   "# Leia\n\ncomeço de tudo\n",
		"docs/spec-m7.md":             "# Módulo 7\n\nfichas clínicas\n",
		"docs/spec-m8.md":             "# Módulo 8\n\nagenda\n",
		"docs/guias/como-rodar.md":    "# Como rodar\n\npnpm dev\n",
		"node_modules/pacote/LEIA.md": "# não conta\n",
		"src/codigo.go":               "package main\n",
	}
	for nome, conteudo := range arquivos {
		caminho := filepath.Join(dir, nome)
		if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
			t.Fatalf("preparar: %v", err)
		}
		if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
			t.Fatalf("preparar: %v", err)
		}
	}
	return dir
}

func abaDeMarkdown(t *testing.T, dir string) Celula {
	t.Helper()
	celula, err := Nova("md")
	if err != nil {
		t.Fatalf("fabricar: %v", err)
	}
	if err := celula.Nascer(Config{ID: "c1", Diretorio: dir, Colunas: 70, Linhas: 20}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	t.Cleanup(func() { celula.Matar() })
	return celula
}

func digitarNaBusca(c Celula, texto string) {
	for _, letra := range texto {
		c.Tecla(Toque{Codigo: letra, Texto: string(letra)})
	}
}

// TestMdListaOsMarkdownsDoProjeto — a aba abre numa lista de tudo que é
// markdown ali dentro, com a busca em cima.
func TestMdListaOsMarkdownsDoProjeto(t *testing.T) {
	celula := abaDeMarkdown(t, projetoComDocumentos(t))
	tela := telaLimpaDe(celula)

	if !strings.Contains(tela, "buscar:") {
		t.Errorf("a barra de busca devia estar no topo:\n%s", tela)
	}
	for _, arquivo := range []string{"README.md", "docs/spec-m7.md", "docs/spec-m8.md", "docs/guias/como-rodar.md"} {
		if !strings.Contains(tela, arquivo) {
			t.Errorf("a lista devia mostrar %q:\n%s", arquivo, tela)
		}
	}
	if strings.Contains(tela, "node_modules") {
		t.Errorf("node_modules não entra na varredura:\n%s", tela)
	}
	if strings.Contains(tela, "codigo.go") {
		t.Errorf("a lista é só de markdown:\n%s", tela)
	}
}

// TestMdBuscaFiltraPeloNome — digitar filtra a lista.
func TestMdBuscaFiltraPeloNome(t *testing.T) {
	celula := abaDeMarkdown(t, projetoComDocumentos(t))

	digitarNaBusca(celula, "m7")
	tela := telaLimpaDe(celula)
	if !strings.Contains(tela, "spec-m7.md") {
		t.Errorf("a busca devia achar o arquivo:\n%s", tela)
	}
	if strings.Contains(tela, "spec-m8.md") || strings.Contains(tela, "README.md") {
		t.Errorf("a busca devia esconder o que não casa:\n%s", tela)
	}
	if !strings.Contains(tela, "1 de 4 documentos") {
		t.Errorf("a lista devia contar o que casou:\n%s", tela)
	}

	// Apagar devolve tudo.
	celula.Tecla(Toque{Codigo: vt.KeyBackspace})
	celula.Tecla(Toque{Codigo: vt.KeyBackspace})
	if tela := telaLimpaDe(celula); !strings.Contains(tela, "README.md") {
		t.Errorf("apagar a busca devia devolver a lista inteira:\n%s", tela)
	}
}

// TestMdEnterAbreOArquivoEscolhido — é o gesto que a aba existe para fazer.
func TestMdEnterAbreOArquivoEscolhido(t *testing.T) {
	celula := abaDeMarkdown(t, projetoComDocumentos(t))

	digitarNaBusca(celula, "como-rodar")
	celula.Tecla(Toque{Codigo: vt.KeyEnter})

	tela := telaLimpaDe(celula)
	// O título vira caixa alta na página, como capítulo.
	if !strings.Contains(tela, "COMO RODAR") || !strings.Contains(tela, "pnpm dev") {
		t.Errorf("o arquivo devia estar aberto e renderizado:\n%s", tela)
	}
	if !strings.Contains(tela, "esc volta à lista") {
		t.Errorf("a leitura devia dizer como voltar:\n%s", tela)
	}

	// E esc volta para a lista, com a busca ainda de pé.
	celula.Tecla(Toque{Codigo: vt.KeyEscape})
	tela = telaLimpaDe(celula)
	if !strings.Contains(tela, "buscar:") || !strings.Contains(tela, "como-rodar.md") {
		t.Errorf("esc devia voltar para a lista:\n%s", tela)
	}
}

// TestMdSetasAndamPelaLista — a escolha anda antes de abrir.
func TestMdSetasAndamPelaLista(t *testing.T) {
	celula := abaDeMarkdown(t, projetoComDocumentos(t))

	digitarNaBusca(celula, "spec")
	celula.Tecla(Toque{Codigo: vt.KeyDown})
	celula.Tecla(Toque{Codigo: vt.KeyEnter})

	if tela := telaLimpaDe(celula); !strings.Contains(tela, "MÓDULO 8") {
		t.Errorf("a seta devia ter movido a escolha para o segundo:\n%s", tela)
	}
}

// TestMdAbreDiretoNoArquivoPedido — criada apontando um arquivo, a aba já nasce
// nele.
func TestMdAbreDiretoNoArquivoPedido(t *testing.T) {
	dir := projetoComDocumentos(t)
	celula, err := Nova("md")
	if err != nil {
		t.Fatalf("fabricar: %v", err)
	}
	if err := celula.Nascer(Config{ID: "c1", Diretorio: dir, Alvo: "docs/spec-m7.md", Colunas: 70, Linhas: 20}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer celula.Matar()

	if tela := telaLimpaDe(celula); !strings.Contains(tela, "fichas clínicas") {
		t.Errorf("a aba devia abrir direto no arquivo pedido:\n%s", tela)
	}
}

// TestMdRecarregaQuandoODiscoMuda — o agente edita o arquivo, e o markdown ao
// lado se atualiza sozinho.
func TestMdRecarregaQuandoODiscoMuda(t *testing.T) {
	dir := t.TempDir()
	arquivo := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(arquivo, []byte("# Antes\n\nprimeira versão\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	celula, _ := Nova("md")
	avisos := make(chan struct{}, 32)
	if err := celula.Nascer(Config{
		ID: "c1", Diretorio: dir, Alvo: arquivo, Colunas: 60, Linhas: 20,
		Avisar: func() {
			select {
			case avisos <- struct{}{}:
			default:
			}
		},
	}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer celula.Matar()

	if !strings.Contains(telaLimpaDe(celula), "primeira versão") {
		t.Fatalf("a primeira versão não apareceu:\n%s", telaLimpaDe(celula))
	}

	if err := os.WriteFile(arquivo, []byte("# Depois\n\nsegunda versão\n"), 0o644); err != nil {
		t.Fatalf("editar: %v", err)
	}
	if !esperarPor(t, time.Second, func() bool {
		return strings.Contains(telaLimpaDe(celula), "segunda versão")
	}) {
		t.Fatalf("o arquivo mudou e a célula não acompanhou em 1s:\n%s", telaLimpaDe(celula))
	}
	select {
	case <-avisos:
	default:
		t.Error("a célula mudou sem avisar a tela")
	}
}

// TestMdComArquivoApagadoNaoEntraEmPanico — some do disco, vira erro legível.
func TestMdComArquivoApagadoNaoEntraEmPanico(t *testing.T) {
	dir := t.TempDir()
	arquivo := filepath.Join(dir, "some.md")
	if err := os.WriteFile(arquivo, []byte("# Existe\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	celula, _ := Nova("md")
	if err := celula.Nascer(Config{ID: "c1", Diretorio: dir, Alvo: arquivo, Colunas: 60, Linhas: 20}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer celula.Matar()

	if err := os.Remove(arquivo); err != nil {
		t.Fatalf("apagar: %v", err)
	}
	if !esperarPor(t, 2*time.Second, func() bool { return celula.Estado() == Caiu }) {
		t.Fatalf("arquivo apagado tinha que virar estado de erro, está %q", celula.Estado())
	}
	if tela := telaLimpaDe(celula); !strings.Contains(tela, "sumiu do disco") {
		t.Fatalf("o erro precisa ser legível, veio:\n%s", tela)
	}
}

// TestMdNasceSemAlvo — a aba não precisa de arquivo para existir.
func TestMdNasceSemAlvo(t *testing.T) {
	celula, _ := Nova("md")
	if err := celula.Nascer(Config{ID: "c1", Diretorio: t.TempDir(), Colunas: 60, Linhas: 20}); err != nil {
		t.Fatalf("a aba de markdown nasce sem alvo: %v", err)
	}
	defer celula.Matar()
	if tela := telaLimpaDe(celula); !strings.Contains(tela, "0 de 0 documentos") {
		t.Errorf("projeto sem markdown mostra a lista vazia:\n%s", tela)
	}
}

// TestMdRolaPeloTexto — arquivo grande rola e volta ao começo.
func TestMdRolaPeloTexto(t *testing.T) {
	dir := t.TempDir()
	arquivo := filepath.Join(dir, "grande.md")
	var texto strings.Builder
	texto.WriteString("# Grande\n\n")
	for i := range 200 {
		texto.WriteString("linha de número " + string(rune('a'+i%26)) + "\n\n")
	}
	if err := os.WriteFile(arquivo, []byte(texto.String()), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	celula, _ := Nova("md")
	if err := celula.Nascer(Config{ID: "c1", Diretorio: dir, Alvo: arquivo, Colunas: 60, Linhas: 10}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer celula.Matar()

	topo := telaLimpaDe(celula)
	celula.Rolar(-20, false)
	rolado := celula.Desenhar()
	if rolado.AoVivo {
		t.Fatal("depois de rolar, a leitura não está no começo")
	}
	if strings.Join(rolado.Linhas, "\n") == topo {
		t.Fatal("rolar não mudou o que está na tela")
	}
	celula.Rolar(0, true)
	if telaLimpaDe(celula) != topo {
		t.Fatal("voltar ao vivo tinha que trazer o começo do arquivo de volta")
	}
}

// TestPaginaTemMargemEMedidaDeLeitura — o markdown é desenhado como página, não
// como saída de terminal: texto centralizado, medida de leitura e margem.
func TestPaginaTemMargemEMedidaDeLeitura(t *testing.T) {
	texto := "# Título\n\n" + strings.Repeat("palavra ", 200) + "\n"
	linhas := renderizarPagina(texto, 160)

	maior := 0
	comRecuo := 0
	for _, linha := range linhas {
		limpa := historico.LimparCodigos(linha)
		if comprimento := len([]rune(strings.TrimRight(limpa, " "))); comprimento > maior {
			maior = comprimento
		}
		if strings.HasPrefix(limpa, "  ") && strings.TrimSpace(limpa) != "" {
			comRecuo++
		}
	}
	if maior > 160 {
		t.Fatalf("a página passou da largura da célula: %d colunas", maior)
	}
	if maior > medidaDeLeitura+40 {
		t.Fatalf("o texto devia respeitar a medida de leitura, veio com %d colunas", maior)
	}
	if comRecuo == 0 {
		t.Fatal("a página devia ter margem à esquerda")
	}
}

// TestPaginaNaoQuebraCodigoLargo — diagrama e tela de terminal são cortados, não
// embaralhados em várias linhas.
func TestPaginaNaoQuebraCodigoLargo(t *testing.T) {
	larga := strings.Repeat("─", 200)
	texto := "# Doc\n\n```\n" + larga + "\n```\n"
	linhas := renderizarPagina(texto, 80)

	for _, linha := range linhas {
		limpa := strings.TrimRight(historico.LimparCodigos(linha), " ")
		if len([]rune(limpa)) > 80 {
			t.Fatalf("linha maior que a célula: %d colunas", len([]rune(limpa)))
		}
	}
	juntas := strings.Join(linhas, "\n")
	if !strings.Contains(juntas, "›") {
		t.Fatalf("o corte do código devia estar marcado:\n%s", juntas)
	}
	comCodigo := 0
	for _, linha := range linhas {
		if strings.Contains(historico.LimparCodigos(linha), "──") {
			comCodigo++
		}
	}
	if comCodigo > 1 {
		t.Fatalf("a linha de código foi quebrada em %d linhas em vez de cortada:\n%s", comCodigo, juntas)
	}
}

// TestPaginaDesenhaTituloComoCapitulo — o H1 vira uma faixa, não um "#".
func TestPaginaDesenhaTituloComoCapitulo(t *testing.T) {
	linhas := renderizarPagina("# Módulo 7\n\ntexto\n", 100)
	juntas := strings.Join(linhas, "\n")
	if strings.Contains(historico.LimparCodigos(juntas), "# Módulo") {
		t.Fatalf("o sustenido não pode sobrar na página:\n%s", juntas)
	}
	if !strings.Contains(historico.LimparCodigos(juntas), "MÓDULO 7") {
		t.Fatalf("o título devia virar faixa em caixa alta:\n%s", juntas)
	}
	// "48;" cobre o fundo em 256 cores e em 24 bits: o que importa é que a
	// faixa tenha fundo, não em que profundidade o terminal a escreve.
	if !strings.Contains(juntas, "48;") {
		t.Fatalf("a faixa do título devia ter fundo próprio:\n%q", juntas)
	}
}
