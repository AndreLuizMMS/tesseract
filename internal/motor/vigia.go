package motor

import (
	"os"
	"path/filepath"
	"time"

	"github.com/andreluiz/tesseract/internal/celula"
	"github.com/andreluiz/tesseract/internal/docker"
)

const (
	// cadenciaDoVigia é de quanto em quanto tempo o motor olha o estado das
	// células para decidir se tem alguém para avisar.
	cadenciaDoVigia = 500 * time.Millisecond
	// voltasParaOlharODisco espaça a conferência de que os diretórios dos
	// projetos ainda existem.
	voltasParaOlharODisco = 20
	// voltasParaOlharODocker espaça a leitura do estado da stack de cada
	// projeto, que é o que a tira mostra.
	voltasParaOlharODocker = 40
)

// Vigiar acompanha as células e avisa quando alguma passa a pedir atenção. É o
// motor que notifica, não a tela: o usuário é avisado mesmo com a tela fechada.
func (m *Motor) Vigiar() {
	relogio := time.NewTicker(cadenciaDoVigia)
	defer relogio.Stop()
	volta := 0
	for {
		select {
		case <-m.parar:
			return
		case <-relogio.C:
			volta++
			m.conferirEstados()
			if volta%voltasParaOlharODisco == 0 {
				m.conferirDiretorios()
			}
			if volta%voltasParaOlharODocker == 1 {
				m.conferirDocker()
			}
			m.marcarSujo()
		}
	}
}

// Encerrar o vigia junto com o motor.
func (m *Motor) pararVigia() {
	m.umaVez.Do(func() { close(m.parar) })
}

// conferirEstados anuncia as mudanças que valem aviso: quem respondeu, quem
// pediu aprovação e quem caiu.
func (m *Motor) conferirEstados() {
	type anuncio struct{ texto string }
	var anuncios []anuncio
	var paraAdotarNome []string

	m.mu.Lock()
	for _, p := range m.projetos {
		nomeProjeto := filepath.Base(p.caminho)
		for _, c := range p.celulas {
			if c.viva == nil {
				continue
			}
			estado := c.viva.Estado()
			if estado == c.ultimoEstado {
				continue
			}
			anterior := c.ultimoEstado
			c.ultimoEstado = estado
			switch estado {
			case celula.Respondeu:
				anuncios = append(anuncios, anuncio{c.nome + " · " + nomeProjeto + " respondeu"})
				if c.aguardandoNome {
					paraAdotarNome = append(paraAdotarNome, c.id)
				}
			case celula.Aprovar:
				anuncios = append(anuncios, anuncio{c.nome + " · " + nomeProjeto + " pediu aprovação"})
			case celula.Caiu:
				if anterior != celula.Parada {
					anuncios = append(anuncios, anuncio{c.nome + " · " + nomeProjeto + " caiu"})
				}
			}
		}
	}
	m.mu.Unlock()

	for _, a := range anuncios {
		m.anunciar(a.texto)
	}
	// Fora do lock: adotar o nome chama Renomear, que trava o mutex de novo.
	for _, id := range paraAdotarNome {
		_ = m.AdotarNomeDoAgente(id)
	}
}

// conferirDiretorios marca como órfã a célula cujo projeto sumiu do disco.
func (m *Motor) conferirDiretorios() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.projetos {
		if info, err := os.Stat(p.caminho); err == nil && info.IsDir() {
			continue
		}
		for _, c := range p.celulas {
			c.ultimoEstado = celula.Orfa
		}
	}
}

// conferirDocker atualiza o resumo da stack de cada projeto que tem compose. O
// Docker nunca sobe sozinho: isto só lê.
func (m *Motor) conferirDocker() {
	m.mu.Lock()
	type alvo struct {
		p       *projeto
		caminho string
		compose string
	}
	var alvos []alvo
	for _, p := range m.projetos {
		if p.compose != "" {
			alvos = append(alvos, alvo{p: p, caminho: p.caminho, compose: p.compose})
		}
	}
	m.mu.Unlock()

	for _, a := range alvos {
		servicos, err := docker.Servicos(a.caminho, a.compose)
		resumo := ""
		if err == nil {
			resumo = docker.Resumo(servicos)
		}
		m.mu.Lock()
		a.p.docker = resumo
		m.mu.Unlock()
	}
}
