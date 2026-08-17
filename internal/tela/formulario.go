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
// termina criando a célula. Não existe "criar projeto" separado.
type Formulario struct {
	tipos  []protocolo.TipoCelula
	campos []*campo
	atual  int
	aviso  string
}

// NovoFormulario abre a criação já com o projeto focado preenchido.
func NovoFormulario(tipos []protocolo.TipoCelula, caminhoFocado string) *Formulario {
	nomes := make([]string, 0, len(tipos))
	for _, tipo := range tipos {
		nomes = append(nomes, tipo.Tipo)
	}
	f := &Formulario{tipos: tipos}
	f.campos = []*campo{
		{rotulo: "PROJETO", valor: caminhoFocado, dica: "tab completa · caminho novo cria projeto", completa: true, soPasta: true},
		{rotulo: "TIPO", opcoes: nomes},
		{rotulo: "NOME", dica: "(vazio → nome automático)"},
	}
	f.ajustarCamposDoTipo()
	return f
}

// ajustarCamposDoTipo põe e tira os campos que dependem do tipo escolhido. O
// formulário não sabe o que cada tipo é: ele lê a ficha que o motor mandou.
func (f *Formulario) ajustarCamposDoTipo() {
	base := f.campos[:min(3, len(f.campos))]
	ficha := f.ficha()
	campos := append([]*campo{}, base...)
	if ficha.RotuloAlvo != "" {
		campos = append(campos, &campo{
			rotulo:   ficha.RotuloAlvo,
			dica:     "tab completa",
			completa: ficha.CompletaArquivo,
		})
	}
	if ficha.AceitaPrompt {
		campos = append(campos, &campo{rotulo: "PROMPT", dica: "(opcional — sobe já trabalhando)"})
	}
	f.campos = campos
	f.atual = min(f.atual, len(f.campos)-1)
}

func (f *Formulario) ficha() protocolo.TipoCelula {
	escolhido := f.campos[1].escolhido()
	for _, tipo := range f.tipos {
		if tipo.Tipo == escolhido {
			return tipo
		}
	}
	return protocolo.TipoCelula{}
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
		criar := &protocolo.Criar{
			Caminho: caminho,
			Tipo:    f.campos[1].escolhido(),
			Nome:    strings.TrimSpace(f.valorDe("NOME")),
			Alvo:    strings.TrimSpace(f.valorDe(f.ficha().RotuloAlvo)),
			Prompt:  f.valorDe("PROMPT"),
		}
		return criar, false
	case teclado.CampoAnterior:
		f.atual = (f.atual - 1 + len(f.campos)) % len(f.campos)
	case teclado.CampoProximo:
		f.atual = (f.atual + 1) % len(f.campos)
	case teclado.OpcaoAnterior:
		f.campos[f.atual].andar(-1)
		if f.atual == 1 {
			f.ajustarCamposDoTipo()
		}
	case teclado.OpcaoProxima:
		f.campos[f.atual].andar(1)
		if f.atual == 1 {
			f.ajustarCamposDoTipo()
		}
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
	return protocolo.Completar{Caminho: atual.valor, SoDiretorio: atual.soPasta}, true
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
		f.aviso = plural(resposta.Quantidade, "pasta casa", "pastas casam")
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
