// Package motor é o dono de tudo: os processos, a tela interna de cada célula,
// o histórico e o estado desejado. A tela é cliente — desenha o que o motor
// manda e devolve tecla. Nenhuma regra mora na tela.
package motor

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/andreluiz/tesseract/internal/celula"
	"github.com/andreluiz/tesseract/internal/docker"
	"github.com/andreluiz/tesseract/internal/motor/historico"
	"github.com/andreluiz/tesseract/internal/protocolo"
)

// Motor guarda a grade viva.
type Motor struct {
	mu        sync.Mutex
	dirEstado string
	config    Configuracao
	projetos  []*projeto
	tela      string
	aviso     string
	versao    atomic.Uint64
	parar     chan struct{}
	umaVez    sync.Once
}

type projeto struct {
	id      string
	caminho string
	cor     int
	// compose é o arquivo detectado na raiz; vazio quando o projeto não tem.
	compose string
	// docker é o resumo da stack para a tira, atualizado de tempos em tempos.
	docker  string
	celulas []*celulaViva
}

type celulaViva struct {
	id        string
	tipo      string
	nome      string
	alvo      string
	aba       string
	conversas map[string]string
	viva      celula.Celula
	registro  *historico.Historico
	// ultimoEstado é o que o vigia viu da última vez, para notificar só a
	// mudança e não o estado parado.
	ultimoEstado celula.Estado
	// aguardandoNome marca que um /rename automático foi mandado e o vigia
	// precisa adotar o nome assim que o turno terminar.
	aguardandoNome bool
}

// Novo prepara um motor sobre um diretório de estado.
func Novo(dirEstado string, config Configuracao) *Motor {
	if config.TetoHistorico <= 0 {
		config.TetoHistorico = historico.TetoPadrao
	}
	if config.Agentes == nil {
		config.Agentes = map[string]PerfilAgente{}
	}
	return &Motor{
		dirEstado: dirEstado,
		config:    config,
		tela:      "mosaico",
		parar:     make(chan struct{}),
	}
}

// Iniciar carrega o estado desejado e reconstitui a grade. Reatar nunca dispara
// trabalho: célula de agente acorda parada.
func (m *Motor) Iniciar() error {
	estado, erroCarga := Carregar(CaminhoEstado(m.dirEstado))

	m.mu.Lock()
	defer m.mu.Unlock()
	if erroCarga != nil {
		m.aviso = erroCarga.Error()
	}
	if estado.Tela != "" {
		m.tela = estado.Tela
	}
	for _, salvo := range estado.Projetos {
		p := &projeto{id: salvo.ID, caminho: salvo.Caminho, cor: salvo.Cor, compose: docker.Detectar(salvo.Caminho)}
		if p.id == "" {
			p.id = novaIdentidade()
		}
		for _, celulaSalva := range salvo.Celulas {
			c := &celulaViva{
				id:        celulaSalva.ID,
				tipo:      celulaSalva.Tipo,
				nome:      celulaSalva.Nome,
				alvo:      celulaSalva.Alvo,
				aba:       celulaSalva.Aba,
				conversas: map[string]string{},
			}
			for aba, conversa := range celulaSalva.Conversas {
				c.conversas[aba] = conversa
			}
			// Estado escrito antes das abas guardava uma conversa só.
			if celulaSalva.Conversa != "" && len(c.conversas) == 0 {
				c.conversas[celulaSalva.Tipo] = celulaSalva.Conversa
			}
			if c.id == "" {
				c.id = novaIdentidade()
			}
			if err := m.acordar(p, c); err != nil {
				m.aviso = err.Error()
			}
			p.celulas = append(p.celulas, c)
		}
		m.projetos = append(m.projetos, p)
	}
	m.marcarSujo()
	return erroCarga
}

// acordar sobe o processo de uma célula reconstituída. Chamado com o cadeado.
func (m *Motor) acordar(p *projeto, c *celulaViva) error {
	nova, err := celula.Nova(c.tipo)
	if err != nil {
		return err
	}
	registro, err := historico.Abrir(CaminhoHistorico(m.dirEstado, c.id), m.config.TetoHistorico)
	if err != nil {
		return err
	}
	c.registro = registro
	c.viva = nova
	c.ultimoEstado = celula.Trabalhando

	idCelula := c.id
	conversas := map[string]string{}
	for aba, conversa := range c.conversas {
		conversas[aba] = conversa
	}
	return nova.Nascer(celula.Config{
		ID:        c.id,
		Diretorio: p.caminho,
		Nome:      c.nome,
		Alvo:      c.alvo,
		Historico: registro,
		Colunas:   80,
		Linhas:    24,
		Avisar:    m.marcarSujo,

		Aba:       c.aba,
		Conversa:  conversas[c.tipo],
		Conversas: conversas,
		// Fora da linha: quem chama isto pode ser o próprio nascimento da
		// célula, que já está com o cadeado do motor na mão.
		AoDescobrirConversa: func(aba, id string) { go m.guardarConversa(idCelula, aba, id) },

		Perfis: m.perfis(),
		AbrirHistorico: func(sufixo string) (*historico.Historico, error) {
			return historico.Abrir(CaminhoHistorico(m.dirEstado, idCelula+"-"+sufixo), m.config.TetoHistorico)
		},
	})
}

// perfis traduz a configuração do usuário no que as células entendem.
func (m *Motor) perfis() map[string]celula.Perfil {
	perfis := map[string]celula.Perfil{}
	for tipo, perfil := range m.config.Agentes {
		perfis[tipo] = celula.Perfil{
			Programa:        perfil.Programa,
			Args:            perfil.Args,
			ComandoRenomear: perfil.ComandoRenomear,
			Marcadores: celula.Marcadores{
				Trabalho: perfil.MarcadoresTrabalho,
				Pergunta: perfil.MarcadoresPergunta,
			},
		}
	}
	return perfis
}

// guardarConversa anota a identidade da conversa que o agente abriu, para ele
// poder ser reatado depois de uma queda. Numa célula com abas, cada aba tem a
// sua.
func (m *Motor) guardarConversa(idCelula, aba, conversa string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.projetos {
		for _, c := range p.celulas {
			if c.id != idCelula {
				continue
			}
			if aba == "" {
				aba = c.tipo
			}
			if c.conversas == nil {
				c.conversas = map[string]string{}
			}
			if c.conversas[aba] == conversa {
				return
			}
			c.conversas[aba] = conversa
			m.salvar()
			return
		}
	}
}

// Criar nasce uma célula, criando o projeto junto quando o caminho ainda não
// está na tela.
func (m *Motor) Criar(pedido protocolo.Criar) (string, error) {
	caminho, err := validarCaminho(pedido.Caminho)
	if err != nil {
		return "", err
	}
	pedido.Tipo = tipoPedido(pedido)

	m.mu.Lock()
	defer m.mu.Unlock()

	p := m.acharProjetoPorCaminho(caminho)
	novoProjeto := p == nil
	if novoProjeto {
		p = &projeto{id: novaIdentidade(), caminho: caminho, cor: m.proximaCor(), compose: docker.Detectar(caminho)}
	}

	c := &celulaViva{
		id:        novaIdentidade(),
		tipo:      pedido.Tipo,
		nome:      strings.TrimSpace(pedido.Nome),
		alvo:      pedido.Alvo,
		conversas: map[string]string{},
	}
	if c.nome == "" {
		c.nome = nomeAutomatico(p, pedido.Tipo)
	}

	if err := m.acordar(p, c); err != nil {
		if c.registro != nil {
			c.registro.Fechar()
		}
		return "", err
	}

	p.celulas = append(p.celulas, c)
	if novoProjeto {
		m.projetos = append(m.projetos, p)
	}
	m.salvar()
	m.marcarSujo()
	return c.id, nil
}

// tipoPedido decide o que nasce quando o pedido não diz o tipo: um arquivo de
// markdown vira célula de leitura, e o resto vira sessão — que já traz os
// agentes por dentro, sem obrigar ninguém a escolher na hora de criar.
func tipoPedido(pedido protocolo.Criar) string {
	if pedido.Tipo != "" {
		return pedido.Tipo
	}
	alvo := strings.ToLower(strings.TrimSpace(pedido.Alvo))
	if strings.HasSuffix(alvo, ".md") || strings.HasSuffix(alvo, ".markdown") {
		return "md"
	}
	return "sessao"
}

// Matar remove a célula. Se for a última do projeto, o projeto sai da tela — e
// o disco continua exatamente como estava.
func (m *Motor) Matar(idCelula string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for iProjeto, p := range m.projetos {
		for iCelula, c := range p.celulas {
			if c.id != idCelula {
				continue
			}
			if c.viva != nil {
				_ = c.viva.Matar()
			}
			if c.registro != nil {
				_ = c.registro.Fechar()
			}
			_ = os.Remove(CaminhoHistorico(m.dirEstado, c.id))
			p.celulas = append(p.celulas[:iCelula], p.celulas[iCelula+1:]...)
			if len(p.celulas) == 0 {
				m.projetos = append(m.projetos[:iProjeto], m.projetos[iProjeto+1:]...)
			}
			m.salvar()
			m.marcarSujo()
			return nil
		}
	}
	return fmt.Errorf("célula %s não existe", idCelula)
}

// EhUltimaCelula responde se matar essa célula tira o projeto da tela, para a
// confirmação avisar.
func (m *Motor) EhUltimaCelula(idCelula string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.projetos {
		for _, c := range p.celulas {
			if c.id == idCelula {
				return len(p.celulas) == 1
			}
		}
	}
	return false
}

// Tecla entrega a tecla à célula focada. É o modo DIGITAR.
func (m *Motor) Tecla(pedido protocolo.Tecla) error {
	idCelula := pedido.Celula
	c := m.acharCelula(idCelula)
	if c == nil || c.viva == nil {
		return fmt.Errorf("célula %s não existe", idCelula)
	}
	toque := celula.Toque{
		Codigo: pedido.Codigo,
		Texto:  pedido.Texto,
		Mod:    pedido.Mod,
		Colar:  pedido.Colar,
	}
	if err := c.viva.Tecla(toque); err != nil {
		return err
	}
	m.marcarSujo()
	return nil
}

// Tamanho ajusta a área da célula ao que a tela reservou para ela.
func (m *Motor) Tamanho(idCelula string, colunas, linhas int) error {
	c := m.acharCelula(idCelula)
	if c == nil || c.viva == nil {
		return fmt.Errorf("célula %s não existe", idCelula)
	}
	return c.viva.Redimensionar(colunas, linhas)
}

// Rolar move a leitura do histórico da célula.
func (m *Motor) Rolar(idCelula string, delta int, aoVivo bool) error {
	c := m.acharCelula(idCelula)
	if c == nil || c.viva == nil {
		return fmt.Errorf("célula %s não existe", idCelula)
	}
	c.viva.Rolar(delta, aoVivo)
	m.marcarSujo()
	return nil
}

// Buscar procura um termo no histórico da célula.
func (m *Motor) Buscar(idCelula, termo string) ([]historico.Ocorrencia, error) {
	c := m.acharCelula(idCelula)
	if c == nil {
		return nil, fmt.Errorf("célula %s não existe", idCelula)
	}
	return m.registroDe(c).Buscar(termo)
}

// registroDe é o histórico que a busca e a rolagem devem olhar: numa célula com
// abas, o da aba que está aparecendo.
func (m *Motor) registroDe(c *celulaViva) *historico.Historico {
	if comHistorico, tem := c.viva.(celula.ComHistorico); tem {
		if registro := comHistorico.HistoricoAtivo(); registro != nil {
			return registro
		}
	}
	return c.registro
}

// Renomear troca o rótulo da célula. Nome é rótulo: não mexe em processo,
// diretório nem histórico.
func (m *Motor) Renomear(idCelula, nome string) error {
	nome = strings.TrimSpace(nome)
	if nome == "" {
		return fmt.Errorf("o nome não pode ficar vazio")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.projetos {
		for _, c := range p.celulas {
			if c.id == idCelula {
				c.nome = nome
				m.salvar()
				m.marcarSujo()
				return nil
			}
		}
	}
	return fmt.Errorf("célula %s não existe", idCelula)
}

// Retrato é o estado inteiro pronto para desenhar.
func (m *Motor) Retrato() protocolo.Estado {
	m.mu.Lock()
	defer m.mu.Unlock()

	retrato := protocolo.Estado{Aviso: m.aviso, Tela: m.tela, Tipos: fichasDosTipos(), Quota: LerQuota()}
	for _, p := range m.projetos {
		desenho := protocolo.Projeto{
			ID:         p.id,
			Caminho:    p.caminho,
			Nome:       filepath.Base(p.caminho),
			Cor:        p.cor,
			TemCompose: p.compose != "",
			Docker:     p.docker,
		}
		for _, c := range p.celulas {
			retratoCelula := protocolo.Celula{
				ID:     c.id,
				Tipo:   c.tipo,
				Nome:   c.nome,
				Estado: string(celula.Parada),
				AoVivo: true,
			}
			if comAbas, tem := c.viva.(celula.ComAbas); tem {
				retratoCelula.Abas = comAbas.Abas()
				retratoCelula.Aba = comAbas.AbaAtiva()
			}
			if c.viva != nil {
				quadro := c.viva.Desenhar()
				retratoCelula.Estado = string(c.viva.Estado())
				retratoCelula.Linhas = quadro.Linhas
				retratoCelula.CursorX = quadro.CursorX
				retratoCelula.CursorY = quadro.CursorY
				retratoCelula.Rolagem = quadro.Rolagem
				retratoCelula.AoVivo = quadro.AoVivo
			}
			desenho.Celulas = append(desenho.Celulas, retratoCelula)
		}
		retrato.Projetos = append(retrato.Projetos, desenho)
	}
	return retrato
}

// proximaCor escolhe a cor da coluna do projeto novo: a primeira que ninguém
// está usando, para que dois projetos vizinhos nunca fiquem iguais.
func (m *Motor) proximaCor() int {
	usadas := map[int]bool{}
	for _, p := range m.projetos {
		usadas[p.cor] = true
	}
	for cor := 0; ; cor++ {
		if !usadas[cor] {
			return cor
		}
	}
}

// fichasDosTipos entrega à tela o que ela precisa para montar o formulário sem
// conhecer nenhum tipo por dentro. A lista de tipos é fixa desde a partida, e
// vai em todo retrato: montar uma vez basta.
var fichasDosTipos = sync.OnceValue(func() []protocolo.TipoCelula {
	var fichas []protocolo.TipoCelula
	for _, descritor := range celula.Descritores() {
		fichas = append(fichas, protocolo.TipoCelula{
			Tipo:            descritor.Tipo,
			RotuloAlvo:      descritor.RotuloAlvo,
			CompletaArquivo: descritor.CompletaArquivo,
			AceitaPrompt:    descritor.AceitaPrompt,
			Conversa:        descritor.Conversa,
		})
	}
	return fichas
})

// Resumo é o que `tess status` mostra.
func (m *Motor) Resumo() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.projetos) == 0 {
		return "motor rodando, nenhum projeto na tela"
	}
	var texto strings.Builder
	celulas := 0
	for _, p := range m.projetos {
		celulas += len(p.celulas)
	}
	fmt.Fprintf(&texto, "motor rodando · %d projeto(s) · %d célula(s)\n", len(m.projetos), celulas)
	for _, p := range m.projetos {
		fmt.Fprintf(&texto, "\n%s  %s\n", filepath.Base(p.caminho), p.caminho)
		for _, c := range p.celulas {
			estado := celula.Parada
			if c.viva != nil {
				estado = c.viva.Estado()
			}
			fmt.Fprintf(&texto, "  %s %-7s %s\n", estado.Marcador(), c.tipo, c.nome)
		}
	}
	return texto.String()
}

// Encerrar mata todas as células e solta os arquivos. O estado desejado
// continua em disco para a próxima subida.
func (m *Motor) Encerrar() {
	m.pararVigia()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.projetos {
		for _, c := range p.celulas {
			if c.viva != nil {
				_ = c.viva.Matar()
			}
			if c.registro != nil {
				_ = c.registro.Fechar()
			}
		}
	}
}

// Versao sobe a cada mudança. Cada tela conectada compara com a que já
// desenhou, para que mais de uma tela possa acompanhar o mesmo motor.
func (m *Motor) Versao() uint64 { return m.versao.Load() }

func (m *Motor) marcarSujo() { m.versao.Add(1) }

// TrocarTela lembra a tela escolhida entre execuções.
func (m *Motor) TrocarTela(tela string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tela = tela
	m.salvar()
}

// Tela é a última tela escolhida.
func (m *Motor) Tela() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tela
}

// salvar grava o estado desejado. Chamado sempre com o cadeado na mão.
func (m *Motor) salvar() {
	estado := EstadoDesejado{Tela: m.tela}
	for _, p := range m.projetos {
		salvo := ProjetoSalvo{ID: p.id, Caminho: p.caminho, Cor: p.cor}
		for _, c := range p.celulas {
			guardada := CelulaSalva{ID: c.id, Tipo: c.tipo, Nome: c.nome, Alvo: c.alvo, Aba: c.aba}
			if len(c.conversas) > 0 {
				guardada.Conversas = c.conversas
			}
			salvo.Celulas = append(salvo.Celulas, guardada)
		}
		estado.Projetos = append(estado.Projetos, salvo)
	}
	if err := Salvar(CaminhoEstado(m.dirEstado), estado); err != nil {
		m.aviso = "não consegui gravar o estado: " + err.Error()
	}
}

func (m *Motor) acharCelula(id string) *celulaViva {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.projetos {
		for _, c := range p.celulas {
			if c.id == id {
				return c
			}
		}
	}
	return nil
}

func (m *Motor) acharProjetoPorCaminho(caminho string) *projeto {
	for _, p := range m.projetos {
		if p.caminho == caminho {
			return p
		}
	}
	return nil
}

// nomeAutomatico dá o próximo nome livre do tipo dentro do projeto.
func nomeAutomatico(p *projeto, tipo string) string {
	for n := 1; ; n++ {
		candidato := fmt.Sprintf("%s %d", tipo, n)
		livre := true
		for _, c := range p.celulas {
			if c.nome == candidato {
				livre = false
				break
			}
		}
		if livre {
			return candidato
		}
	}
}

// validarCaminho exige um diretório que existe e é gravável, na hora.
func validarCaminho(bruto string) (string, error) {
	bruto = strings.TrimSpace(bruto)
	if bruto == "" {
		return "", fmt.Errorf("o projeto precisa de um caminho")
	}
	if strings.HasPrefix(bruto, "~") {
		if casa, err := os.UserHomeDir(); err == nil {
			bruto = filepath.Join(casa, strings.TrimPrefix(bruto, "~"))
		}
	}
	caminho, err := filepath.Abs(bruto)
	if err != nil {
		return "", fmt.Errorf("caminho inválido: %s", bruto)
	}
	info, err := os.Stat(caminho)
	if err != nil {
		return "", fmt.Errorf("o diretório não existe: %s", caminho)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("o caminho não é um diretório: %s", caminho)
	}
	if err := syscall.Access(caminho, 2 /* W_OK */); err != nil {
		return "", fmt.Errorf("sem permissão de escrita em: %s", caminho)
	}
	return caminho, nil
}

// novaIdentidade é a identidade estável da célula ou do projeto — ela não é o
// processo, e sobrevive a ele.
func novaIdentidade() string {
	bruto := make([]byte, 6)
	if _, err := rand.Read(bruto); err != nil {
		return fmt.Sprintf("%d", os.Getpid())
	}
	return hex.EncodeToString(bruto)
}
