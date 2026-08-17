package tela

import (
	"strconv"
	"strings"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

// campo é uma linha do formulário: um rótulo e o que já foi digitado.
type campo struct {
	rotulo string
	valor  string
	dica   string
	// opcoes preenchido faz o campo virar um seletor em vez de texto.
	opcoes    []string
	escolhida int
	completa  bool // aceita tab para completar caminho
	soPasta   bool
}

func (c *campo) escrever(texto string) {
	if len(c.opcoes) > 0 {
		return
	}
	c.valor += texto
}

func (c *campo) apagar() {
	if len(c.opcoes) > 0 || c.valor == "" {
		return
	}
	runas := []rune(c.valor)
	c.valor = string(runas[:len(runas)-1])
}

func (c *campo) andar(passo int) {
	if len(c.opcoes) == 0 {
		return
	}
	c.escolhida = (c.escolhida + passo + len(c.opcoes)) % len(c.opcoes)
}

func (c *campo) escolhido() string {
	if len(c.opcoes) == 0 {
		return c.valor
	}
	return c.opcoes[c.escolhida]
}

// Formulario é o único caminho de criação: começa perguntando o projeto e
// termina criando a célula. Não existe "criar projeto" separado — e não existe
// escolher o que a sessão vai ser: ela nasce com as abas dos agentes dentro.
type Formulario struct {
	campos []*campo
	atual  int
	aviso  string
}

// NovoFormulario abre a criação com o caminho da casa preenchido, que é de onde
// todo projeto sai.
func NovoFormulario(_ []protocolo.TipoCelula, _ string) *Formulario {
	f := &Formulario{}
	f.campos = []*campo{
		{rotulo: "PROJETO", valor: casaComBarra(), dica: "tab completa · caminho novo cria projeto", completa: true, soPasta: true},
		{rotulo: "NOME", dica: "(vazio → nome automático)"},
		{rotulo: "MD", dica: "(opcional — arquivo markdown vira célula de leitura)", completa: true},
		{rotulo: "PROMPT", dica: "(opcional — sobe já trabalhando)"},
	}
	return f
}

// casaComBarra é o diretório da conta, terminado em barra para o tab completar
// já de dentro dele.
func casaComBarra() string {
	casa := lar()
	if casa == "" {
		return ""
	}
	return strings.TrimSuffix(casa, "/") + "/"
}

// Tecla trata uma tecla do formulário. Devolve o pedido de criação quando o
// usuário confirma, e se o formulário deve continuar aberto.
func (f *Formulario) Tecla(acao teclado.Acao, texto string, apagou bool) (pedido *protocolo.Criar, aberto bool) {
	f.aviso = ""
	switch acao {
	case teclado.Cancelar:
		return nil, false
	case teclado.Confirmar:
		caminho := strings.TrimSpace(f.campos[0].valor)
		if caminho == "" {
			f.aviso = "o projeto precisa de um caminho"
			return nil, true
		}
		// O tipo não é perguntado: quem decide é o motor, pelo que foi
		// preenchido. Sem markdown, nasce uma sessão com as abas dos agentes.
		criar := &protocolo.Criar{
			Caminho: caminho,
			Nome:    strings.TrimSpace(f.valorDe("NOME")),
			Alvo:    strings.TrimSpace(f.valorDe("MD")),
			Prompt:  f.valorDe("PROMPT"),
		}
		return criar, false
	case teclado.CampoAnterior:
		f.atual = (f.atual - 1 + len(f.campos)) % len(f.campos)
	case teclado.CampoProximo:
		f.atual = (f.atual + 1) % len(f.campos)
	case teclado.OpcaoAnterior:
		f.campos[f.atual].andar(-1)
	case teclado.OpcaoProxima:
		f.campos[f.atual].andar(1)
	default:
		if apagou {
			f.campos[f.atual].apagar()
			return nil, true
		}
		f.campos[f.atual].escrever(texto)
	}
	return nil, true
}

// PedeCompletar diz se a tecla de completar tem o que fazer no campo atual.
func (f *Formulario) PedeCompletar(acao teclado.Acao) (protocolo.Completar, bool) {
	if acao != teclado.Completar {
		return protocolo.Completar{}, false
	}
	atual := f.campos[f.atual]
	if !atual.completa {
		return protocolo.Completar{}, false
	}
	caminho := atual.valor
	if !atual.soPasta && !strings.Contains(caminho, "/") {
		// O arquivo é procurado dentro do projeto que está sendo criado.
		caminho = strings.TrimSuffix(f.campos[0].valor, "/") + "/" + caminho
	}
	return protocolo.Completar{Caminho: caminho, SoDiretorio: atual.soPasta}, true
}

// Completou recebe a resposta do motor sobre o caminho digitado.
func (f *Formulario) Completou(resposta protocolo.Completado) {
	atual := f.campos[f.atual]
	if !atual.completa {
		return
	}
	atual.valor = resposta.Caminho
	switch resposta.Quantidade {
	case 0:
		f.aviso = "nada casa com isso"
	case 1:
		f.aviso = ""
	default:
		if atual.soPasta {
			f.aviso = plural(resposta.Quantidade, "pasta casa", "pastas casam")
			return
		}
		f.aviso = plural(resposta.Quantidade, "arquivo casa", "arquivos casam")
	}
}

func (f *Formulario) valorDe(rotulo string) string {
	if rotulo == "" {
		return ""
	}
	for _, c := range f.campos {
		if c.rotulo == rotulo {
			return c.valor
		}
	}
	return ""
}

// Desenhar monta a caixa do formulário.
func (f *Formulario) Desenhar(largura int) []string {
	interno := min(largura-8, 60)
	var corpo []string
	for i, c := range f.campos {
		marca := "  "
		if i == f.atual {
			marca = "▸ "
		}
		valor := c.valor + "▏"
		if len(c.opcoes) > 0 {
			var opcoes []string
			for j, opcao := range c.opcoes {
				if j == c.escolhida {
					opcoes = append(opcoes, corSelo.Render(" "+opcao+" "))
					continue
				}
				opcoes = append(opcoes, corApagada.Render(" "+opcao+" "))
			}
			valor = strings.Join(opcoes, " ")
		}
		corpo = append(corpo, marca+corTitulo.Render(preencher(c.rotulo, 8))+valor)
		if c.dica != "" {
			corpo = append(corpo, "  "+strings.Repeat(" ", 8)+corApagada.Render(c.dica))
		}
	}
	if f.aviso != "" {
		corpo = append(corpo, "", "  "+corRolagem.Render(f.aviso))
	}
	corpo = append(corpo, "", "  "+corApagada.Render("↵ criar        esc cancelar"))
	return caixaFlutuante("NOVA", corpo, interno)
}

// caixaFlutuante desenha uma caixa com título, do tamanho pedido.
func caixaFlutuante(titulo string, corpo []string, largura int) []string {
	linhas := []string{corBordaFocada.Render("┌ ") + corTitulo.Render(titulo) + corBordaFocada.Render(" "+strings.Repeat("─", max(largura-len([]rune(titulo))-4, 0))+"┐")}
	linhas = append(linhas, corBordaFocada.Render("│")+preencher("", largura-2)+corBordaFocada.Render("│"))
	for _, linha := range corpo {
		linhas = append(linhas, corBordaFocada.Render("│")+preencher(linha, largura-2)+corBordaFocada.Render("│"))
	}
	linhas = append(linhas, corBordaFocada.Render("│")+preencher("", largura-2)+corBordaFocada.Render("│"))
	linhas = append(linhas, corBordaFocada.Render("└"+strings.Repeat("─", largura-2)+"┘"))
	return linhas
}

func plural(quantos int, um, varios string) string {
	if quantos == 1 {
		return strconv.Itoa(quantos) + " " + um
	}
	return strconv.Itoa(quantos) + " " + varios
}
