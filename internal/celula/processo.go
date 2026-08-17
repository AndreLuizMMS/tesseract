package celula

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// caudaReproduzida é quanto do histórico é reencenado na tela interna quando a
// célula renasce. O suficiente para a última tela reaparecer inteira, pouco o
// bastante para não custar nada.
const caudaReproduzida = 256 << 10

// Processo é a base das células que rodam alguma coisa: mantém o pseudo
// terminal, a tela interna e a gravação do histórico. Os tipos concretos só
// dizem qual comando sobe.
type Processo struct {
	mu        sync.Mutex
	pincel    sync.Mutex // protege a tela interna entre quem escreve e quem desenha
	emulador  *vt.SafeEmulator
	terminal  *os.File
	comando   *exec.Cmd
	config    Config
	estado    Estado
	rolagem   int
	morrendo  bool // verdadeiro quando fomos nós que matamos
	encerrado bool
}

// iniciar sobe o comando dentro de um pseudo terminal do tamanho pedido e
// reencena o histórico anterior na tela interna.
func (p *Processo) iniciar(cfg Config, programa string, args []string, ambiente []string) error {
	if cfg.Colunas <= 0 {
		cfg.Colunas = 80
	}
	if cfg.Linhas <= 0 {
		cfg.Linhas = 24
	}
	if info, err := os.Stat(cfg.Diretorio); err != nil || !info.IsDir() {
		return fmt.Errorf("diretório do projeto não existe: %s", cfg.Diretorio)
	}

	p.config = cfg
	p.emulador = vt.NewSafeEmulator(cfg.Colunas, cfg.Linhas)
	p.estado = Trabalhando

	// A tela interna responde sozinha às perguntas que o programa faz ao
	// terminal. Quem escuta essas respostas precisa estar de pé antes de
	// qualquer byte entrar — inclusive antes da reencenação do histórico, que
	// traz perguntas velhas junto e travaria aqui se ninguém as recolhesse.
	go p.responderAoProcesso()

	if cfg.Historico != nil {
		if cauda, err := cfg.Historico.Cauda(caudaReproduzida); err == nil && len(cauda) > 0 {
			// Só a tela interna recebe: reproduzir não é produzir, e nada
			// disso volta para o arquivo nem chega ao processo. As respostas
			// que a reencenação provoca respondem a perguntas velhas e morrem
			// aqui, porque ainda não há processo para recebê-las.
			p.pincel.Lock()
			_, _ = p.emulador.Write(cauda)
			p.pincel.Unlock()
		}
	}

	return p.religar(programa, args, ambiente)
}

// religar sobe o comando na mesma tela interna, que é o que permite uma célula
// de log voltar a acompanhar o serviço quando ele sobe de novo, sem perder o
// que já estava escrito.
func (p *Processo) religar(programa string, args, ambiente []string) error {
	p.mu.Lock()
	colunas, linhas := p.config.Colunas, p.config.Linhas
	p.mu.Unlock()

	comando := exec.Command(programa, args...)
	comando.Dir = p.config.Diretorio
	comando.Env = ambiente

	terminal, err := pty.StartWithSize(comando, &pty.Winsize{
		Cols: uint16(colunas),
		Rows: uint16(linhas),
	})
	if err != nil {
		return fmt.Errorf("subindo %s em %s: %w", programa, p.config.Diretorio, err)
	}

	p.mu.Lock()
	p.comando, p.terminal = comando, terminal
	p.encerrado, p.morrendo = false, false
	p.estado = Trabalhando
	p.mu.Unlock()

	go p.lerDoProcesso(terminal, comando)
	return nil
}

// lerDoProcesso leva o que o processo escreveu para a tela interna e para o
// histórico, e percebe quando ele morre.
func (p *Processo) lerDoProcesso(terminal *os.File, comando *exec.Cmd) {
	buf := make([]byte, 32<<10)
	for {
		n, err := terminal.Read(buf)
		if n > 0 {
			p.pincel.Lock()
			_, _ = p.emulador.Write(buf[:n])
			p.pincel.Unlock()
			if p.config.Historico != nil {
				_, _ = p.config.Historico.Write(buf[:n])
			}
			p.avisar()
		}
		if err != nil {
			break
		}
	}

	_ = comando.Wait()

	p.mu.Lock()
	if p.morrendo {
		p.estado = Parada
	} else {
		p.estado = Caiu
	}
	p.encerrado = true
	p.mu.Unlock()
	p.avisar()
}

// responderAoProcesso devolve ao programa as respostas que a tela interna
// produz — consultas de posição de cursor, de capacidade do terminal e afins.
// É um só, vivo pela vida inteira da célula: o processo pode cair e voltar, a
// tela interna continua a mesma.
func (p *Processo) responderAoProcesso() {
	buf := make([]byte, 4<<10)
	for {
		n, err := p.emulador.Read(buf)
		if n > 0 {
			p.mu.Lock()
			terminal := p.terminal
			p.mu.Unlock()
			if terminal != nil {
				_, _ = terminal.Write(buf[:n])
			}
		}
		if err != nil {
			return
		}
	}
}

func (p *Processo) avisar() {
	if p.config.Avisar != nil {
		p.config.Avisar()
	}
}

// Desenhar monta o retrato da célula: a tela interna quando a leitura está ao
// vivo, ou a janela do histórico quando o usuário rolou para trás. Nos dois
// casos o texto vai estilizado — rolar não pode apagar a cor de nada.
func (p *Processo) Desenhar() Quadro {
	p.mu.Lock()
	rolagem := p.rolagem
	p.mu.Unlock()

	if p.emulador == nil {
		return Quadro{AoVivo: true}
	}

	p.pincel.Lock()
	defer p.pincel.Unlock()

	altura := p.emulador.Height()
	acima := p.emulador.ScrollbackLen()
	rolagem = min(rolagem, acima)
	naTela := strings.Split(p.emulador.Render(), "\n")

	if rolagem == 0 {
		posicao := p.emulador.CursorPosition()
		return Quadro{Linhas: naTela, CursorX: posicao.X, CursorY: posicao.Y, AoVivo: true}
	}

	// A janela rolada mistura as linhas que já saíram por cima com as que
	// ainda estão na tela por baixo.
	historico := p.emulador.Scrollback()
	linhas := make([]string, 0, altura)
	topo := acima - rolagem
	for i := range altura {
		linha := topo + i
		switch {
		case linha < acima:
			linhas = append(linhas, historico.Line(linha).Render())
		case linha-acima < len(naTela):
			linhas = append(linhas, naTela[linha-acima])
		default:
			linhas = append(linhas, "")
		}
	}
	return Quadro{Linhas: linhas, Rolagem: rolagem, CursorX: -1, CursorY: -1}
}

// Tecla entrega a tecla ao processo. Em modo DIGITAR toda tecla passa por
// aqui, sem exceção. A tela interna traduz a tecla nos bytes que aquele
// programa espera, respeitando os modos que ele ligou.
func (p *Processo) Tecla(toque Toque) error {
	p.mu.Lock()
	encerrado := p.encerrado
	p.rolagem = 0 // digitar traz a leitura de volta para o vivo
	p.mu.Unlock()

	if encerrado || p.emulador == nil {
		return fmt.Errorf("a célula não está rodando")
	}
	if toque.Colar != "" {
		// Colar vai como colagem, não como digitação: quando o programa lá
		// dentro pediu bracketed paste, o texto chega marcado como colado e as
		// quebras de linha não viram enter — um prompt de várias linhas entra
		// inteiro em vez de ser enviado linha a linha.
		p.emulador.Paste(toque.Colar)
		return nil
	}
	// Tecla que imprime alguma coisa vai como texto: é o que ela escreve que
	// interessa, e assim maiúscula com shift não se perde no caminho.
	if toque.Texto != "" && toque.Mod&^int(vt.ModShift) == 0 {
		p.emulador.SendText(toque.Texto)
		return nil
	}
	p.emulador.SendKey(vt.KeyPressEvent{
		Code: toque.Codigo,
		Text: toque.Texto,
		Mod:  vt.KeyMod(toque.Mod),
	})
	return nil
}

func (p *Processo) Estado() Estado {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.estado
}

func (p *Processo) Estados() []Estado {
	return []Estado{Trabalhando, Caiu, Parada, Orfa}
}

// Redimensionar ajusta a tela interna e o pseudo terminal ao espaço que a tela
// reservou para a célula.
func (p *Processo) Redimensionar(colunas, linhas int) error {
	if colunas <= 0 || linhas <= 0 {
		return nil
	}
	p.mu.Lock()
	// Redimensionar de novo para o mesmo tamanho limpa a tela interna, e o
	// programa lá dentro só redesenha quando tem o que dizer — foi assim que
	// uma aba recém-aberta aparecia em branco.
	if p.config.Colunas == colunas && p.config.Linhas == linhas {
		p.mu.Unlock()
		return nil
	}
	terminal := p.terminal
	p.config.Colunas, p.config.Linhas = colunas, linhas
	p.mu.Unlock()

	if p.emulador != nil {
		p.pincel.Lock()
		p.emulador.Resize(colunas, linhas)
		p.pincel.Unlock()
	}
	if terminal != nil {
		_ = pty.Setsize(terminal, &pty.Winsize{Cols: uint16(colunas), Rows: uint16(linhas)})
	}
	p.avisar()
	return nil
}

// Rolar move a leitura pelo histórico. Delta positivo sobe.
func (p *Processo) Rolar(delta int, aoVivo bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if aoVivo {
		p.rolagem = 0
		return
	}
	teto := 0
	if p.emulador != nil {
		teto = p.emulador.ScrollbackLen()
	}
	p.rolagem = min(max(p.rolagem+delta, 0), teto)
}

// Matar encerra o processo. O disco do projeto nunca é tocado.
func (p *Processo) Matar() error {
	p.mu.Lock()
	if p.encerrado || p.comando == nil || p.comando.Process == nil {
		p.mu.Unlock()
		return nil
	}
	p.morrendo = true
	pid := p.comando.Process.Pid
	terminal := p.terminal
	p.mu.Unlock()

	// O pseudo terminal cria um grupo próprio; matar o grupo leva junto o que
	// o shell tiver aberto.
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	if terminal != nil {
		_ = terminal.Close()
	}
	return nil
}
