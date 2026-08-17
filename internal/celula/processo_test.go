package celula

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"

	"github.com/andreluiz/tesseract/internal/motor/historico"
)

// esperarPor tenta a condição até dar certo ou o prazo estourar.
func esperarPor(t *testing.T, prazo time.Duration, condicao func() bool) bool {
	t.Helper()
	limite := time.Now().Add(prazo)
	for time.Now().Before(limite) {
		if condicao() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return condicao()
}

func telaDe(c Celula) string {
	return strings.Join(c.Desenhar().Linhas, "\n")
}

// colarEEntrar faz o que o usuário faz: cola o comando e depois manda o enter.
// A colagem vai marcada como colagem, então a quebra de linha não vem junto do
// texto colado.
func colarEEntrar(t *testing.T, c Celula, comando string) {
	t.Helper()
	if err := c.Tecla(Toque{Colar: comando}); err != nil {
		t.Fatalf("colar: %v", err)
	}
	if err := c.Tecla(Toque{Codigo: vt.KeyEnter}); err != nil {
		t.Fatalf("enter: %v", err)
	}
}

// TestBashMostraOQueFoiDigitado é a fatia vertical: sobe um shell de verdade,
// escreve nele, e a tela interna do motor passa a conter a saída.
func TestBashMostraOQueFoiDigitado(t *testing.T) {
	dir := t.TempDir()
	hist, err := historico.Abrir(filepath.Join(dir, "hist.log"), historico.TetoPadrao)
	if err != nil {
		t.Fatalf("abrir histórico: %v", err)
	}
	defer hist.Fechar()

	c, err := Nova("bash")
	if err != nil {
		t.Fatalf("fabricar célula: %v", err)
	}
	if err := c.Nascer(Config{
		ID: "c1", Diretorio: dir, Nome: "testes",
		Historico: hist, Colunas: 60, Linhas: 12,
	}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer c.Matar()

	if c.Estado() != Trabalhando {
		t.Fatalf("célula viva devia estar trabalhando, está %q", c.Estado())
	}

	colarEEntrar(t, c, "echo tesseract")
	if !esperarPor(t, 2*time.Second, func() bool {
		return strings.Contains(telaDe(c), "tesseract")
	}) {
		t.Fatalf("a saída não apareceu na tela interna em 2s:\n%s", telaDe(c))
	}

	// O que a célula produziu também foi para o histórico em disco.
	if !esperarPor(t, 2*time.Second, func() bool {
		achados, _ := hist.Buscar("tesseract")
		return len(achados) > 0
	}) {
		t.Fatal("a saída não foi gravada no histórico")
	}
}

// TestCelulaMortaViraParada garante que matar não é o mesmo que cair.
func TestCelulaMortaViraParada(t *testing.T) {
	dir := t.TempDir()
	c, err := Nova("bash")
	if err != nil {
		t.Fatalf("fabricar célula: %v", err)
	}
	if err := c.Nascer(Config{ID: "c1", Diretorio: dir, Colunas: 40, Linhas: 10}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	if err := c.Matar(); err != nil {
		t.Fatalf("matar: %v", err)
	}
	if !esperarPor(t, 2*time.Second, func() bool { return c.Estado() == Parada }) {
		t.Fatalf("estado depois de matar: %q, esperado parada", c.Estado())
	}
}

// TestProcessoQueMorreSozinhoCai é a outra ponta: ninguém mandou, então caiu.
func TestProcessoQueMorreSozinhoCai(t *testing.T) {
	dir := t.TempDir()
	c, err := Nova("bash")
	if err != nil {
		t.Fatalf("fabricar célula: %v", err)
	}
	if err := c.Nascer(Config{ID: "c1", Diretorio: dir, Colunas: 40, Linhas: 10}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer c.Matar()

	colarEEntrar(t, c, "exit")
	if !esperarPor(t, 3*time.Second, func() bool { return c.Estado() == Caiu }) {
		t.Fatalf("estado depois do shell sair: %q, esperado caiu", c.Estado())
	}
}

// TestNascerEmDiretorioInexistenteFalha protege o motor de estado inválido.
func TestNascerEmDiretorioInexistenteFalha(t *testing.T) {
	c, err := Nova("bash")
	if err != nil {
		t.Fatalf("fabricar célula: %v", err)
	}
	err = c.Nascer(Config{ID: "c1", Diretorio: filepath.Join(t.TempDir(), "não-existe"), Colunas: 40, Linhas: 10})
	if err == nil {
		t.Fatal("nascer em diretório inexistente devia falhar")
	}
	if !strings.Contains(err.Error(), "não existe") {
		t.Fatalf("mensagem de erro pouco clara: %v", err)
	}
}

// TestTipoDesconhecido garante que o registro é a única porta de entrada.
func TestTipoDesconhecido(t *testing.T) {
	if _, err := Nova("planilha"); err == nil {
		t.Fatal("tipo fora do registro devia falhar")
	}
}

// TestTeclaPorTeclaChegaNoShell cobre o caminho do modo DIGITAR: tecla que
// imprime, tecla com shift e tecla especial.
func TestTeclaPorTeclaChegaNoShell(t *testing.T) {
	dir := t.TempDir()
	c, err := Nova("bash")
	if err != nil {
		t.Fatalf("fabricar célula: %v", err)
	}
	if err := c.Nascer(Config{ID: "c1", Diretorio: dir, Colunas: 60, Linhas: 12}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer c.Matar()
	time.Sleep(300 * time.Millisecond) // deixa o shell desenhar o prompt

	for _, letra := range "echo Tesseract" {
		toque := Toque{Codigo: letra, Texto: string(letra)}
		if letra >= 'A' && letra <= 'Z' {
			toque.Codigo = letra + 32
			toque.Mod = int(vt.ModShift)
		}
		if err := c.Tecla(toque); err != nil {
			t.Fatalf("tecla %q: %v", letra, err)
		}
	}
	if err := c.Tecla(Toque{Codigo: vt.KeyEnter}); err != nil {
		t.Fatalf("enter: %v", err)
	}

	if !esperarPor(t, 3*time.Second, func() bool {
		return strings.Count(telaDe(c), "Tesseract") >= 2 // o eco do comando e a saída
	}) {
		t.Fatalf("a maiúscula ou o enter se perderam no caminho:\n%s", telaDe(c))
	}
}

// TestRolarMostraOPassado cobre a roda do mouse: sobe pelo histórico e volta ao
// vivo.
func TestRolarMostraOPassado(t *testing.T) {
	dir := t.TempDir()
	c, err := Nova("bash")
	if err != nil {
		t.Fatalf("fabricar célula: %v", err)
	}
	if err := c.Nascer(Config{ID: "c1", Diretorio: dir, Colunas: 40, Linhas: 8}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer c.Matar()

	colarEEntrar(t, c, "for i in $(seq 1 40); do echo linha-$i; done")
	if !esperarPor(t, 3*time.Second, func() bool {
		return strings.Contains(telaDe(c), "linha-40")
	}) {
		t.Fatalf("a saída não terminou de sair:\n%s", telaDe(c))
	}
	if strings.Contains(telaDe(c), "linha-1\n") {
		t.Fatal("com 8 linhas de tela, o começo já devia ter subido")
	}

	c.Rolar(30, false)
	quadro := c.Desenhar()
	if quadro.AoVivo {
		t.Fatal("depois de rolar, a leitura não está mais ao vivo")
	}
	if quadro.Rolagem != 30 {
		t.Fatalf("rolagem veio %d, esperado 30", quadro.Rolagem)
	}
	if !strings.Contains(strings.Join(quadro.Linhas, "\n"), "linha-1") {
		t.Fatalf("o passado não apareceu ao rolar:\n%s", strings.Join(quadro.Linhas, "\n"))
	}

	c.Rolar(0, true)
	quadro = c.Desenhar()
	if !quadro.AoVivo || quadro.Rolagem != 0 {
		t.Fatalf("esc devia voltar ao vivo: %#v", quadro)
	}
	if !strings.Contains(strings.Join(quadro.Linhas, "\n"), "linha-40") {
		t.Fatal("de volta ao vivo, o fim tinha que estar na tela")
	}
}

// TestRolagemPreservaEstilo é a regra de leitura: rolar mostra o passado do
// jeito que ele apareceu, com cor e negrito, não em texto pelado.
func TestRolagemPreservaEstilo(t *testing.T) {
	dir := t.TempDir()
	c, err := Nova("bash")
	if err != nil {
		t.Fatalf("fabricar célula: %v", err)
	}
	if err := c.Nascer(Config{ID: "c1", Diretorio: dir, Colunas: 40, Linhas: 6}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer c.Matar()

	// Cada linha sai verde e em negrito, e depois sobe para o histórico.
	colarEEntrar(t, c, "for i in $(seq 1 30); do printf '\\033[1;32mverde-%s\\033[0m\\n' $i; done")
	if !esperarPor(t, 3*time.Second, func() bool {
		return strings.Contains(telaDe(c), "verde-30")
	}) {
		t.Fatalf("a saída não terminou de sair:\n%s", telaDe(c))
	}

	c.Rolar(25, false)
	rolado := strings.Join(c.Desenhar().Linhas, "\n")
	if !strings.Contains(rolado, "verde-2") {
		t.Fatalf("o passado não apareceu ao rolar:\n%q", rolado)
	}
	if !strings.Contains(rolado, "\x1b[") {
		t.Fatalf("a rolagem entregou texto sem estilo nenhum:\n%q", rolado)
	}
	if !strings.Contains(rolado, "32") {
		t.Fatalf("a cor verde se perdeu na rolagem:\n%q", rolado)
	}
}

// TestNascerComHistoricoQuePerguntaAoTerminal é a regressão de um travamento
// real: a reencenação do histórico traz perguntas que o programa antigo fez ao
// terminal, e a tela interna responde a elas. Sem alguém escutando essas
// respostas, a célula nascia travada e o motor nunca abria o socket.
func TestNascerComHistoricoQuePerguntaAoTerminal(t *testing.T) {
	dir := t.TempDir()
	hist, err := historico.Abrir(filepath.Join(dir, "hist.log"), historico.TetoPadrao)
	if err != nil {
		t.Fatalf("abrir histórico: %v", err)
	}
	defer hist.Fechar()

	// Perguntas de sobra: atributos do dispositivo, posição do cursor e modos.
	perguntas := strings.Repeat("\x1b[c\x1b[6n\x1b[>0c\x1b[?1049$p", 200)
	if _, err := hist.Write([]byte("saída antiga\n" + perguntas)); err != nil {
		t.Fatalf("escrever histórico: %v", err)
	}

	nasceu := make(chan error, 1)
	go func() {
		c, err := Nova("bash")
		if err != nil {
			nasceu <- err
			return
		}
		err = c.Nascer(Config{ID: "c1", Diretorio: dir, Historico: hist, Colunas: 60, Linhas: 12})
		if err == nil {
			t.Cleanup(func() { c.Matar() })
		}
		nasceu <- err
	}()

	select {
	case err := <-nasceu:
		if err != nil {
			t.Fatalf("nascer: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a célula travou ao reencenar um histórico com perguntas ao terminal")
	}
}

// TestRedimensionarParaOMesmoTamanhoNaoLimpaATela é a regressão de uma aba que
// aparecia em branco: a tela interna era zerada por um redimensionamento que
// não mudava nada, e o programa lá dentro não tinha motivo para redesenhar.
func TestRedimensionarParaOMesmoTamanhoNaoLimpaATela(t *testing.T) {
	dir := t.TempDir()
	c, err := Nova("bash")
	if err != nil {
		t.Fatalf("fabricar célula: %v", err)
	}
	if err := c.Nascer(Config{ID: "c1", Diretorio: dir, Colunas: 60, Linhas: 12}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer c.Matar()

	colarEEntrar(t, c, "echo marca-na-tela")
	if !esperarPor(t, 3*time.Second, func() bool {
		return strings.Contains(telaDe(c), "marca-na-tela")
	}) {
		t.Fatalf("a saída não apareceu:\n%s", telaDe(c))
	}

	if err := c.Redimensionar(60, 12); err != nil {
		t.Fatalf("redimensionar: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if !strings.Contains(telaDe(c), "marca-na-tela") {
		t.Fatalf("o mesmo tamanho não podia limpar a tela:\n%s", telaDe(c))
	}
}
