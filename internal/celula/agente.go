package celula

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/vt"

	"github.com/andreluiz/tesseract/internal/motor/historico"
)

// cadenciaDeObservacao é de quanto em quanto tempo a célula olha a própria tela
// para decidir em que estado de turno está.
const cadenciaDeObservacao = 500 * time.Millisecond

// esperaAntesDoEnter dá ao agente o tempo de digerir o texto colado antes de
// receber o enter que manda o turno começar.
const esperaAntesDoEnter = 120 * time.Millisecond

// ComPrompt é a célula que aceita trabalho sem o usuário entrar nela.
type ComPrompt interface {
	Prompt(texto string) error
}

// ComConversa é a célula que tem uma conversa com nome próprio: dá para
// empurrar o nome para dentro dela e para puxar o nome que ela escolheu.
type ComConversa interface {
	// PropagarNome manda o nome para dentro do agente.
	PropagarNome(nome string) error
	// NomeDaConversa é o nome que o próprio agente deu à conversa.
	NomeDaConversa() (string, error)
	// Conversa é a identidade da conversa, para reatar depois de uma queda.
	Conversa() string
}

// Agente é a base de claude e cursor: um processo interativo com estado de
// turno, conversa com nome e prompt sem entrar.
type Agente struct {
	Processo
	turno           *Turno
	comandoRenomear string
	conversa        string
	nascidoEm       time.Time
	parar           chan struct{}
	lerNome         func(diretorio, conversa string) (string, error)
	acharConversa   func(diretorio string, desde time.Time) string
}

// nascer sobe o agente e começa a acompanhar o turno dele.
func (a *Agente) nascer(cfg Config, programa string, args []string, marcadores Marcadores) error {
	// A configuração do usuário manda: ela existe para o dia em que o agente
	// trocar o texto que escreve na própria tela.
	if len(cfg.Marcadores.Trabalho) > 0 {
		marcadores.Trabalho = cfg.Marcadores.Trabalho
	}
	if len(cfg.Marcadores.Pergunta) > 0 {
		marcadores.Pergunta = cfg.Marcadores.Pergunta
	}
	a.turno = NovoTurno(marcadores)
	a.conversa = cfg.Conversa
	a.nascidoEm = time.Now()
	a.parar = make(chan struct{})
	if err := a.iniciar(cfg, programa, args, ambienteDeTerminal()); err != nil {
		return err
	}
	go a.acompanharTurno()
	return nil
}

// acompanharTurno lê a tela em intervalos regulares e atualiza o estado. É
// aqui que a heurística contra alarme falso roda.
func (a *Agente) acompanharTurno() {
	relogio := time.NewTicker(cadenciaDeObservacao)
	defer relogio.Stop()
	anterior := a.turno.Estado()
	for {
		select {
		case <-a.parar:
			return
		case <-relogio.C:
			if a.Processo.Estado() != Trabalhando {
				return // o processo morreu; quem manda no estado é ele
			}
			estado := a.turno.Observar(a.telaEmTexto())
			if estado != anterior {
				anterior = estado
				a.avisar()
			}
			if a.conversa == "" && a.acharConversa != nil {
				if achada := a.acharConversa(a.config.Diretorio, a.nascidoEm); achada != "" {
					a.conversa = achada
					if a.config.AoDescobrirConversa != nil {
						a.config.AoDescobrirConversa(achada)
					}
				}
			}
		}
	}
}

// telaEmTexto é a tela da célula sem os códigos de cor, que é o que os
// marcadores do agente são comparados contra.
func (a *Agente) telaEmTexto() string {
	linhas := a.Desenhar().Linhas
	limpas := make([]string, len(linhas))
	for i, linha := range linhas {
		limpas[i] = historico.LimparCodigos(linha)
	}
	return strings.Join(limpas, "\n")
}

// Estado é o do processo quando ele morreu, e o do turno enquanto ele vive.
func (a *Agente) Estado() Estado {
	if base := a.Processo.Estado(); base != Trabalhando {
		return base
	}
	return a.turno.Estado()
}

func (a *Agente) Estados() []Estado {
	return []Estado{Trabalhando, Respondeu, Aprovar, Caiu, Parada, Orfa}
}

// Tecla marca a célula como lida: quem entrou nela já viu o que tinha para ver.
func (a *Agente) Tecla(toque Toque) error {
	a.turno.Visto()
	return a.Processo.Tecla(toque)
}

// Prompt manda trabalho para o agente sem o usuário entrar na célula.
func (a *Agente) Prompt(texto string) error {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return fmt.Errorf("o prompt está vazio")
	}
	if err := a.Processo.Tecla(Toque{Colar: texto}); err != nil {
		return err
	}
	time.Sleep(esperaAntesDoEnter)
	return a.Processo.Tecla(Toque{Codigo: vt.KeyEnter})
}

// PropagarNome empurra o nome escolhido pelo usuário para dentro do agente.
func (a *Agente) PropagarNome(nome string) error {
	if a.comandoRenomear == "" {
		return fmt.Errorf("este agente não sabe renomear a própria conversa")
	}
	return a.Prompt(a.comandoRenomear + " " + nome)
}

// NomeDaConversa é o nome que o próprio agente deu à conversa. Conversa sem
// nome avisa em vez de renomear a célula para vazio.
func (a *Agente) NomeDaConversa() (string, error) {
	if a.lerNome == nil {
		return "", fmt.Errorf("este agente não guarda nome de conversa")
	}
	nome, err := a.lerNome(a.config.Diretorio, a.conversa)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(nome) == "" {
		return "", fmt.Errorf("a conversa ainda não tem nome")
	}
	return nome, nil
}

// Conversa é a identidade da conversa, que o motor guarda para reatar depois.
func (a *Agente) Conversa() string { return a.conversa }

// Matar encerra o acompanhamento junto com o processo.
func (a *Agente) Matar() error {
	select {
	case <-a.parar:
	default:
		close(a.parar)
	}
	return a.Processo.Matar()
}

// novaIdentidadeDeConversa gera o identificador que o agente recebe para poder
// ser reatado depois de uma queda.
func novaIdentidadeDeConversa() string {
	bruto := make([]byte, 16)
	if _, err := rand.Read(bruto); err != nil {
		return ""
	}
	// Formato de UUID versão 4, que é o que os agentes esperam.
	bruto[6] = (bruto[6] & 0x0f) | 0x40
	bruto[8] = (bruto[8] & 0x3f) | 0x80
	texto := hex.EncodeToString(bruto)
	return texto[0:8] + "-" + texto[8:12] + "-" + texto[12:16] + "-" + texto[16:20] + "-" + texto[20:]
}
