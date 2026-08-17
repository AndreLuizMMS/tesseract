package celula

import (
	"fmt"
	"sync"

	"github.com/andreluiz/tesseract/internal/motor/historico"
)

func init() {
	Registrar(Descritor{
		Tipo:         "sessao",
		Ordem:        0,
		AceitaPrompt: true,
		Conversa:     true,
	}, func() Celula { return &Sessao{} })
}

// abasDaSessao são os agentes que toda sessão carrega por dentro. Criar uma
// sessão não obriga a escolher qual deles será usado: a tecla de aba troca
// entre eles a qualquer momento.
var abasDaSessao = []string{"claude", "cursor", "bash"}

// Sessao é uma célula com várias abas — uma por agente. Só a aba que está sendo
// usada tem processo: as outras nascem quando alguém troca para elas, para uma
// sessão não custar três programas de uma vez.
type Sessao struct {
	mu     sync.Mutex
	config Config
	abas   []string
	ativa  int
	// dentro é a célula de cada aba já aberta, pela ordem de abas.
	dentro     map[string]Celula
	registros  map[string]*historico.Historico
	colunas    int
	linhas     int
	conversas  map[string]string
	encerrando bool
}

func (s *Sessao) Nascer(cfg Config) error {
	s.config = cfg
	s.abas = append([]string{}, abasDaSessao...)
	s.dentro = map[string]Celula{}
	s.registros = map[string]*historico.Historico{}
	s.conversas = map[string]string{}
	for aba, conversa := range cfg.Conversas {
		s.conversas[aba] = conversa
	}
	s.colunas, s.linhas = max(cfg.Colunas, 20), max(cfg.Linhas, 5)

	s.ativa = 0
	for i, aba := range s.abas {
		if aba == cfg.Aba {
			s.ativa = i
		}
	}
	return s.abrirAba(s.abas[s.ativa])
}

// abrirAba sobe o agente de uma aba, se ele ainda não estiver de pé.
func (s *Sessao) abrirAba(aba string) error {
	s.mu.Lock()
	if _, jaAberta := s.dentro[aba]; jaAberta {
		s.mu.Unlock()
		return nil
	}
	config := s.config
	conversa := s.conversas[aba]
	colunas, linhas := s.colunas, s.linhas
	s.mu.Unlock()

	nova, err := Nova(aba)
	if err != nil {
		return err
	}

	registro := config.Historico
	if config.AbrirHistorico != nil {
		aberto, err := config.AbrirHistorico(aba)
		if err != nil {
			return err
		}
		registro = aberto
	}

	config.Aba = aba
	config.Conversa = conversa
	config.Historico = registro
	config.Colunas, config.Linhas = colunas, linhas
	config.AoDescobrirConversa = func(_ string, id string) {
		s.mu.Lock()
		s.conversas[aba] = id
		s.mu.Unlock()
		if s.config.AoDescobrirConversa != nil {
			s.config.AoDescobrirConversa(aba, id)
		}
	}

	if err := nova.Nascer(config); err != nil {
		if registro != nil && registro != s.config.Historico {
			registro.Fechar()
		}
		return err
	}

	s.mu.Lock()
	s.dentro[aba] = nova
	if registro != nil {
		s.registros[aba] = registro
	}
	s.mu.Unlock()
	return nil
}

// atual é a célula da aba ativa, ou nil enquanto ela não subiu.
func (s *Sessao) atual() Celula {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.abas) == 0 {
		return nil
	}
	return s.dentro[s.abas[s.ativa]]
}

// Abas lista as abas da sessão, na ordem em que a tela deve mostrá-las.
func (s *Sessao) Abas() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.abas...)
}

// AbaAtiva é a aba que está aparecendo agora.
func (s *Sessao) AbaAtiva() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.abas) == 0 {
		return ""
	}
	return s.abas[s.ativa]
}

// TrocarAba anda para a aba do lado, subindo o agente dela se for a primeira
// vez. Passo positivo vai para a direita.
func (s *Sessao) TrocarAba(passo int) error {
	s.mu.Lock()
	if len(s.abas) == 0 {
		s.mu.Unlock()
		return fmt.Errorf("a sessão não tem abas")
	}
	if passo == 0 {
		passo = 1
	}
	s.ativa = (s.ativa + passo%len(s.abas) + len(s.abas)) % len(s.abas)
	aba := s.abas[s.ativa]
	s.mu.Unlock()

	if err := s.abrirAba(aba); err != nil {
		return err
	}
	s.avisar()
	return nil
}

func (s *Sessao) avisar() {
	if s.config.Avisar != nil {
		s.config.Avisar()
	}
}

func (s *Sessao) Desenhar() Quadro {
	if atual := s.atual(); atual != nil {
		return atual.Desenhar()
	}
	return Quadro{AoVivo: true, CursorX: -1, CursorY: -1}
}

func (s *Sessao) Tecla(toque Toque) error {
	atual := s.atual()
	if atual == nil {
		return fmt.Errorf("a aba ainda não subiu")
	}
	return atual.Tecla(toque)
}

func (s *Sessao) Estado() Estado {
	if atual := s.atual(); atual != nil {
		return atual.Estado()
	}
	return Parada
}

func (s *Sessao) Estados() []Estado {
	return []Estado{Trabalhando, Respondeu, Aprovar, Caiu, Parada, Orfa}
}

// Redimensionar guarda o tamanho e repassa a todas as abas já abertas: trocar
// de aba não pode mostrar um terminal do tamanho errado.
func (s *Sessao) Redimensionar(colunas, linhas int) error {
	s.mu.Lock()
	if s.colunas == colunas && s.linhas == linhas {
		s.mu.Unlock()
		return nil
	}
	s.colunas, s.linhas = colunas, linhas
	abertas := make([]Celula, 0, len(s.dentro))
	for _, celula := range s.dentro {
		abertas = append(abertas, celula)
	}
	s.mu.Unlock()

	for _, celula := range abertas {
		if err := celula.Redimensionar(colunas, linhas); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sessao) Rolar(delta int, aoVivo bool) {
	if atual := s.atual(); atual != nil {
		atual.Rolar(delta, aoVivo)
	}
}

func (s *Sessao) Matar() error {
	s.mu.Lock()
	if s.encerrando {
		s.mu.Unlock()
		return nil
	}
	s.encerrando = true
	abertas := make([]Celula, 0, len(s.dentro))
	for _, celula := range s.dentro {
		abertas = append(abertas, celula)
	}
	registros := make([]*historico.Historico, 0, len(s.registros))
	for _, registro := range s.registros {
		registros = append(registros, registro)
	}
	s.mu.Unlock()

	for _, celula := range abertas {
		_ = celula.Matar()
	}
	for _, registro := range registros {
		_ = registro.Fechar()
	}
	return nil
}

// HistoricoAtivo é o arquivo da aba que está aparecendo — é nele que a busca
// procura.
func (s *Sessao) HistoricoAtivo() *historico.Historico {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.abas) == 0 {
		return s.config.Historico
	}
	if registro, tem := s.registros[s.abas[s.ativa]]; tem {
		return registro
	}
	return s.config.Historico
}

// Prompt manda trabalho para a aba ativa, quando ela aceita.
func (s *Sessao) Prompt(texto string) error {
	atual := s.atual()
	if atual == nil {
		return fmt.Errorf("a aba ainda não subiu")
	}
	comPrompt, aceita := atual.(ComPrompt)
	if !aceita {
		return fmt.Errorf("a aba %s não recebe prompt", s.AbaAtiva())
	}
	return comPrompt.Prompt(texto)
}

// PropagarNome empurra o nome para dentro do agente da aba ativa.
func (s *Sessao) PropagarNome(nome string) error {
	comConversa, tem := s.atual().(ComConversa)
	if !tem {
		return fmt.Errorf("a aba %s não tem conversa com nome", s.AbaAtiva())
	}
	return comConversa.PropagarNome(nome)
}

// NomeDaConversa é o nome que o agente da aba ativa deu à conversa.
func (s *Sessao) NomeDaConversa() (string, error) {
	comConversa, tem := s.atual().(ComConversa)
	if !tem {
		return "", fmt.Errorf("a aba %s não tem conversa com nome", s.AbaAtiva())
	}
	return comConversa.NomeDaConversa()
}

// Conversa é a identidade da conversa da aba ativa.
func (s *Sessao) Conversa() string {
	if comConversa, tem := s.atual().(ComConversa); tem {
		return comConversa.Conversa()
	}
	return ""
}

// Conversas é o que o motor guarda para reatar cada aba depois de uma queda.
func (s *Sessao) Conversas() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	copia := make(map[string]string, len(s.conversas))
	for aba, conversa := range s.conversas {
		copia[aba] = conversa
	}
	return copia
}
