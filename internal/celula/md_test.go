package celula

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestMdRecarregaQuandoODiscoMuda — o agente edita o arquivo, e o markdown ao
// lado se atualiza sozinho.
func TestMdRecarregaQuandoODiscoMuda(t *testing.T) {
	dir := t.TempDir()
	arquivo := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(arquivo, []byte("# Antes\n\nprimeira versão\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	celula, err := Nova("md")
	if err != nil {
		t.Fatalf("fabricar: %v", err)
	}
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

	celula, err := Nova("md")
	if err != nil {
		t.Fatalf("fabricar: %v", err)
	}
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
	tela := telaLimpaDe(celula)
	if !strings.Contains(tela, "sumiu do disco") {
		t.Fatalf("o erro precisa ser legível, veio:\n%s", tela)
	}
}

// TestMdNaoNasceSemArquivo — pedir markdown sem dizer qual falha claro.
func TestMdNaoNasceSemArquivo(t *testing.T) {
	celula, _ := Nova("md")
	err := celula.Nascer(Config{ID: "c1", Diretorio: t.TempDir(), Colunas: 60, Linhas: 20})
	if err == nil {
		t.Fatal("markdown sem arquivo devia falhar")
	}
	if !strings.Contains(err.Error(), "arquivo") {
		t.Fatalf("mensagem pouco clara: %v", err)
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
