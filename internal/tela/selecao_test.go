package tela

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// linhasComCor é o que a célula manda de verdade: texto já estilizado pelo
// programa lá dentro. Copiar não pode levar os códigos de cor junto.
var linhasComCor = []string{
	"\x1b[32mMovi a validação\x1b[0m de token",
	"pro guard.",
	"Qual você prefere?",
}

// TestSelecaoCopiaSoAsLetras — o que vai para a área de transferência é o
// texto, sem cor e sem os espaços do fim da linha.
func TestSelecaoCopiaSoAsLetras(t *testing.T) {
	s := Selecao{Celula: "c1", AncoraX: 5, AncoraY: 0, AtualX: 9, AtualY: 2}

	texto := s.Texto(linhasComCor, 40)
	esperado := "a validação de token\npro guard.\nQual você"
	if texto != esperado {
		t.Fatalf("o trecho copiado veio %q, esperado %q", texto, esperado)
	}
	if strings.Contains(texto, "\x1b") {
		t.Errorf("o trecho copiado levou código de cor junto: %q", texto)
	}
}

// TestSelecaoValeNosDoisSentidos — arrastar de baixo para cima marca o mesmo
// trecho de arrastar de cima para baixo.
func TestSelecaoValeNosDoisSentidos(t *testing.T) {
	descendo := Selecao{AncoraX: 5, AncoraY: 0, AtualX: 9, AtualY: 2}
	subindo := Selecao{AncoraX: 9, AncoraY: 2, AtualX: 5, AtualY: 0}
	if descendo.Texto(linhasComCor, 40) != subindo.Texto(linhasComCor, 40) {
		t.Fatalf("o sentido do arrasto mudou o trecho:\n%q\n%q",
			descendo.Texto(linhasComCor, 40), subindo.Texto(linhasComCor, 40))
	}
}

// TestSelecaoNumaLinhaSo — a marca de uma linha só vai da âncora até o ponto,
// inclusive a letra debaixo do cursor.
func TestSelecaoNumaLinhaSo(t *testing.T) {
	s := Selecao{AncoraX: 0, AncoraY: 1, AtualX: 2, AtualY: 1}
	if texto := s.Texto(linhasComCor, 40); texto != "pro" {
		t.Fatalf("o trecho de uma linha veio %q, esperado %q", texto, "pro")
	}
}

// TestPintarCabeNoMiolo — a marca acende até a margem, como no terminal, mas
// nunca passa da largura da célula, ou a caixa entorta.
func TestPintarCabeNoMiolo(t *testing.T) {
	s := Selecao{AncoraX: 5, AncoraY: 0, AtualX: 9, AtualY: 2}
	pintadas := s.Pintar(linhasComCor, 40)

	for i, linha := range pintadas {
		if largura := lipgloss.Width(linha); largura > 40 {
			t.Errorf("a linha %d ficou com %d colunas, e o miolo tem 40", i, largura)
		}
	}
	if !strings.Contains(pintadas[1], aberturaDaMarca()+"pro guard.") {
		t.Errorf("a linha do meio devia estar inteira em destaque: %q", pintadas[1])
	}
	if inicio := lipgloss.Width(ansiAteAMarca(pintadas[0])); inicio != 5 {
		t.Errorf("a marca da primeira linha devia começar na coluna 5, começou na %d", inicio)
	}
	if strings.Contains(pintadas[2], aberturaDaMarca()+"Qual você prefere?") {
		t.Error("a última linha devia parar no ponto onde o mouse soltou")
	}
}

// aberturaDaMarca é o código que acende o destaque, sem o texto nem o que
// apaga depois.
func aberturaDaMarca() string {
	return strings.SplitN(corSelecao.Render("marca"), "marca", 2)[0]
}

// ansiAteAMarca é o pedaço da linha antes do destaque começar.
func ansiAteAMarca(linha string) string {
	if corte := strings.Index(linha, aberturaDaMarca()); corte >= 0 {
		return linha[:corte]
	}
	return linha
}

// TestArrastarNaCelulaCopia é o caminho inteiro: o botão desce numa célula que
// nem estava focada, o mouse arrasta, e ao soltar o trecho vai para a área de
// transferência do terminal.
func TestArrastarNaCelulaCopia(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())
	area, tem := m.areasVisiveis()["c2"]
	if !tem {
		t.Fatal("a célula c2 devia estar à vista no mosaico")
	}

	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: area[0] + 2, Y: area[1]})
	m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: area[0] + 8, Y: area[1]})
	_, cmd := m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: area[0] + 8, Y: area[1]})

	if celula := m.celulaFocada(); celula == nil || celula.ID != "c2" {
		t.Fatalf("clicar numa célula também é escolhê-la, foco em %#v", m.foco)
	}
	if cmd == nil {
		t.Fatal("soltar o botão devia mandar o trecho para a área de transferência")
	}
	if copiado := fmt.Sprint(cmd()); copiado != "go test" {
		t.Fatalf("o trecho copiado veio %q, esperado %q", copiado, "go test")
	}
}

// TestEscApagaAMarca — a marca fica acesa depois de copiar, como recibo, e sai
// com esc como tudo o mais.
func TestEscApagaAMarca(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())
	area := m.areasVisiveis()["c1"]

	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: area[0], Y: area[1]})
	m.Update(tea.MouseMotionMsg{Button: tea.MouseLeft, X: area[0] + 4, Y: area[1]})
	m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: area[0] + 4, Y: area[1]})

	if m.selecao == nil {
		t.Fatal("a marca devia continuar acesa depois de copiar")
	}
	m.telaNavegando("esc")
	if m.selecao != nil {
		t.Fatal("esc devia apagar a marca")
	}
}

// TestCliqueSoEscolheACelula — descer e subir o botão no mesmo lugar é clique.
// Ele muda o foco e não encosta na área de transferência, senão escolher uma
// célula jogaria fora o que o usuário tinha copiado antes.
func TestCliqueSoEscolheACelula(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())
	area := m.areasVisiveis()["c3"]

	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: area[0] + 3, Y: area[1]})
	_, cmd := m.Update(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: area[0] + 3, Y: area[1]})

	if celula := m.celulaFocada(); celula == nil || celula.ID != "c3" {
		t.Fatalf("o clique devia escolher a célula c3, foco em %#v", m.foco)
	}
	if cmd != nil {
		t.Fatal("clique sem arrasto não copia nada")
	}
	if m.selecao != nil {
		t.Fatal("clique sem arrasto não deixa marca acesa")
	}
}

// TestArrastarEmDigitarNaoTrocaOTeclado — em DIGITAR o dono do teclado não muda
// por causa de um clique; a marca só vale dentro da célula que já tem o teclado.
func TestArrastarEmDigitarNaoTrocaOTeclado(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())
	m.digitando = true
	area := m.areasVisiveis()["c2"]

	m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: area[0] + 2, Y: area[1]})

	if celula := m.celulaFocada(); celula == nil || celula.ID != "c1" {
		t.Fatalf("o teclado devia continuar com c1, foco em %#v", m.foco)
	}
	if m.selecao != nil {
		t.Fatal("em DIGITAR a marca não começa numa célula que não tem o teclado")
	}
}
