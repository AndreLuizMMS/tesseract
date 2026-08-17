package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

var (
	compilarUmaVez sync.Once
	binario        string
	erroDeCompilar error
)

// comando compila o `ts` uma vez e devolve um comando pronto, apontando para um
// estado de mentira — nada do teste encosta no Tesseract de verdade.
func comando(t *testing.T, casa string, argumentos ...string) *exec.Cmd {
	t.Helper()
	compilarUmaVez.Do(func() {
		destino, err := os.MkdirTemp("/tmp", "ts-bin")
		if err != nil {
			erroDeCompilar = err
			return
		}
		binario = filepath.Join(destino, "ts")
		saida, err := exec.Command("go", "build", "-o", binario, ".").CombinedOutput()
		if err != nil {
			erroDeCompilar = err
			t.Logf("compilar: %s", saida)
		}
	})
	if erroDeCompilar != nil {
		t.Fatalf("compilar: %v", erroDeCompilar)
	}

	cmd := exec.Command(binario, argumentos...)
	cmd.Env = append(os.Environ(),
		"XDG_STATE_HOME="+filepath.Join(casa, "estado"),
		"XDG_RUNTIME_DIR="+filepath.Join(casa, "run"),
		"XDG_CONFIG_HOME="+filepath.Join(casa, "config"),
		// Sem systemd no teste: o motor sobe como processo solto.
		"PATH="+semSystemctl(),
	)
	return cmd
}

// semSystemctl tira o systemctl do caminho para o teste não mexer no serviço de
// verdade da máquina.
func semSystemctl() string {
	var mantidos []string
	for _, pedaco := range filepath.SplitList(os.Getenv("PATH")) {
		if _, err := os.Stat(filepath.Join(pedaco, "systemctl")); err == nil {
			continue
		}
		mantidos = append(mantidos, pedaco)
	}
	return strings.Join(mantidos, string(filepath.ListSeparator))
}

// casaDeTeste é um lar curto: o socket unix não aceita caminho comprido.
func casaDeTeste(t *testing.T) string {
	t.Helper()
	casa, err := os.MkdirTemp("/tmp", "ts")
	if err != nil {
		t.Fatalf("preparar casa: %v", err)
	}
	t.Cleanup(func() {
		parar := comando(t, casa, "stop")
		_ = parar.Run()
		os.RemoveAll(casa)
	})
	return casa
}

func rodar(t *testing.T, casa string, argumentos ...string) (string, int) {
	t.Helper()
	cmd := comando(t, casa, argumentos...)
	saida, err := cmd.CombinedOutput()
	codigo := 0
	var comErro *exec.ExitError
	if err != nil {
		if ok := comoExitError(err, &comErro); ok {
			codigo = comErro.ExitCode()
		} else {
			t.Fatalf("rodar %v: %v", argumentos, err)
		}
	}
	return string(saida), codigo
}

func comoExitError(err error, destino **exec.ExitError) bool {
	comErro, ok := err.(*exec.ExitError)
	if ok {
		*destino = comErro
	}
	return ok
}

// TestStatusSemMotor — sem motor de pé, o status diz isso e sai bem.
func TestStatusSemMotor(t *testing.T) {
	casa := casaDeTeste(t)
	saida, codigo := rodar(t, casa, "status")
	if codigo != 0 {
		t.Fatalf("status saiu com %d: %s", codigo, saida)
	}
	if !strings.Contains(saida, "não está rodando") {
		t.Fatalf("status devia dizer que o motor não está rodando: %q", saida)
	}
}

// TestNovoSobeOMotorECriaOProjeto — `ts novo` funciona sem abrir a tela, e sobe
// o motor se ele não estiver de pé.
func TestNovoSobeOMotorECriaOProjeto(t *testing.T) {
	casa := casaDeTeste(t)
	projeto := filepath.Join(casa, "meu-projeto")
	if err := os.MkdirAll(projeto, 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	saida, codigo := rodar(t, casa, "novo", projeto)
	if codigo != 0 {
		t.Fatalf("novo saiu com %d: %s", codigo, saida)
	}
	if !strings.Contains(saida, "meu-projeto") {
		t.Fatalf("novo devia confirmar o projeto: %q", saida)
	}

	saida, codigo = rodar(t, casa, "status")
	if codigo != 0 {
		t.Fatalf("status saiu com %d: %s", codigo, saida)
	}
	if !strings.Contains(saida, "1 projeto") || !strings.Contains(saida, "meu-projeto") {
		t.Fatalf("status devia mostrar o projeto novo: %q", saida)
	}
}

// TestNovoEmCaminhoInexistenteFalha — erro claro e código de saída de erro.
func TestNovoEmCaminhoInexistenteFalha(t *testing.T) {
	casa := casaDeTeste(t)
	saida, codigo := rodar(t, casa, "novo", filepath.Join(casa, "não-existe"))
	if codigo != 1 {
		t.Fatalf("esperava código 1, veio %d: %s", codigo, saida)
	}
	if !strings.Contains(saida, "não existe") {
		t.Fatalf("mensagem pouco clara: %q", saida)
	}
}

// TestStopDesligaOMotor.
func TestStopDesligaOMotor(t *testing.T) {
	casa := casaDeTeste(t)
	projeto := filepath.Join(casa, "projeto")
	if err := os.MkdirAll(projeto, 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if _, codigo := rodar(t, casa, "novo", projeto); codigo != 0 {
		t.Fatal("não consegui criar o projeto")
	}

	saida, codigo := rodar(t, casa, "stop")
	if codigo != 0 {
		t.Fatalf("stop saiu com %d: %s", codigo, saida)
	}
	esperarAte(t, 5*time.Second, func() bool {
		texto, _ := rodar(t, casa, "status")
		return strings.Contains(texto, "não está rodando")
	})
}

// TestResetApagaOEstadoEPreservaAConfiguracao.
func TestResetApagaOEstadoEPreservaAConfiguracao(t *testing.T) {
	casa := casaDeTeste(t)
	projeto := filepath.Join(casa, "projeto")
	if err := os.MkdirAll(projeto, 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	config := filepath.Join(casa, "config", "tesseract", "config.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if err := os.WriteFile(config, []byte(`{"editor":"vim"}`), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	if _, codigo := rodar(t, casa, "novo", projeto); codigo != 0 {
		t.Fatal("não consegui criar o projeto")
	}
	arquivoDeEstado := filepath.Join(casa, "estado", "tesseract", "estado.json")
	if _, err := os.Stat(arquivoDeEstado); err != nil {
		t.Fatalf("o estado devia ter sido salvo: %v", err)
	}

	saida, codigo := rodar(t, casa, "reset")
	if codigo != 0 {
		t.Fatalf("reset saiu com %d: %s", codigo, saida)
	}
	if _, err := os.Stat(arquivoDeEstado); !os.IsNotExist(err) {
		t.Fatalf("o estado devia ter sumido: %v", err)
	}
	if _, err := os.Stat(config); err != nil {
		t.Fatalf("a configuração tinha que ser preservada: %v", err)
	}
	if _, err := os.Stat(filepath.Join(casa, "estado", "tesseract", "historico")); !os.IsNotExist(err) {
		t.Fatal("os históricos deviam ter sumido junto com o estado")
	}
}

// TestComandoDesconhecidoSaiComDois.
func TestComandoDesconhecidoSaiComDois(t *testing.T) {
	casa := casaDeTeste(t)
	saida, codigo := rodar(t, casa, "voar")
	if codigo != 2 {
		t.Fatalf("esperava código 2, veio %d: %s", codigo, saida)
	}
	if !strings.Contains(saida, "ts novo") {
		t.Fatalf("o uso devia ser mostrado: %q", saida)
	}
}

// TestAjudaListaOsCincoComandos.
func TestAjudaListaOsCincoComandos(t *testing.T) {
	casa := casaDeTeste(t)
	saida, codigo := rodar(t, casa, "--help")
	if codigo != 0 {
		t.Fatalf("ajuda saiu com %d", codigo)
	}
	for _, comando := range []string{"ts novo", "ts status", "ts stop", "ts reset"} {
		if !strings.Contains(saida, comando) {
			t.Errorf("a ajuda não fala de %q", comando)
		}
	}
}

// TestTelaAbreDesenhaEFecha é a prova de que a tela funciona de ponta a ponta:
// o `ts` sobe o motor, desenha o mosaico com uma célula, aceita tecla e fecha
// deixando o motor vivo.
func TestTelaAbreDesenhaEFecha(t *testing.T) {
	casa := casaDeTeste(t)
	projeto := filepath.Join(casa, "projeto")
	if err := os.MkdirAll(projeto, 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	cmd := comando(t, casa)
	cmd.Dir = projeto
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	tela := vt.NewSafeEmulator(100, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 100, Rows: 30})
	if err != nil {
		t.Fatalf("abrir a tela: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = copiar(tela, terminal) }()
	go func() { _, _ = copiar(terminal, tela) }()

	// A tela abre com uma célula bash em tela cheia.
	esperarAte(t, 10*time.Second, func() bool {
		return strings.Contains(tela.Render(), "TESSERACT") && strings.Contains(tela.Render(), "bash")
	})
	desenho := tela.Render()
	if !strings.Contains(desenho, "NAVEGAR") {
		t.Fatalf("a tela devia abrir em NAVEGAR:\n%s", desenho)
	}
	if !strings.Contains(desenho, "n criar") {
		t.Fatalf("o rodapé devia estar aceso:\n%s", desenho)
	}

	// A ajuda abre e fecha.
	_, _ = terminal.Write([]byte("?"))
	esperarAte(t, 3*time.Second, func() bool { return strings.Contains(tela.Render(), "AJUDA") })
	_, _ = terminal.Write([]byte{0x1b})
	esperarAte(t, 3*time.Second, func() bool { return !strings.Contains(tela.Render(), "AJUDA") })

	// O formulário de criação abre com o projeto focado preenchido.
	_, _ = terminal.Write([]byte("n"))
	esperarAte(t, 3*time.Second, func() bool { return strings.Contains(tela.Render(), "NOVA") })
	if !strings.Contains(tela.Render(), "PROJETO") {
		t.Fatalf("o formulário devia perguntar o projeto:\n%s", tela.Render())
	}
	_, _ = terminal.Write([]byte{0x1b})
	esperarAte(t, 3*time.Second, func() bool { return !strings.Contains(tela.Render(), "NOVA") })

	// Entra em DIGITAR, escreve no shell e volta.
	_, _ = terminal.Write([]byte("\r"))
	esperarAte(t, 3*time.Second, func() bool { return strings.Contains(tela.Render(), "DIGITAR") })
	_, _ = terminal.Write([]byte("echo tesseract-vivo\r"))
	esperarAte(t, 5*time.Second, func() bool { return strings.Contains(tela.Render(), "tesseract-vivo") })
	_, _ = terminal.Write([]byte{0x0c}) // ctrl-l
	esperarAte(t, 3*time.Second, func() bool { return strings.Contains(tela.Render(), "NAVEGAR") })

	// Fecha a tela — e o motor continua rodando.
	_, _ = terminal.Write([]byte("q"))
	if err := esperarSaida(cmd, 5*time.Second); err != nil {
		t.Fatalf("a tela não fechou: %v", err)
	}
	saida, codigo := rodar(t, casa, "status")
	if codigo != 0 || !strings.Contains(saida, "1 célula") {
		t.Fatalf("o motor devia continuar vivo com a célula: %q", saida)
	}
}

func esperarSaida(cmd *exec.Cmd, prazo time.Duration) error {
	pronto := make(chan error, 1)
	go func() { pronto <- cmd.Wait() }()
	select {
	case err := <-pronto:
		return err
	case <-time.After(prazo):
		_ = cmd.Process.Kill()
		return os.ErrDeadlineExceeded
	}
}

func esperarAte(t *testing.T, prazo time.Duration, condicao func() bool) {
	t.Helper()
	limite := time.Now().Add(prazo)
	for time.Now().Before(limite) {
		if condicao() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !condicao() {
		t.Fatalf("a condição não aconteceu em %s", prazo)
	}
}

// copiar liga as duas pontas do pseudo terminal.
func copiar(destino interface{ Write([]byte) (int, error) }, origem interface {
	Read([]byte) (int, error)
},
) (int64, error) {
	buf := make([]byte, 32<<10)
	var total int64
	for {
		n, err := origem.Read(buf)
		if n > 0 {
			escrito, erroEscrita := destino.Write(buf[:n])
			total += int64(escrito)
			if erroEscrita != nil {
				return total, erroEscrita
			}
		}
		if err != nil {
			return total, err
		}
	}
}

// TestMosaicoComDoisProjetos é a prova visual da fase do mosaico: dois projetos
// viram duas colunas, a focada larga e a outra em tira, e a seta troca qual
// engorda. Matar a última célula tira o projeto da tela.
func TestMosaicoComDoisProjetos(t *testing.T) {
	casa := casaDeTeste(t)
	primeiro := filepath.Join(casa, "cortz-web")
	segundo := filepath.Join(casa, "doxar-api")
	for _, dir := range []string{primeiro, segundo} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("preparar: %v", err)
		}
		if saida, codigo := rodar(t, casa, "novo", dir); codigo != 0 {
			t.Fatalf("criar projeto: %s", saida)
		}
	}

	cmd := comando(t, casa)
	cmd.Dir = primeiro
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	tela := vt.NewSafeEmulator(120, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("abrir a tela: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = copiar(tela, terminal) }()
	go func() { _, _ = copiar(terminal, tela) }()

	esperarAte(t, 10*time.Second, func() bool { return strings.Contains(tela.Render(), "CORTZ-WEB") })
	if !strings.Contains(tela.Render(), "┬") {
		t.Fatalf("dois projetos deviam virar duas colunas:\n%s", tela.Render())
	}

	// A coluna do lado engorda quando o foco anda.
	_, _ = terminal.Write([]byte("\x1b[C")) // seta para a direita
	esperarAte(t, 5*time.Second, func() bool { return strings.Contains(tela.Render(), "DOXAR-API") })

	// D pede confirmação, e avisa que o projeto sai da tela.
	_, _ = terminal.Write([]byte("D"))
	esperarAte(t, 5*time.Second, func() bool { return strings.Contains(tela.Render(), "MATAR") })
	if !strings.Contains(tela.Render(), "sai da tela") {
		t.Fatalf("a confirmação devia avisar que o projeto sai da tela:\n%s", tela.Render())
	}

	_, _ = terminal.Write([]byte("\r")) // confirma
	esperarAte(t, 10*time.Second, func() bool { return !strings.Contains(tela.Render(), "DOXAR-API") })

	// O diretório continua intacto no disco.
	if _, err := os.Stat(segundo); err != nil {
		t.Fatalf("o disco foi tocado: %v", err)
	}

	_, _ = terminal.Write([]byte("q"))
	_ = esperarSaida(cmd, 5*time.Second)
}

// TestListaMostraOMesmoQueOMosaico — a tecla de trocar de tela leva ao índice,
// com os mesmos projetos e células.
func TestListaMostraOMesmoQueOMosaico(t *testing.T) {
	casa := casaDeTeste(t)
	projeto := filepath.Join(casa, "cortz-web")
	if err := os.MkdirAll(projeto, 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if saida, codigo := rodar(t, casa, "novo", projeto); codigo != 0 {
		t.Fatalf("criar projeto: %s", saida)
	}

	cmd := comando(t, casa)
	cmd.Dir = projeto
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	tela := vt.NewSafeEmulator(120, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("abrir a tela: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = copiar(tela, terminal) }()
	go func() { _, _ = copiar(terminal, tela) }()

	esperarAte(t, 10*time.Second, func() bool { return strings.Contains(tela.Render(), "CORTZ-WEB") })

	_, _ = terminal.Write([]byte("v"))
	esperarAte(t, 5*time.Second, func() bool {
		desenho := tela.Render()
		return strings.Contains(desenho, "CORTZ-WEB") && strings.Contains(desenho, "bash")
	})

	_, _ = terminal.Write([]byte("q"))
	_ = esperarSaida(cmd, 5*time.Second)

	// A tela escolhida é lembrada entre execuções.
	segunda := comando(t, casa)
	segunda.Dir = projeto
	segunda.Env = append(segunda.Env, "TERM=xterm-256color")
	tela2 := vt.NewSafeEmulator(120, 30)
	terminal2, err := pty.StartWithSize(segunda, &pty.Winsize{Cols: 120, Rows: 30})
	if err != nil {
		t.Fatalf("reabrir a tela: %v", err)
	}
	defer terminal2.Close()
	go func() { _, _ = copiar(tela2, terminal2) }()
	go func() { _, _ = copiar(terminal2, tela2) }()

	esperarAte(t, 10*time.Second, func() bool { return strings.Contains(tela2.Render(), "CORTZ-WEB") })
	if !strings.Contains(tela2.Render(), "┬") && !strings.Contains(tela2.Render(), "│") {
		t.Fatalf("a tela devia ter reaberto:\n%s", tela2.Render())
	}
	_, _ = terminal2.Write([]byte("q"))
	_ = esperarSaida(segunda, 5*time.Second)
}

// TestBuscaNoHistoricoPelaTela — a tecla de busca pergunta o termo, o motor
// procura no histórico da célula focada e a tela mostra o que achou.
func TestBuscaNoHistoricoPelaTela(t *testing.T) {
	casa := casaDeTeste(t)
	projeto := filepath.Join(casa, "projeto")
	if err := os.MkdirAll(projeto, 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	cmd := comando(t, casa)
	cmd.Dir = projeto
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	tela := vt.NewSafeEmulator(110, 30)
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 110, Rows: 30})
	if err != nil {
		t.Fatalf("abrir a tela: %v", err)
	}
	defer terminal.Close()
	go func() { _, _ = copiar(tela, terminal) }()
	go func() { _, _ = copiar(terminal, tela) }()

	esperarAte(t, 10*time.Second, func() bool { return strings.Contains(tela.Render(), "bash") })

	// Escreve algo no shell para haver histórico.
	_, _ = terminal.Write([]byte("\r"))
	esperarAte(t, 3*time.Second, func() bool { return strings.Contains(tela.Render(), "DIGITAR") })
	_, _ = terminal.Write([]byte("echo agulha-procurada\r"))
	esperarAte(t, 5*time.Second, func() bool { return strings.Contains(tela.Render(), "agulha-procurada") })
	_, _ = terminal.Write([]byte{0x0c}) // ctrl-l
	esperarAte(t, 3*time.Second, func() bool { return strings.Contains(tela.Render(), "NAVEGAR") })

	// Busca.
	_, _ = terminal.Write([]byte("/"))
	esperarAte(t, 3*time.Second, func() bool { return strings.Contains(tela.Render(), "BUSCAR") })
	_, _ = terminal.Write([]byte("agulha-procurada\r"))
	esperarAte(t, 5*time.Second, func() bool { return strings.Contains(tela.Render(), "BUSCA · agulha-procurada") })
	if !strings.Contains(tela.Render(), "agulha-procurada") {
		t.Fatalf("a busca devia mostrar a linha achada:\n%s", tela.Render())
	}

	_, _ = terminal.Write([]byte{0x1b}) // esc fecha
	esperarAte(t, 3*time.Second, func() bool { return !strings.Contains(tela.Render(), "BUSCA ·") })

	_, _ = terminal.Write([]byte("q"))
	_ = esperarSaida(cmd, 5*time.Second)
}
