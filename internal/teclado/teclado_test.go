package teclado

import (
	"strings"
	"testing"
)

// TestUmaTeclaUmSignificado é a garantia mecânica da regra: se este teste
// quebrar, alguém deu dois sentidos à mesma tecla no mesmo modo — que foi
// exatamente o que apodreceu o fork.
func TestUmaTeclaUmSignificado(t *testing.T) {
	for _, modo := range Modos() {
		vistas := map[string]Acao{}
		for _, atalho := range Atalhos(modo) {
			if anterior, repetida := vistas[atalho.Tecla]; repetida {
				t.Errorf("modo %s: a tecla %q significa %q e também %q", modo, atalho.Tecla, anterior, atalho.Acao)
			}
			vistas[atalho.Tecla] = atalho.Acao
		}
	}
}

// TestDigitarSoReservaCtrlL — em DIGITAR toda tecla é da célula, sem nenhuma
// exceção além da que devolve o teclado.
func TestDigitarSoReservaCtrlL(t *testing.T) {
	atalhos := Atalhos(Digitar)
	if len(atalhos) != 1 {
		t.Fatalf("DIGITAR reserva %d teclas, e só pode reservar uma: %#v", len(atalhos), atalhos)
	}
	if atalhos[0].Tecla != "ctrl+l" || atalhos[0].Acao != SairDigitar {
		t.Fatalf("a única tecla reservada em DIGITAR tem que ser ctrl-l: %#v", atalhos[0])
	}
	for _, tecla := range []string{"q", "D", "tab", "esc", "enter", "up", "down", "left", "right", "/", "?"} {
		if acao := Buscar(Digitar, tecla); acao != Nada {
			t.Errorf("em DIGITAR a tecla %q tinha que ir para a célula, mas significa %q", tecla, acao)
		}
	}
}

// TestTodaTeclaTemAjuda — tecla sem ajuda é tecla que ninguém descobre.
func TestTodaTeclaTemAjuda(t *testing.T) {
	for _, modo := range Modos() {
		for _, atalho := range Atalhos(modo) {
			if atalho.Ajuda == "" {
				t.Errorf("modo %s: a tecla %q não tem texto de ajuda", modo, atalho.Tecla)
			}
			if atalho.Acao == Nada {
				t.Errorf("modo %s: a tecla %q não faz nada", modo, atalho.Tecla)
			}
		}
	}
}

// TestNenhumaLetraAndaPelaGrade — movimento é direcional e ponto. É de onde
// vinha a sensação de tecla imprevisível no fork.
func TestNenhumaLetraAndaPelaGrade(t *testing.T) {
	andar := map[Acao]bool{
		CelulaAnterior: true, CelulaProxima: true,
		ProjetoAnterior: true, ProjetoProximo: true,
	}
	setas := map[string]bool{"up": true, "down": true, "left": true, "right": true}
	for _, atalho := range Atalhos(Navegar) {
		if andar[atalho.Acao] && !setas[atalho.Tecla] {
			t.Errorf("a tecla %q anda pela grade sem ser seta", atalho.Tecla)
		}
	}
	for _, letra := range []string{"j", "k", "h", "l", "J", "K", "H", "L"} {
		if acao := Buscar(Navegar, letra); acao != Nada {
			t.Errorf("a letra %q não pode significar nada em NAVEGAR, mas significa %q", letra, acao)
		}
	}
}

// TestAjudaAgrupaAsFamilias — a ajuda mostra uma linha por ideia, não nove
// linhas iguais para os nove números.
func TestAjudaAgrupaAsFamilias(t *testing.T) {
	linhas := Ajudas(Navegar)
	contagem := map[string]int{}
	for _, linha := range linhas {
		contagem[linha.Teclas]++
		if linha.Ajuda == "" {
			t.Errorf("a linha de ajuda de %q está vazia", linha.Teclas)
		}
	}
	for teclas, quantas := range contagem {
		if quantas > 1 {
			t.Errorf("a ajuda repete a linha %q %d vezes", teclas, quantas)
		}
	}
	if contagem["1…9 projeto N"] != 1 {
		t.Error("os números deviam aparecer como uma linha só na ajuda")
	}
	if contagem["↑↓ célula"] != 1 || contagem["←→ projeto"] != 1 {
		t.Error("as setas deviam aparecer como uma linha por eixo")
	}
}

// TestPainelDockerTemTecladoProprio — o painel não empresta significado do mapa
// global sem declarar, e a maiúscula é sempre a versão da stack.
func TestPainelDockerTemTecladoProprio(t *testing.T) {
	pares := map[string]string{"u": "U", "s": "S", "r": "R"}
	daStack := map[Acao]bool{SobeStack: true, ParaStack: true, ReiniciaStack: true}
	for minuscula, maiuscula := range pares {
		if Buscar(PainelDocker, minuscula) == Nada {
			t.Errorf("o painel devia declarar a tecla %q", minuscula)
		}
		acao := Buscar(PainelDocker, maiuscula)
		if !daStack[acao] {
			t.Errorf("a maiúscula %q tinha que ser a versão da stack, mas é %q", maiuscula, acao)
		}
	}
	if Buscar(PainelDocker, "esc") != Voltar {
		t.Error("esc tem que devolver o teclado ao aplicativo")
	}
	// Nenhuma ação destrutiva mora aqui.
	for _, atalho := range Atalhos(PainelDocker) {
		for _, proibida := range []string{"apaga", "remove", "volume", "destr"} {
			if contem(atalho.Ajuda, proibida) {
				t.Errorf("o painel Docker não pode ter ação destrutiva: %q", atalho.Ajuda)
			}
		}
	}
}

func contem(texto, pedaco string) bool {
	return strings.Contains(texto, pedaco)
}

// TestRodapeTemTextoCurto — tecla que aparece no rodapé precisa de um rótulo
// que caiba nele.
func TestRodapeTemTextoCurto(t *testing.T) {
	for _, modo := range Modos() {
		for _, atalho := range Atalhos(modo) {
			if atalho.Rodape && atalho.Curto == "" {
				t.Errorf("modo %s: a tecla %q está no rodapé sem rótulo curto", modo, atalho.Tecla)
			}
		}
	}
}
