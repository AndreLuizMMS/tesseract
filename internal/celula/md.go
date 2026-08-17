package celula

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/x/vt"
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

const (
	// esperaDoDisco junta várias mexidas seguidas no arquivo numa releitura só —
	// editor costuma escrever em duas ou três etapas.
	esperaDoDisco = 80 * time.Millisecond
	// cadenciaDaVarredura é de quanto em quanto tempo a aba procura arquivos
	// novos no projeto. Espaçada de propósito: a varredura também acontece
	// toda vez que a aba volta a aparecer, que é quando ela importa.
	cadenciaDaVarredura = 60 * time.Second
	// fundoMaximo é até que profundidade a procura desce a partir da raiz do
	// projeto.
	fundoMaximo = 6
	// tetoDeArquivos evita que um projeto gigante trave a varredura.
	tetoDeArquivos = 2000
)

// pastasQueNaoTemDoc não guardam documentação do projeto e custam caro para
// varrer.
var pastasQueNaoTemDoc = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true, "dist": true,
	"build": true, "target": true, ".next": true, ".venv": true,
	"venv": true, "coverage": true, ".cache": true, ".idea": true,
}

// Md é a aba de markdown: uma lista dos arquivos do projeto com busca por nome
// em cima, e o arquivo escolhido renderizado. Só leitura — a célula não edita
// nada.
type Md struct {
	mu        sync.Mutex
	diretorio string
	arquivos  []string // caminhos relativos à raiz do projeto
	filtro    string
	escolhido int
	// aberto é o arquivo em leitura; vazio quer dizer que a lista está na tela.
	aberto   string
	conteudo []string
	rolagem  int
	colunas  int
	linhas   int
	estado   Estado
	avisar   func()
	vigia    *fsnotify.Watcher
	vigiando string
	parar    chan struct{}
}

func (m *Md) Nascer(cfg Config) error {
	m.diretorio = cfg.Diretorio
	m.colunas, m.linhas = max(cfg.Colunas, 40), max(cfg.Linhas, 10)
	m.avisar = cfg.Avisar
	m.parar = make(chan struct{})
	m.estado = Parada

	vigia, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	m.vigia = vigia

	m.varrer()
	// Criada apontando um arquivo, a aba já abre nele; sem alvo, começa na
	// lista.
	if cfg.Alvo != "" {
		m.abrir(cfg.Alvo)
	}

	go m.acompanharODisco()
	go m.varrerDeTemposEmTempos()
	return nil
}

// varrer procura os arquivos de markdown do projeto.
func (m *Md) varrer() {
	m.mu.Lock()
	raiz := m.diretorio
	m.mu.Unlock()

	var achados []string
	_ = filepath.WalkDir(raiz, func(caminho string, entrada os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relativo, erroRelativo := filepath.Rel(raiz, caminho)
		if erroRelativo != nil {
			return nil
		}
		if entrada.IsDir() {
			nome := entrada.Name()
			if caminho == raiz {
				return nil
			}
			if pastasQueNaoTemDoc[nome] || (strings.HasPrefix(nome, ".") && nome != ".github") {
				return filepath.SkipDir
			}
			if strings.Count(relativo, string(os.PathSeparator)) >= fundoMaximo {
				return filepath.SkipDir
			}
			return nil
		}
		if !ehMarkdown(entrada.Name()) {
			return nil
		}
		achados = append(achados, relativo)
		if len(achados) >= tetoDeArquivos {
			return filepath.SkipAll
		}
		return nil
	})

	// Os de cima primeiro: o README da raiz interessa mais que o quarto nível.
	sort.Slice(achados, func(i, j int) bool {
		fundoI := strings.Count(achados[i], string(os.PathSeparator))
		fundoJ := strings.Count(achados[j], string(os.PathSeparator))
		if fundoI != fundoJ {
			return fundoI < fundoJ
		}
		return achados[i] < achados[j]
	})

	m.mu.Lock()
	m.arquivos = achados
	m.escolhido = min(m.escolhido, max(len(m.filtrados())-1, 0))
	m.mu.Unlock()
}

func ehMarkdown(nome string) bool {
	minusculo := strings.ToLower(nome)
	return strings.HasSuffix(minusculo, ".md") || strings.HasSuffix(minusculo, ".markdown")
}

// filtrados são os arquivos que casam com a busca. Chamado com o cadeado.
func (m *Md) filtrados() []string {
	if strings.TrimSpace(m.filtro) == "" {
		return m.arquivos
	}
	alvo := strings.ToLower(m.filtro)
	var casaram []string
	for _, arquivo := range m.arquivos {
		if strings.Contains(strings.ToLower(filepath.Base(arquivo)), alvo) {
			casaram = append(casaram, arquivo)
			continue
		}
		// O caminho também vale: quem digita "docs/spec" sabe o que quer.
		if strings.Contains(strings.ToLower(arquivo), alvo) {
			casaram = append(casaram, arquivo)
		}
	}
	return casaram
}

// abrir põe um arquivo em leitura.
func (m *Md) abrir(caminho string) {
	if !filepath.IsAbs(caminho) {
		caminho = filepath.Join(m.diretorio, caminho)
	}
	m.mu.Lock()
	m.aberto = caminho
	m.rolagem = 0
	m.mu.Unlock()

	m.acompanharArquivo(caminho)
	m.reler()
}

// fechar devolve a aba para a lista.
func (m *Md) fechar() {
	m.mu.Lock()
	m.aberto, m.conteudo, m.rolagem, m.estado = "", nil, 0, Parada
	m.mu.Unlock()
}

// acompanharArquivo passa a vigiar o diretório do arquivo aberto. É o diretório
// e não o arquivo porque editor que salva trocando o arquivo por outro faz o
// vigia do arquivo perder o rastro.
func (m *Md) acompanharArquivo(caminho string) {
	pasta := filepath.Dir(caminho)
	m.mu.Lock()
	anterior, vigia := m.vigiando, m.vigia
	m.vigiando = pasta
	m.mu.Unlock()

	if vigia == nil || anterior == pasta {
		return
	}
	if anterior != "" {
		_ = vigia.Remove(anterior)
	}
	_ = vigia.Add(pasta)
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
			m.mu.Lock()
			interessa := m.aberto != "" && filepath.Clean(evento.Name) == filepath.Clean(m.aberto)
			m.mu.Unlock()
			if !interessa {
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

func (m *Md) varrerDeTemposEmTempos() {
	relogio := time.NewTicker(cadenciaDaVarredura)
	defer relogio.Stop()
	for {
		select {
		case <-m.parar:
			return
		case <-relogio.C:
			m.varrer()
		}
	}
}

// reler carrega o arquivo aberto e o renderiza na largura atual.
func (m *Md) reler() {
	m.mu.Lock()
	caminho, colunas := m.aberto, m.colunas
	m.mu.Unlock()
	if caminho == "" {
		return
	}

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
	if m.aberto == "" {
		quadro.Linhas = m.desenharLista()
		return quadro
	}

	quadro.Linhas = append(quadro.Linhas, corDoCaminho+relativoA(m.diretorio, m.aberto)+resetDeCor+"   esc volta à lista")
	for i := range max(m.linhas-1, 1) {
		linha := m.rolagem + i
		if linha >= len(m.conteudo) {
			break
		}
		quadro.Linhas = append(quadro.Linhas, m.conteudo[linha])
	}
	return quadro
}

// desenharLista monta a busca em cima e os arquivos embaixo. Chamado com o
// cadeado.
func (m *Md) desenharLista() []string {
	casaram := m.filtrados()
	linhas := []string{
		corDaBusca + "buscar: " + resetDeCor + m.filtro + "▏",
		corApagadaDoMd + strconv.Itoa(len(casaram)) + " de " + strconv.Itoa(len(m.arquivos)) + " arquivos · ↑↓ escolhe · ↵ abre" + resetDeCor,
		"",
	}

	cabem := max(m.linhas-len(linhas), 1)
	primeiro := min(max(m.escolhido-cabem/2, 0), max(len(casaram)-cabem, 0))
	for i := primeiro; i < len(casaram) && i-primeiro < cabem; i++ {
		marca := "  "
		nome := casaram[i]
		if i == m.escolhido {
			marca = "▸ "
			nome = corEscolhido + nome + resetDeCor
		}
		linhas = append(linhas, marca+nome)
	}
	if len(casaram) == 0 {
		linhas = append(linhas, corApagadaDoMd+"  nenhum markdown casa com a busca"+resetDeCor)
	}
	return linhas
}

// Cores da aba de markdown. Escritas aqui em vez de na tela porque a célula
// desenha o próprio conteúdo — a tela só recebe linhas prontas.
const (
	corDaBusca     = "\x1b[1;38;5;252m"
	corEscolhido   = "\x1b[1;38;5;42m"
	corDoCaminho   = "\x1b[38;5;39m"
	corApagadaDoMd = "\x1b[38;5;240m"
	resetDeCor     = "\x1b[0m"
)

func relativoA(raiz, caminho string) string {
	if relativo, err := filepath.Rel(raiz, caminho); err == nil {
		return relativo
	}
	return caminho
}

// Tecla é a busca e a navegação da lista: o que se digita filtra, as setas
// escolhem e o enter abre. Em leitura, esc volta para a lista.
func (m *Md) Tecla(toque Toque) error {
	m.mu.Lock()
	emLeitura := m.aberto != ""
	m.mu.Unlock()

	if emLeitura {
		switch toque.Codigo {
		case vt.KeyEscape:
			m.fechar()
		case vt.KeyUp:
			m.Rolar(1, false)
		case vt.KeyDown:
			m.Rolar(-1, false)
		case vt.KeyPgUp:
			m.Rolar(m.altura(), false)
		case vt.KeyPgDown:
			m.Rolar(-m.altura(), false)
		}
		m.tocar()
		return nil
	}

	switch toque.Codigo {
	case vt.KeyEnter:
		m.mu.Lock()
		casaram := m.filtrados()
		escolhido := ""
		if m.escolhido < len(casaram) {
			escolhido = casaram[m.escolhido]
		}
		m.mu.Unlock()
		if escolhido != "" {
			m.abrir(escolhido)
		}
	case vt.KeyUp:
		m.andarNaLista(-1)
	case vt.KeyDown:
		m.andarNaLista(1)
	case vt.KeyBackspace:
		m.mu.Lock()
		if runas := []rune(m.filtro); len(runas) > 0 {
			m.filtro = string(runas[:len(runas)-1])
		}
		m.escolhido = 0
		m.mu.Unlock()
	case vt.KeyEscape:
		m.mu.Lock()
		m.filtro, m.escolhido = "", 0
		m.mu.Unlock()
	default:
		if toque.Colar != "" {
			m.mu.Lock()
			m.filtro += toque.Colar
			m.escolhido = 0
			m.mu.Unlock()
			break
		}
		if toque.Texto != "" {
			m.mu.Lock()
			m.filtro += toque.Texto
			m.escolhido = 0
			m.mu.Unlock()
		}
	}
	m.tocar()
	return nil
}

func (m *Md) andarNaLista(passo int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	casaram := len(m.filtrados())
	if casaram == 0 {
		m.escolhido = 0
		return
	}
	m.escolhido = min(max(m.escolhido+passo, 0), casaram-1)
}

func (m *Md) altura() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return max(m.linhas-2, 1)
}

func (m *Md) tocar() {
	if m.avisar != nil {
		m.avisar()
	}
}

// Atualizar é chamado quando a aba volta a aparecer: é a hora certa de
// procurar arquivo novo, e não de minuto em minuto no escuro.
func (m *Md) Atualizar() {
	m.varrer()
	m.reler()
	m.tocar()
}

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
	m.tocar()
	return nil
}

// Rolar anda pelo texto do arquivo aberto. Aqui rolar para cima é voltar ao
// começo.
func (m *Md) Rolar(delta int, aoVivo bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if aoVivo {
		m.rolagem = 0
		return
	}
	if m.aberto == "" {
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
