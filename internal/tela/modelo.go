package tela

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

// linhasPorGiro é quanto a roda do mouse anda de cada vez.
const linhasPorGiro = 3

const (
	visaoMosaico = "mosaico"
	visaoLista   = "lista"
)

type chegouEstado protocolo.Estado

type chegouCompletado protocolo.Completado

type chegouAchados protocolo.Achados

type chegouServicos protocolo.Servicos

type chegouErro string

type morreuMotor struct{}

// Modelo é a tela. Ela guarda o que o motor mandou desenhar, o modo do teclado
// e onde está o foco — nenhuma regra de negócio mora aqui.
type Modelo struct {
	cliente *Cliente
	estado  protocolo.Estado
	foco    Foco
	visao   string
	largura int
	altura  int
	erro    string

	digitando bool
	tamanhos  map[string]Geometria

	painel          *PainelDocker
	formulario      *Formulario
	pergunta        *Pergunta
	confirmacao     *Confirmacao
	ajuda           bool
	achados         *protocolo.Achados
	achadoEscolhido int
}

// NovoModelo prepara a tela ligada a um motor já conectado, partindo do
// primeiro retrato que ele mandou.
func NovoModelo(cliente *Cliente, inicial protocolo.Estado) *Modelo {
	visao := inicial.Tela
	if visao != visaoLista {
		visao = visaoMosaico
	}
	return &Modelo{
		cliente:  cliente,
		estado:   inicial,
		visao:    visao,
		tamanhos: map[string]Geometria{},
	}
}

// Ouvir passa as mensagens do motor para dentro do programa da tela.
func (m *Modelo) Ouvir(programa *tea.Program) {
	go func() {
		for {
			envelope, err := m.cliente.Receber()
			if err != nil {
				programa.Send(morreuMotor{})
				return
			}
			switch envelope.Tipo {
			case protocolo.TipoEstado:
				if estado, err := protocolo.Desempacotar[protocolo.Estado](envelope); err == nil {
					programa.Send(chegouEstado(estado))
				}
			case protocolo.TipoCompletado:
				if resposta, err := protocolo.Desempacotar[protocolo.Completado](envelope); err == nil {
					programa.Send(chegouCompletado(resposta))
				}
			case protocolo.TipoServicos:
				if servicos, err := protocolo.Desempacotar[protocolo.Servicos](envelope); err == nil {
					programa.Send(chegouServicos(servicos))
				}
			case protocolo.TipoAchados:
				if achados, err := protocolo.Desempacotar[protocolo.Achados](envelope); err == nil {
					programa.Send(chegouAchados(achados))
				}
			case protocolo.TipoErro:
				if erro, err := protocolo.Desempacotar[protocolo.Erro](envelope); err == nil {
					programa.Send(chegouErro(erro.Mensagem))
				}
			}
		}
	}()
}

func (m *Modelo) Init() tea.Cmd { return nil }

func (m *Modelo) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.largura, m.altura = msg.Width, msg.Height
		m.avisarTamanhos()

	case chegouEstado:
		m.estado = protocolo.Estado(msg)
		m.foco = Ajustar(m.estado, m.foco)
		m.avisarTamanhos()

	case chegouCompletado:
		if m.formulario != nil {
			m.formulario.Completou(protocolo.Completado(msg))
		}

	case chegouServicos:
		if m.painel != nil {
			m.painel.Chegou(protocolo.Servicos(msg))
		}

	case chegouAchados:
		achados := protocolo.Achados(msg)
		m.achados, m.achadoEscolhido = &achados, 0

	case chegouErro:
		m.erro = string(msg)

	case morreuMotor:
		return m, tea.Quit

	case tea.MouseWheelMsg:
		return m, m.rolar(msg)

	case tea.KeyPressMsg:
		return m.teclou(msg)
	}
	return m, nil
}

// Modo é quem tem o teclado agora — o que a barra de título mostra.
func (m *Modelo) Modo() teclado.Modo {
	switch {
	case m.painel != nil:
		return teclado.PainelDocker
	case m.formulario != nil || m.pergunta != nil || m.confirmacao != nil:
		return teclado.Formulario
	case m.digitando:
		return teclado.Digitar
	}
	return teclado.Navegar
}

// teclou entrega a tecla a quem for o dono dela agora. Nunca há dois donos ao
// mesmo tempo, então colisão é estruturalmente impossível.
func (m *Modelo) teclou(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	tecla := msg.String()
	apagou := msg.Code == tea.KeyBackspace

	switch {
	case m.painel != nil:
		return m, m.telaDoPainel(tecla)
	case m.formulario != nil:
		return m, m.telaDoFormulario(tecla, msg.Text, apagou)
	case m.pergunta != nil:
		return m, m.telaDaPergunta(tecla, msg.Text, apagou)
	case m.confirmacao != nil:
		return m, m.telaDaConfirmacao(tecla)
	case m.ajuda:
		if acao := teclado.Buscar(teclado.Navegar, tecla); acao == teclado.Voltar || acao == teclado.Ajuda {
			m.ajuda = false
		}
		return m, nil
	case m.achados != nil:
		return m, m.telaDosAchados(tecla)
	case m.digitando:
		return m, m.telaDigitando(tecla, msg)
	}
	return m.telaNavegando(tecla)
}

func (m *Modelo) telaDigitando(tecla string, msg tea.KeyPressMsg) tea.Cmd {
	if teclado.Buscar(teclado.Digitar, tecla) == teclado.SairDigitar {
		m.digitando = false
		return nil
	}
	// Todo o resto vai para a célula, sem exceção.
	foco := m.celulaFocada()
	if foco == nil {
		return nil
	}
	m.enviar(protocolo.TipoTecla, protocolo.Tecla{
		Celula: foco.ID,
		Codigo: msg.Code,
		Texto:  msg.Text,
		Mod:    int(msg.Mod),
	})
	return nil
}

func (m *Modelo) telaNavegando(tecla string) (tea.Model, tea.Cmd) {
	m.erro = ""
	acao := teclado.Buscar(teclado.Navegar, tecla)
	projeto, celula := m.focoAtual()

	switch acao {
	case teclado.CelulaAnterior:
		m.foco.Celula--
	case teclado.CelulaProxima:
		m.foco.Celula++
	case teclado.ProjetoAnterior:
		m.foco.Projeto, m.foco.Celula = m.foco.Projeto-1, 0
	case teclado.ProjetoProximo:
		m.foco.Projeto, m.foco.Celula = m.foco.Projeto+1, 0
	case teclado.IrParaProjeto:
		if n, err := strconv.Atoi(tecla); err == nil {
			m.foco.Projeto, m.foco.Celula = n-1, 0
		}
	case teclado.PularChamado:
		m.pularParaQuemChamou()

	case teclado.EntrarDigitar:
		if celula != nil {
			m.digitando = true
		}
	case teclado.TelaCheia:
		m.foco.Cheia = !m.foco.Cheia
	case teclado.TrocarTela:
		m.visao = visaoLista
		if !m.mostrandoMosaico() {
			m.visao = visaoMosaico
		}
		m.enviar(protocolo.TipoTela, protocolo.Tela{Tela: m.visao})

	case teclado.Criar:
		caminho := ""
		if projeto != nil {
			caminho = projeto.Caminho
		}
		m.formulario = NovoFormulario(m.estado.Tipos, caminho)
	case teclado.Retomar:
		if celula != nil {
			m.enviar(protocolo.TipoRetomar, protocolo.Retomar{Celula: celula.ID})
		}
	case teclado.Matar:
		if celula != nil {
			m.confirmacao = confirmacaoDeMorte(*projeto, *celula)
		}

	case teclado.Renomear:
		if celula != nil {
			m.pergunta = &Pergunta{
				Titulo: "RENOMEAR", Rotulo: "NOME", Texto: celula.Nome,
				Dica: "o nome vai junto para dentro do agente", Acao: teclado.Renomear,
			}
		}
	case teclado.AdotarNome:
		if celula != nil {
			m.enviar(protocolo.TipoAdotar, protocolo.Adotar{Celula: celula.ID})
		}

	case teclado.Prompt:
		if celula != nil {
			m.pergunta = &Pergunta{
				Titulo: "PROMPT · " + celula.Nome, Rotulo: "PROMPT",
				Dica: "vai para a célula sem você entrar nela", Acao: teclado.Prompt,
			}
		}
	case teclado.AbrirDocker:
		m.abrirDocker()
	case teclado.AbrirEditor:
		if projeto != nil {
			m.enviar(protocolo.TipoEditor, protocolo.Editor{Projeto: projeto.ID})
		}

	case teclado.BuscarTermo:
		if celula != nil {
			m.pergunta = &Pergunta{
				Titulo: "BUSCAR", Rotulo: "TERMO",
				Dica: "procura no histórico da célula focada", Acao: teclado.BuscarTermo,
			}
		}
	case teclado.Voltar:
		if m.foco.Cheia {
			m.foco.Cheia = false
			break
		}
		if celula != nil {
			m.enviar(protocolo.TipoRolar, protocolo.Rolar{Celula: celula.ID, AoVivo: true})
		}
	case teclado.Ajuda:
		m.ajuda = true
	case teclado.Fechar:
		return m, tea.Quit
	}

	m.foco = Ajustar(m.estado, m.foco)
	m.avisarTamanhos()
	return m, nil
}

// telaDoPainel entrega a tecla ao painel Docker, que é o dono do teclado
// enquanto está aberto.
func (m *Modelo) telaDoPainel(tecla string) tea.Cmd {
	pedido, aberto := m.painel.Tecla(tecla)
	if pedido != nil {
		m.enviar(protocolo.TipoDocker, *pedido)
	}
	if !aberto {
		m.painel = nil
	}
	return nil
}

func (m *Modelo) telaDoFormulario(tecla, texto string, apagou bool) tea.Cmd {
	acao := teclado.Buscar(teclado.Formulario, tecla)
	if pedido, quer := m.formulario.PedeCompletar(acao); quer {
		m.enviar(protocolo.TipoCompletar, pedido)
		return nil
	}
	criar, aberto := m.formulario.Tecla(acao, texto, apagou)
	if criar != nil {
		m.enviar(protocolo.TipoCriar, *criar)
	}
	if !aberto {
		m.formulario = nil
	}
	return nil
}

func (m *Modelo) telaDaPergunta(tecla, texto string, apagou bool) tea.Cmd {
	acao := teclado.Buscar(teclado.Formulario, tecla)
	confirmou, aberta := m.pergunta.Tecla(acao, texto, apagou)
	if aberta {
		return nil
	}
	pergunta := m.pergunta
	m.pergunta = nil
	if !confirmou {
		return nil
	}
	celula := m.celulaFocada()
	if celula == nil {
		return nil
	}
	switch pergunta.Acao {
	case teclado.Renomear:
		m.enviar(protocolo.TipoRenomear, protocolo.Renomear{Celula: celula.ID, Nome: pergunta.Texto})
	case teclado.Prompt:
		m.enviar(protocolo.TipoPrompt, protocolo.Prompt{Celula: celula.ID, Texto: pergunta.Texto})
	case teclado.BuscarTermo:
		m.enviar(protocolo.TipoBuscar, protocolo.Buscar{Celula: celula.ID, Termo: pergunta.Texto})
	}
	return nil
}

func (m *Modelo) telaDaConfirmacao(tecla string) tea.Cmd {
	acao := teclado.Buscar(teclado.Formulario, tecla)
	if acao != teclado.Confirmar && acao != teclado.Cancelar {
		return nil
	}
	confirmacao := m.confirmacao
	m.confirmacao = nil
	if acao != teclado.Confirmar {
		return nil
	}
	if confirmacao.Acao == teclado.Matar {
		m.enviar(protocolo.TipoMatar, protocolo.Matar{Celula: confirmacao.Alvo})
	}
	return nil
}

func (m *Modelo) telaDosAchados(tecla string) tea.Cmd {
	switch teclado.Buscar(teclado.Navegar, tecla) {
	case teclado.CelulaAnterior:
		m.achadoEscolhido = max(m.achadoEscolhido-1, 0)
	case teclado.CelulaProxima:
		m.achadoEscolhido = min(m.achadoEscolhido+1, max(len(m.achados.Linhas)-1, 0))
	case teclado.Voltar:
		m.achados = nil
	case teclado.EntrarDigitar:
		if len(m.achados.Linhas) > 0 {
			m.enviar(protocolo.TipoIrParaLinha, protocolo.IrParaLinha{
				Celula: m.achados.Celula,
				Linha:  m.achados.Linhas[m.achadoEscolhido].Linha,
			})
		}
		m.achados = nil
	}
	return nil
}

// pularParaQuemChamou vai para a próxima célula que pede atenção, atravessando
// projeto.
func (m *Modelo) pularParaQuemChamou() {
	chama := func(estado string) bool { return estado == "respondeu" || estado == "aprovar" }
	total := 0
	for _, projeto := range m.estado.Projetos {
		total += len(projeto.Celulas)
	}
	if total == 0 {
		return
	}
	atual := m.posicaoLinear()
	for passo := 1; passo <= total; passo++ {
		p, c := m.posicaoDe((atual + passo) % total)
		if chama(m.estado.Projetos[p].Celulas[c].Estado) {
			m.foco.Projeto, m.foco.Celula = p, c
			return
		}
	}
}

func (m *Modelo) posicaoLinear() int {
	linear := 0
	for i, projeto := range m.estado.Projetos {
		if i == m.foco.Projeto {
			return linear + m.foco.Celula
		}
		linear += len(projeto.Celulas)
	}
	return 0
}

func (m *Modelo) posicaoDe(linear int) (int, int) {
	for i, projeto := range m.estado.Projetos {
		if linear < len(projeto.Celulas) {
			return i, linear
		}
		linear -= len(projeto.Celulas)
	}
	return 0, 0
}

// rolar move a leitura do histórico. A roda do mouse vale nos dois modos,
// porque não é tecla.
func (m *Modelo) rolar(msg tea.MouseWheelMsg) tea.Cmd {
	foco := m.celulaFocada()
	if foco == nil {
		return nil
	}
	delta := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = linhasPorGiro
	case tea.MouseWheelDown:
		delta = -linhasPorGiro
	default:
		return nil
	}
	m.enviar(protocolo.TipoRolar, protocolo.Rolar{Celula: foco.ID, Delta: delta})
	return nil
}

func (m *Modelo) View() tea.View {
	vista := tea.NewView("")
	vista.AltScreen = true
	vista.MouseMode = tea.MouseModeCellMotion
	if m.largura == 0 || m.altura == 0 {
		return vista
	}

	erro := m.erro
	if erro == "" {
		erro = m.estado.Aviso
	}

	modo := m.Modo()
	var fundo string
	if m.mostrandoMosaico() {
		fundo = Desenhar(m.estado, m.foco, modo, m.largura, m.altura, erro)
	} else {
		fundo = DesenharLista(m.estado, m.foco, modo, m.largura, m.altura, erro)
	}

	switch {
	case m.painel != nil:
		fundo = sobrepor(fundo, m.painel.Desenhar(m.largura), m.largura, m.altura)
	case m.formulario != nil:
		fundo = sobrepor(fundo, m.formulario.Desenhar(m.largura), m.largura, m.altura)
	case m.pergunta != nil:
		fundo = sobrepor(fundo, m.pergunta.Desenhar(m.largura), m.largura, m.altura)
	case m.confirmacao != nil:
		fundo = sobrepor(fundo, m.confirmacao.Desenhar(m.largura), m.largura, m.altura)
	case m.ajuda:
		fundo = sobrepor(fundo, caixaDeAjuda(m.largura, m.altura), m.largura, m.altura)
	case m.achados != nil:
		fundo = sobrepor(fundo, caixaDeAchados(*m.achados, m.achadoEscolhido, m.largura), m.largura, m.altura)
	}
	vista.SetContent(fundo)

	// O cursor só aparece quando o teclado é da célula e a leitura está ao
	// vivo — piscar cursor num histórico rolado seria mentira.
	if celula := m.celulaFocada(); m.digitando && celula != nil && celula.AoVivo && celula.CursorX >= 0 {
		if x, y, tem := m.origemDaCelula(celula.ID); tem {
			vista.Cursor = tea.NewCursor(x+celula.CursorX, y+celula.CursorY)
		}
	}
	return vista
}

func (m *Modelo) mostrandoMosaico() bool { return m.visao != visaoLista }

// avisarTamanhos conta ao motor a área reservada para cada célula visível, para
// os processos lá dentro enxergarem um terminal do tamanho certo.
func (m *Modelo) avisarTamanhos() {
	if m.largura == 0 || m.altura == 0 {
		return
	}
	for id, miolo := range m.miolosVisiveis() {
		if anterior, jaAvisado := m.tamanhos[id]; jaAvisado && anterior == miolo {
			continue
		}
		m.tamanhos[id] = miolo
		m.enviar(protocolo.TipoTamanho, protocolo.Tamanho{Celula: id, Colunas: miolo.Colunas, Linhas: miolo.Linhas})
	}
}

func (m *Modelo) miolosVisiveis() map[string]Geometria {
	if m.mostrandoMosaico() {
		return Dispor(m.estado, m.foco, m.largura, m.altura).Miolos()
	}
	celula := m.celulaFocada()
	if celula == nil {
		return nil
	}
	return map[string]Geometria{celula.ID: MioloDaPrevia(m.largura, m.altura)}
}

// origemDaCelula é onde o miolo da célula começa na tela, para o cursor cair no
// lugar certo.
func (m *Modelo) origemDaCelula(id string) (int, int, bool) {
	if !m.mostrandoMosaico() {
		return larguraDoIndice + 2, 4, true
	}
	if m.foco.Cheia {
		return 1, 2, true
	}
	return OrigemNoMosaico(m.estado, m.foco, m.largura, m.altura, id)
}

func (m *Modelo) enviar(tipo string, dados any) {
	if m.cliente == nil {
		return // tela sem motor do outro lado: só o desenho é exercitado
	}
	if err := m.cliente.Enviar(tipo, dados); err != nil {
		m.erro = "o motor não respondeu: " + err.Error()
	}
}

func (m *Modelo) focoAtual() (*protocolo.Projeto, *protocolo.Celula) {
	if m.foco.Projeto >= len(m.estado.Projetos) {
		return nil, nil
	}
	projeto := &m.estado.Projetos[m.foco.Projeto]
	if m.foco.Celula >= len(projeto.Celulas) {
		return projeto, nil
	}
	return projeto, &projeto.Celulas[m.foco.Celula]
}

func (m *Modelo) celulaFocada() *protocolo.Celula {
	_, celula := m.focoAtual()
	return celula
}

// confirmacaoDeMorte monta o aviso de matar, dizendo quando o projeto inteiro
// sai da tela junto.
func confirmacaoDeMorte(projeto protocolo.Projeto, celula protocolo.Celula) *Confirmacao {
	linhas := []string{"matar " + celula.Tipo + " · " + celula.Nome + "?"}
	if len(projeto.Celulas) == 1 {
		linhas = append(linhas, "",
			corRolagem.Render("é a última célula de "+strings.ToUpper(projeto.Nome)+","),
			corRolagem.Render("então o projeto sai da tela."),
			corApagada.Render("o diretório no disco não é tocado."))
	}
	return &Confirmacao{Titulo: "MATAR", Linhas: linhas, Acao: teclado.Matar, Alvo: celula.ID}
}

// abrirDocker abre o painel do projeto focado. Projeto sem compose não tem
// painel, e dizer isso é melhor do que abrir uma caixa vazia.
func (m *Modelo) abrirDocker() {
	projeto, _ := m.focoAtual()
	if projeto == nil {
		return
	}
	if !projeto.TemCompose {
		m.erro = "o projeto " + projeto.Nome + " não tem arquivo de compose na raiz"
		return
	}
	m.painel = NovoPainelDocker(projeto.ID, projeto.Nome)
	m.enviar(protocolo.TipoDocker, protocolo.Docker{Projeto: projeto.ID, Acao: "listar"})
}
