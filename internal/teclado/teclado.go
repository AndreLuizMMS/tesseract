// Package teclado é o único lugar onde tecla vira ação. Nenhum arquivo fora
// daqui associa tecla a significado — é essa a regra que impede a mesma tecla
// de fazer coisas diferentes dependendo da tela.
package teclado

import "strconv"

// Modo é quem é o dono do teclado agora. Nunca há dois donos ao mesmo tempo.
type Modo int

const (
	// Navegar: toda tecla é do aplicativo.
	Navegar Modo = iota
	// Digitar: toda tecla é da célula, sem nenhuma exceção além de ctrl-l.
	Digitar
	// Formulario: o teclado é do campo em foco; só as teclas de comando saem.
	Formulario
	// PainelDocker: enquanto o painel está aberto, o teclado é dele.
	PainelDocker
)

func (m Modo) String() string {
	switch m {
	case Digitar:
		return "DIGITAR"
	case Formulario:
		return "FORMULÁRIO"
	case PainelDocker:
		return "DOCKER"
	}
	return "NAVEGAR"
}

// Acao é o que uma tecla significa.
type Acao string

const (
	Nada Acao = ""

	// Andar — só setas, nenhuma letra.
	CelulaAnterior  Acao = "celula-anterior"
	CelulaProxima   Acao = "celula-proxima"
	ProjetoAnterior Acao = "projeto-anterior"
	ProjetoProximo  Acao = "projeto-proximo"
	PularChamado    Acao = "pular-chamado"
	IrParaProjeto   Acao = "ir-para-projeto"
	ProximaAba      Acao = "proxima-aba"
	AbaAnterior     Acao = "aba-anterior"

	// Teclado e tela.
	EntrarDigitar Acao = "entrar-digitar"
	SairDigitar   Acao = "sair-digitar"
	TelaCheia     Acao = "tela-cheia"
	TrocarTela    Acao = "trocar-tela"

	// Criar e matar.
	Criar   Acao = "criar"
	Retomar Acao = "retomar"
	Matar   Acao = "matar"

	// Nomear.
	Renomear Acao = "renomear"

	// Agir.
	Prompt      Acao = "prompt"
	AbrirDocker Acao = "abrir-docker"
	AbrirEditor Acao = "abrir-editor"
	BuscarTermo Acao = "buscar-termo"
	Voltar      Acao = "voltar"
	Ajuda       Acao = "ajuda"
	Fechar      Acao = "fechar"

	// Formulário.
	Confirmar     Acao = "confirmar"
	Cancelar      Acao = "cancelar"
	Completar     Acao = "completar"
	CampoAnterior Acao = "campo-anterior"
	CampoProximo  Acao = "campo-proximo"
	OpcaoAnterior Acao = "opcao-anterior"
	OpcaoProxima  Acao = "opcao-proxima"

	// Painel Docker. Maiúscula é sempre a versão da stack inteira.
	ServicoAnterior Acao = "servico-anterior"
	ServicoProximo  Acao = "servico-proximo"
	SobeServico     Acao = "sobe-servico"
	ParaServico     Acao = "para-servico"
	ReiniciaServico Acao = "reinicia-servico"
	RebuildaServico Acao = "rebuilda-servico"
	LogComoCelula   Acao = "log-como-celula"
	SobeStack       Acao = "sobe-stack"
	ParaStack       Acao = "para-stack"
	ReiniciaStack   Acao = "reinicia-stack"
)

// Atalho é uma linha do mapa: a tecla, o que ela faz e o texto da ajuda.
type Atalho struct {
	Tecla string
	Acao  Acao
	Ajuda string
	// Grupo junta na ajuda as teclas que são a mesma ideia (as setas, os
	// números). Vazio quando a tecla se explica sozinha.
	Grupo string
	// Rodape marca o punhado de teclas que cabem na barra de baixo, e Curto é
	// como elas aparecem lá.
	Rodape bool
	Curto  string
}

// mapa é o teclado inteiro. Uma tecla, um significado, em qualquer tela.
var mapa = map[Modo][]Atalho{
	Navegar: {
		// Navegação é exclusivamente direcional: letra nenhuma anda pela grade.
		{Tecla: "left", Acao: CelulaAnterior, Ajuda: "anda entre as células, atravessando projeto", Grupo: "←→ célula", Rodape: true, Curto: "←→ célula"},
		{Tecla: "right", Acao: CelulaProxima, Ajuda: "próxima célula", Grupo: "←→ célula"},
		{Tecla: "up", Acao: ProjetoAnterior, Ajuda: "anda entre os projetos", Grupo: "↑↓ projeto", Rodape: true, Curto: "↑↓ projeto"},
		{Tecla: "down", Acao: ProjetoProximo, Ajuda: "próximo projeto", Grupo: "↑↓ projeto"},
		{Tecla: "space", Acao: PularChamado, Ajuda: "pula para a próxima célula que pede atenção, atravessando projeto"},
		{Tecla: "tab", Acao: ProximaAba, Ajuda: "troca a aba da célula: claude, cursor, shell", Grupo: "tab ⇥ aba", Rodape: true, Curto: "tab aba"},
		{Tecla: "shift+tab", Acao: AbaAnterior, Ajuda: "aba anterior", Grupo: "tab ⇥ aba"},

		{Tecla: "enter", Acao: EntrarDigitar, Ajuda: "entra em DIGITAR na célula focada", Rodape: true, Curto: "↵ digitar"},
		{Tecla: "o", Acao: TelaCheia, Ajuda: "célula focada em tela cheia (liga e desliga)"},
		{Tecla: "v", Acao: TrocarTela, Ajuda: "alterna mosaico e lista", Rodape: true, Curto: "v lista"},

		{Tecla: "n", Acao: Criar, Ajuda: "criar — pede o projeto, depois a célula", Rodape: true, Curto: "n criar"},
		{Tecla: "r", Acao: Retomar, Ajuda: "retoma célula parada, ou sobe célula caída"},
		{Tecla: "D", Acao: Matar, Ajuda: "mata a célula focada — sempre confirma"},

		{Tecla: "ctrl+r", Acao: Renomear, Ajuda: "manda o agente escolher o nome da conversa e adota esse nome na célula"},

		{Tecla: "p", Acao: Prompt, Ajuda: "manda prompt para a célula focada sem entrar nela"},
		{Tecla: "d", Acao: AbrirDocker, Ajuda: "abre o painel Docker do projeto focado", Rodape: true, Curto: "d docker"},
		{Tecla: "ctrl+e", Acao: AbrirEditor, Ajuda: "abre o diretório do projeto na IDE configurada"},

		{Tecla: "/", Acao: BuscarTermo, Ajuda: "busca no histórico da célula focada"},
		{Tecla: "esc", Acao: Voltar, Ajuda: "sai da rolagem e fecha o que estiver aberto"},

		{Tecla: "?", Acao: Ajuda, Ajuda: "ajuda", Rodape: true, Curto: "? ajuda"},
		{Tecla: "q", Acao: Fechar, Ajuda: "fecha a tela — o motor continua rodando"},
	},
	Digitar: {
		// A única tecla reservada. Todo o resto vai para a célula.
		{Tecla: "ctrl+l", Acao: SairDigitar, Ajuda: "devolve o teclado ao aplicativo", Rodape: true, Curto: "ctrl-l devolve o teclado"},
	},
	Formulario: {
		{Tecla: "enter", Acao: Confirmar, Ajuda: "confirma", Rodape: true, Curto: "↵ confirma"},
		{Tecla: "esc", Acao: Cancelar, Ajuda: "cancela", Rodape: true, Curto: "esc cancela"},
		{Tecla: "tab", Acao: Completar, Ajuda: "completa o caminho"},
		{Tecla: "up", Acao: CampoAnterior, Ajuda: "anda entre os campos", Grupo: "↑↓ campo"},
		{Tecla: "down", Acao: CampoProximo, Ajuda: "próximo campo", Grupo: "↑↓ campo"},
		{Tecla: "left", Acao: OpcaoAnterior, Ajuda: "anda entre as opções do campo", Grupo: "←→ opção"},
		{Tecla: "right", Acao: OpcaoProxima, Ajuda: "próxima opção", Grupo: "←→ opção"},
	},
	PainelDocker: {
		{Tecla: "up", Acao: ServicoAnterior, Ajuda: "escolhe o serviço — escolher não executa nada", Grupo: "↑↓ escolhe serviço", Rodape: true, Curto: "↑↓ serviço"},
		{Tecla: "down", Acao: ServicoProximo, Ajuda: "próximo serviço", Grupo: "↑↓ escolhe serviço"},
		{Tecla: "u", Acao: SobeServico, Ajuda: "sobe o serviço escolhido", Rodape: true, Curto: "u sobe"},
		{Tecla: "s", Acao: ParaServico, Ajuda: "para o serviço escolhido", Rodape: true, Curto: "s para"},
		{Tecla: "r", Acao: ReiniciaServico, Ajuda: "reinicia o serviço escolhido", Rodape: true, Curto: "r reinicia"},
		{Tecla: "b", Acao: RebuildaServico, Ajuda: "rebuilda o serviço escolhido"},
		{Tecla: "l", Acao: LogComoCelula, Ajuda: "abre o log do serviço como célula", Rodape: true, Curto: "l log vira célula"},
		{Tecla: "U", Acao: SobeStack, Ajuda: "sobe a stack inteira"},
		{Tecla: "S", Acao: ParaStack, Ajuda: "para a stack inteira"},
		{Tecla: "R", Acao: ReiniciaStack, Ajuda: "reinicia a stack inteira"},
		{Tecla: "esc", Acao: Voltar, Ajuda: "fecha o painel e devolve o teclado", Rodape: true, Curto: "esc fecha"},
	},
}

func init() {
	// Os números levam direto ao projeto N. Ficam no mapa como qualquer outra
	// tecla, agrupados numa linha só na ajuda.
	for n := 1; n <= 9; n++ {
		mapa[Navegar] = append(mapa[Navegar], Atalho{
			Tecla: strconv.Itoa(n),
			Acao:  IrParaProjeto,
			Ajuda: "vai direto para o projeto N",
			Grupo: "1…9 projeto N",
		})
	}
}

// Buscar traduz uma tecla no modo atual. O que não está no mapa não significa
// nada — em DIGITAR e no formulário, vai inteiro para quem tem o teclado.
func Buscar(modo Modo, tecla string) Acao {
	for _, atalho := range mapa[modo] {
		if atalho.Tecla == tecla {
			return atalho.Acao
		}
	}
	return Nada
}

// Atalhos é o mapa do modo, para a ajuda e o rodapé desenharem.
func Atalhos(modo Modo) []Atalho {
	copia := make([]Atalho, len(mapa[modo]))
	copy(copia, mapa[modo])
	return copia
}

// Modos lista todos os modos que têm teclado próprio.
func Modos() []Modo {
	return []Modo{Navegar, Digitar, Formulario, PainelDocker}
}

// Linha é como uma tecla aparece na ajuda: já agrupada quando faz parte de uma
// família.
type Linha struct {
	Teclas string
	Ajuda  string
}

// Ajudas monta a ajuda de um modo, com uma linha por ideia.
func Ajudas(modo Modo) []Linha {
	var linhas []Linha
	vistos := map[string]bool{}
	for _, atalho := range mapa[modo] {
		if atalho.Grupo != "" {
			if vistos[atalho.Grupo] {
				continue
			}
			vistos[atalho.Grupo] = true
			linhas = append(linhas, Linha{Teclas: atalho.Grupo, Ajuda: atalho.Ajuda})
			continue
		}
		linhas = append(linhas, Linha{Teclas: mostrar(atalho.Tecla), Ajuda: atalho.Ajuda})
	}
	return linhas
}

// mostrar troca o nome interno da tecla pelo símbolo que o usuário vê.
func mostrar(tecla string) string {
	switch tecla {
	case "enter":
		return "↵"
	case "space":
		return "espaço"
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "left":
		return "←"
	case "right":
		return "→"
	}
	return tecla
}
