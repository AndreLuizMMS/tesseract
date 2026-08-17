package celula

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"charm.land/glamour/v2"
	"github.com/fsnotify/fsnotify"
)

func init() {
	Registrar(Descritor{
		Tipo:            "md",
		Ordem:           50,
		RotuloAlvo:      "MD",
		CompletaArquivo: true,
	}, func() Celula { return &Md{} })
}

// esperaDoDisco junta várias mexidas seguidas no arquivo numa releitura só —
// editor costuma escrever em duas ou três etapas.
const esperaDoDisco = 80 * time.Millisecond

// Md é um arquivo markdown renderizado, que recarrega quando o disco muda.
// Só leitura: a célula não edita nada.
type Md struct {
	mu       sync.Mutex
	caminho  string
	colunas  int
	linhas   int
	conteudo []string
	rolagem  int
	estado   Estado
	avisar   func()
	vigia    *fsnotify.Watcher
	parar    chan struct{}
}

func (m *Md) Nascer(cfg Config) error {
	if cfg.Alvo == "" {
		return fmt.Errorf("a célula de markdown precisa saber qual arquivo mostrar")
	}
	caminho := cfg.Alvo
	if !filepath.IsAbs(caminho) {
		caminho = filepath.Join(cfg.Diretorio, caminho)
	}
	m.caminho = caminho
	m.colunas, m.linhas = max(cfg.Colunas, 40), max(cfg.Linhas, 10)
	m.avisar = cfg.Avisar
	m.parar = make(chan struct{})
	m.estado = Parada
	m.reler()

	vigia, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	// O diretório é vigiado, e não o arquivo: editor que salva trocando o
	// arquivo por outro faz o vigia do arquivo perder o rastro.
	if err := vigia.Add(filepath.Dir(caminho)); err != nil {
		vigia.Close()
		return fmt.Errorf("não consegui acompanhar %s: %w", filepath.Dir(caminho), err)
	}
	m.vigia = vigia
	go m.acompanharODisco()
	return nil
}

func (m *Md) acompanharODisco() {
	var espera <-chan time.Time
	for {
		select {
		case <-m.parar:
			return
		case evento, aberto := <-m.vigia.Events:
			if !aberto {
				return
			}
			if filepath.Clean(evento.Name) != filepath.Clean(m.caminho) {
				continue
			}
			espera = time.After(esperaDoDisco)
		case <-m.vigia.Errors:
			continue
		case <-espera:
			espera = nil
			m.reler()
			if m.avisar != nil {
				m.avisar()
			}
		}
	}
}

// reler carrega o arquivo do disco e o renderiza na largura atual.
func (m *Md) reler() {
	m.mu.Lock()
	caminho, colunas := m.caminho, m.colunas
	m.mu.Unlock()

	bruto, err := os.ReadFile(caminho)
	if err != nil {
		m.mu.Lock()
		m.estado = Caiu
		m.conteudo = []string{"", "  não consegui ler " + caminho, "", "  " + erroLegivel(err)}
		m.rolagem = 0
		m.mu.Unlock()
		return
	}

	texto := renderizarMarkdown(string(bruto), colunas)
	m.mu.Lock()
	m.estado = Parada
	m.conteudo = strings.Split(strings.TrimRight(texto, "\n"), "\n")
	if m.rolagem > len(m.conteudo) {
		m.rolagem = 0
	}
	m.mu.Unlock()
}

// renderizarMarkdown desenha o markdown para o terminal. Se o renderizador não
// der conta, o texto cru é melhor do que nada.
func renderizarMarkdown(bruto string, colunas int) string {
	desenhista, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(max(colunas-2, 20)),
	)
	if err != nil {
		return bruto
	}
	saida, err := desenhista.Render(bruto)
	if err != nil {
		return bruto
	}
	return saida
}

func erroLegivel(err error) string {
	if os.IsNotExist(err) {
		return "o arquivo sumiu do disco"
	}
	if os.IsPermission(err) {
		return "sem permissão de leitura"
	}
	return err.Error()
}

func (m *Md) Desenhar() Quadro {
	m.mu.Lock()
	defer m.mu.Unlock()

	quadro := Quadro{CursorX: -1, CursorY: -1, Rolagem: m.rolagem, AoVivo: m.rolagem == 0}
	for i := range m.linhas {
		linha := m.rolagem + i
		if linha >= len(m.conteudo) {
			break
		}
		quadro.Linhas = append(quadro.Linhas, m.conteudo[linha])
	}
	return quadro
}

// Tecla não faz nada: a célula de markdown só lê.
func (m *Md) Tecla(Toque) error { return nil }

func (m *Md) Estado() Estado {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.estado
}

func (m *Md) Estados() []Estado {
	return []Estado{Parada, Caiu, Orfa}
}

func (m *Md) Redimensionar(colunas, linhas int) error {
	m.mu.Lock()
	mudouLargura := colunas != m.colunas
	m.colunas, m.linhas = colunas, linhas
	m.mu.Unlock()
	if mudouLargura {
		m.reler()
	}
	if m.avisar != nil {
		m.avisar()
	}
	return nil
}

// Rolar anda pelo texto. Aqui rolar para cima é voltar ao começo do arquivo.
func (m *Md) Rolar(delta int, aoVivo bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if aoVivo {
		m.rolagem = 0
		return
	}
	teto := max(len(m.conteudo)-m.linhas, 0)
	m.rolagem = min(max(m.rolagem-delta, 0), teto)
}

func (m *Md) Matar() error {
	select {
	case <-m.parar:
	default:
		close(m.parar)
	}
	if m.vigia != nil {
		return m.vigia.Close()
	}
	return nil
}
