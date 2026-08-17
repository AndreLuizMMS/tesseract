package tela

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
	"github.com/andreluiz/tesseract/internal/tema"
)

// Pergunta é uma caixa de uma linha só: renomear, mandar prompt, buscar. O
// teclado dela é o do formulário, então enter confirma e esc cancela.
type Pergunta struct {
	Titulo string
	Rotulo string
	Dica   string
	Texto  string
	// Acao lembra quem abriu a pergunta, para o modelo saber o que fazer com a
	// resposta.
	Acao teclado.Acao
}

// Tecla trata uma tecla da pergunta e diz se ela continua aberta e se foi
// confirmada.
func (p *Pergunta) Tecla(acao teclado.Acao, texto string, apagou bool) (confirmou, aberta bool) {
	switch acao {
	case teclado.Cancelar:
		return false, false
	case teclado.Confirmar:
		return true, false
	}
	if apagou {
		if runas := []rune(p.Texto); len(runas) > 0 {
			p.Texto = string(runas[:len(runas)-1])
		}
		return false, true
	}
	p.Texto += texto
	return false, true
}

func (p *Pergunta) Desenhar(largura int) []string {
	corpo := []string{"  " + corTitulo.Render(preencher(p.Rotulo, 8)) + p.Texto + "▏"}
	if p.Dica != "" {
		corpo = append(corpo, "  "+strings.Repeat(" ", 8)+corApagada.Render(p.Dica))
	}
	corpo = append(corpo, "", "  "+corApagada.Render("↵ confirma        esc cancela"))
	return caixaFlutuante(p.Titulo, corpo, min(largura-8, 60))
}

// Confirmacao é o sim ou não de uma ação que não dá para desfazer.
type Confirmacao struct {
	Titulo string
	Linhas []string
	Acao   teclado.Acao
	Alvo   string
}

func (c *Confirmacao) Desenhar(largura int) []string {
	var corpo []string
	for _, linha := range c.Linhas {
		corpo = append(corpo, "  "+linha)
	}
	corpo = append(corpo, "", "  "+corApagada.Render("↵ confirma        esc cancela"))
	return caixaFlutuante(c.Titulo, corpo, min(largura-8, 60))
}

// caixaDeAjuda lista todas as teclas, separadas por modo. Nenhuma tecla que não
// exista aparece aqui, porque a lista sai do próprio mapa.
func caixaDeAjuda(largura, altura int) []string {
	var corpo []string
	for _, modo := range teclado.Modos() {
		if len(corpo) > 0 {
			corpo = append(corpo, "")
		}
		corpo = append(corpo, "  "+corSelo.Render(" "+modo.String()+" "))
		for _, linha := range teclado.Ajudas(modo) {
			corpo = append(corpo, "  "+corBarra.Render(preencher(linha.Teclas, 16))+corApagada.Render(linha.Ajuda))
		}
	}
	if sobra := altura - 6; sobra > 0 && len(corpo) > sobra {
		corpo = corpo[:sobra]
		corpo = append(corpo, "  "+corApagada.Render("…"))
	}
	return caixaFlutuante(tema.Glifo+" AJUDA", corpo, min(largura-8, 76))
}

// caixaDeAchados mostra o resultado da busca no histórico da célula.
func caixaDeAchados(achados protocolo.Achados, escolhido, largura int) []string {
	var corpo []string
	if len(achados.Linhas) == 0 {
		corpo = append(corpo, "  "+corApagada.Render("nada encontrado para "+strconv.Quote(achados.Termo)))
	}
	for i, achado := range achados.Linhas {
		if i >= 12 {
			corpo = append(corpo, "  "+corApagada.Render("… e mais "+strconv.Itoa(len(achados.Linhas)-12)))
			break
		}
		marca := "  "
		if i == escolhido {
			marca = "▸ "
		}
		corpo = append(corpo, marca+corApagada.Render(preencher(strconv.Itoa(achado.Linha), 7))+cortar(achado.Texto, largura-24))
	}
	corpo = append(corpo, "", "  "+corApagada.Render("↑↓ escolhe   ↵ vai até lá   esc fecha"))
	return caixaFlutuante("BUSCA · "+achados.Termo, corpo, min(largura-8, 76))
}

// sobrepor põe uma caixa por cima do fundo, centralizada na vertical. As linhas
// da faixa da caixa são substituídas inteiras — modal é modal.
func sobrepor(fundo string, caixa []string, largura, altura int) string {
	linhas := strings.Split(fundo, "\n")
	for len(linhas) < altura {
		linhas = append(linhas, strings.Repeat(" ", largura))
	}
	topo := max((altura-len(caixa))/2, 0)
	margem := max((largura-larguraVisivel(caixa))/2, 0)
	for i, linha := range caixa {
		if topo+i >= len(linhas) {
			break
		}
		linhas[topo+i] = preencher(strings.Repeat(" ", margem)+linha, largura)
	}
	return strings.Join(linhas, "\n")
}

func larguraVisivel(caixa []string) int {
	maior := 0
	for _, linha := range caixa {
		if largura := lipgloss.Width(linha); largura > maior {
			maior = largura
		}
	}
	return maior
}
