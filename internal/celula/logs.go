package celula

import (
	"fmt"
	"time"

	"github.com/andreluiz/tesseract/internal/docker"
)

func init() {
	Registrar(Descritor{
		Tipo:       "logs",
		Ordem:      40,
		RotuloAlvo: "SERVIÇO",
	}, func() Celula { return &Logs{} })
}

// esperaParaEngatar é de quanto em quanto tempo a célula tenta acompanhar de
// novo um serviço que não está de pé.
const esperaParaEngatar = 3 * time.Second

// Logs é o log ao vivo de um serviço do compose. Só leitura.
type Logs struct {
	Processo
	arquivo  string
	servico  string
	programa string
	args     []string
	parar    chan struct{}
}

func (l *Logs) Nascer(cfg Config) error {
	if cfg.Alvo == "" {
		return fmt.Errorf("a célula de log precisa saber qual serviço acompanhar")
	}
	l.arquivo = docker.Detectar(cfg.Diretorio)
	if l.arquivo == "" {
		return fmt.Errorf("o projeto não tem arquivo de compose na raiz")
	}
	l.servico = cfg.Alvo
	l.programa, l.args = docker.ComandoDeLog(l.arquivo, l.servico)
	l.parar = make(chan struct{})

	if err := l.iniciar(cfg, l.programa, l.args, ambienteDeTerminal()); err != nil {
		return err
	}
	go l.engatarQuandoSubir()
	return nil
}

// engatarQuandoSubir é o que faz a célula esperar o serviço: enquanto ele está
// parado o log morre sozinho, e a célula tenta de novo até ele voltar.
func (l *Logs) engatarQuandoSubir() {
	relogio := time.NewTicker(esperaParaEngatar)
	defer relogio.Stop()
	for {
		select {
		case <-l.parar:
			return
		case <-relogio.C:
			l.mu.Lock()
			caiu := l.encerrado && !l.morrendo
			l.mu.Unlock()
			if !caiu {
				continue
			}
			_ = l.religar(l.programa, l.args, ambienteDeTerminal())
		}
	}
}

// Estado: log de serviço parado é célula parada esperando, não célula caída —
// o serviço voltar é normal, e ela engata sozinha.
func (l *Logs) Estado() Estado {
	if estado := l.Processo.Estado(); estado == Caiu {
		return Parada
	} else {
		return estado
	}
}

func (l *Logs) Estados() []Estado {
	return []Estado{Trabalhando, Parada, Orfa}
}

// Tecla não faz nada: célula de log é só leitura.
func (l *Logs) Tecla(Toque) error { return nil }

func (l *Logs) Matar() error {
	select {
	case <-l.parar:
	default:
		close(l.parar)
	}
	return l.Processo.Matar()
}
