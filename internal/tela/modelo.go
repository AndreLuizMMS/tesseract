package tela

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

// linhasPorGiro é quanto a roda do mouse anda de cada vez.
const linhasPorGiro = 3

// cadenciaDoGiro é de quanto em quanto tempo o desenho de trabalho em andamento
// anda um quadro.
const cadenciaDoGiro = 120 * time.Millisecond

// giroPorConferida é de quantos quadros em quantos quadros a tela pergunta ao
// motor como está a stack, enquanto o Docker trabalha. Assim os serviços vão
// ficando verdes na tela, em vez de tudo mudar de uma vez no fim.
const giroPorConferida = 10

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

// tiqueDoPainel move o desenho de trabalho em andamento do painel Docker.
type tiqueDoPainel struct{ contagem int }

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
	selecao   *Selecao

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

	case tiqueDoPainel:
		return m, m.girarOPainel(msg)

	case tea.MouseWheelMsg:
		return m, m.rolar(msg)

	case tea.MouseClickMsg:
		m.comecarSelecao(msg.Button, msg.X, msg.Y)

	case tea.MouseMotionMsg:
		m.arrastarSelecao(msg.X, msg.Y)

	case tea.MouseReleaseMsg:
		return m, m.copiarSelecao()

	case tea.KeyPressMsg:
		return m.teclou(msg)

	case tea.PasteMsg:
		return m, m.colou(msg.Content)
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

// colou entrega o texto colado a quem tem o teclado agora. Colar não chega como
// tecla — o terminal manda o conteúdo inteiro de uma vez —, então não passa por
// teclou; mas obedece ao mesmo dono, que continua sendo um só.
func (m *Modelo) colou(texto string) tea.Cmd {
	if texto == "" {
		return nil
	}
	switch {
	case m.formulario != nil:
		return m.telaDoFormulario("", umaLinha(texto), false)
	case m.pergunta != nil:
		return m.telaDaPergunta("", umaLinha(texto), false)
	case m.digitando:
		foco := m.celulaFocada()
		if foco == nil {
			return nil
		}
		m.enviar(protocolo.TipoTecla, protocolo.Tecla{Celula: foco.ID, Colar: texto})
	}
	return nil
}

// umaLinha achata o texto colado para caber num campo que só tem uma linha.
func umaLinha(texto string) string {
	return strings.Join(strings.Fields(texto), " ")
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
		m.andarPelasCelulas(-1)
	case teclado.CelulaProxima:
		m.andarPelasCelulas(1)
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
	case teclado.ProximaAba:
		if celula != nil {
			m.enviar(protocolo.TipoAba, protocolo.Aba{Celula: celula.ID, Passo: 1})
		}
	case teclado.AbaAnterior:
		if celula != nil {
			m.enviar(protocolo.TipoAba, protocolo.Aba{Celula: celula.ID, Passo: -1})
		}

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
			m.enviar(protocolo.TipoRenomear, protocolo.Renomear{Celula: celula.ID})
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
		if m.selecao != nil {
			m.selecao = nil
			break
		}
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
	if pedido == nil {
		if !aberto {
			m.painel = nil
		}
		return nil
	}

	m.enviar(protocolo.TipoDocker, *pedido)
	if !aberto {
		m.painel = nil
		return nil
	}
	// Ação de verdade: o painel passa a dizer que está trabalhando, e a tela
	// volta a desenhar sozinha enquanto isso.
	if pedido.Acao != "listar" {
		m.painel.Comecou(pedido.Acao, pedido.Servico)
		return tiqueDaqui(0)
	}
	return nil
}

// girarOPainel anda o desenho de trabalho e, de tempos em tempos, pergunta ao
// motor como a stack está indo.
func (m *Modelo) girarOPainel(tique tiqueDoPainel) tea.Cmd {
	if m.painel == nil || !m.painel.EmTrabalho() {
		return nil
	}
	m.painel.Girar()
	if tique.contagem%giroPorConferida == giroPorConferida-1 {
		m.enviar(protocolo.TipoDocker, protocolo.Docker{Projeto: m.painel.Projeto, Acao: "listar"})
	}
	return tiqueDaqui(tique.contagem + 1)
}

func tiqueDaqui(contagem int) tea.Cmd {
	return tea.Tick(cadenciaDoGiro, func(time.Time) tea.Msg { return tiqueDoPainel{contagem: contagem} })
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

// andarPelasCelulas caminha pela grade inteira, atravessando projeto: no
// mosaico todas as células estão à vista, então andar entre elas não para na
// fronteira do projeto.
func (m *Modelo) andarPelasCelulas(passo int) {
	total := m.totalDeCelulas()
	if total == 0 {
		return
	}
	m.foco.Projeto, m.foco.Celula = m.posicaoDe((m.posicaoLinear() + passo + total) % total)
}

func (m *Modelo) totalDeCelulas() int {
	total := 0
	for _, projeto := range m.estado.Projetos {
		total += len(projeto.Celulas)
	}
	return total
}

// pularParaQuemChamou vai para a próxima célula que pede atenção, atravessando
// projeto.
func (m *Modelo) pularParaQuemChamou() {
	chama := func(estado string) bool { return estado == "respondeu" || estado == "aprovar" }
	total := m.totalDeCelulas()
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
	if m.selecao != nil {
		// Rolar no meio do arrasto moveria o texto debaixo da marca. Depois de
		// copiar, rolar apaga a marca: ela ficaria acesa em cima de outro texto.
		if m.selecao.Arrastando {
			return nil
		}
		m.selecao = nil
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

// comecarSelecao ancora a marca onde o botão desceu. Clicar numa célula também
// é escolhê-la — menos em DIGITAR, onde trocar o dono do teclado sem pedir
// seria surpresa.
func (m *Modelo) comecarSelecao(botao tea.MouseButton, x, y int) {
	if botao != tea.MouseLeft {
		return
	}
	m.selecao = nil
	id, cx, cy, achou := m.celulaSobOPonto(x, y)
	if !achou {
		return
	}
	if !m.digitando {
		m.focarCelula(id)
	} else if foco := m.celulaFocada(); foco == nil || foco.ID != id {
		return
	}
	m.selecao = &Selecao{Celula: id, AncoraX: cx, AncoraY: cy, AtualX: cx, AtualY: cy, Arrastando: true}
}

// arrastarSelecao estica a marca até onde o mouse está, sem deixar ela sair da
// célula onde começou.
func (m *Modelo) arrastarSelecao(x, y int) {
	if m.selecao == nil || !m.selecao.Arrastando {
		return
	}
	area, tem := m.areasVisiveis()[m.selecao.Celula]
	if !tem {
		return
	}
	m.selecao.AtualX = min(max(x-area[0], 0), area[2]-1)
	m.selecao.AtualY = min(max(y-area[1], 0), area[3]-1)
}

// copiarSelecao fecha o arrasto e manda o trecho para a área de transferência
// do terminal. A marca continua acesa: ela é o recibo do que foi copiado.
func (m *Modelo) copiarSelecao() tea.Cmd {
	if m.selecao == nil {
		return nil
	}
	m.selecao.Arrastando = false
	if m.selecao.Vazia() {
		// Descer e subir no mesmo lugar é clique, não marca: escolher a célula
		// não pode trocar o que o usuário tinha copiado antes.
		m.selecao = nil
		return nil
	}
	texto := m.textoSelecionado()
	if strings.TrimSpace(texto) == "" {
		m.selecao = nil
		return nil
	}
	return tea.SetClipboard(texto)
}

// textoSelecionado tira do retrato o que está debaixo da marca.
func (m *Modelo) textoSelecionado() string {
	if m.selecao == nil {
		return ""
	}
	miolo, tem := m.miolosVisiveis()[m.selecao.Celula]
	if !tem {
		return ""
	}
	celula := m.celulaPorID(m.selecao.Celula)
	if celula == nil {
		return ""
	}
	return m.selecao.Texto(celula.Linhas, miolo.Colunas)
}

// areasVisiveis é onde cada célula à vista está na tela — origem e tamanho do
// miolo. É o que o mouse precisa para saber em quem ele tocou.
func (m *Modelo) areasVisiveis() map[string][4]int {
	areas := map[string][4]int{}
	for id, miolo := range m.miolosVisiveis() {
		if x, y, tem := m.origemDaCelula(id); tem {
			areas[id] = [4]int{x, y, miolo.Colunas, miolo.Linhas}
		}
	}
	return areas
}

// celulaSobOPonto diz qual célula está debaixo do ponto da tela, e onde o ponto
// cai dentro do miolo dela.
func (m *Modelo) celulaSobOPonto(x, y int) (id string, cx, cy int, achou bool) {
	for celula, area := range m.areasVisiveis() {
		if x >= area[0] && x < area[0]+area[2] && y >= area[1] && y < area[1]+area[3] {
			return celula, x - area[0], y - area[1], true
		}
	}
	return "", 0, 0, false
}

func (m *Modelo) celulaPorID(id string) *protocolo.Celula {
	for i, projeto := range m.estado.Projetos {
		for j, celula := range projeto.Celulas {
			if celula.ID == id {
				return &m.estado.Projetos[i].Celulas[j]
			}
		}
	}
	return nil
}

func (m *Modelo) focarCelula(id string) {
	for i, projeto := range m.estado.Projetos {
		for j, celula := range projeto.Celulas {
			if celula.ID == id {
				m.foco.Projeto, m.foco.Celula = i, j
				return
			}
		}
	}
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
	estado := m.estado
	if m.selecao != nil {
		if miolo, tem := m.miolosVisiveis()[m.selecao.Celula]; tem {
			estado = comSelecao(estado, *m.selecao, miolo.Colunas)
		}
	}

	var fundo string
	if m.mostrandoMosaico() {
		fundo = Desenhar(estado, m.foco, modo, m.largura, m.altura, erro)
	} else {
		fundo = DesenharLista(estado, m.foco, modo, m.largura, m.altura, erro)
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
