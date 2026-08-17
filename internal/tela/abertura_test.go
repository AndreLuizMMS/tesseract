package tela

import (
	"io"
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/tema"
)

// simbolo devolve só o desenho de um quadro, sem cor e sem a assinatura ao
// lado — é o que o olho vê da marca naquele instante.
func simbolo(a *Abertura, ponta *[2]int) []string {
	// O bloco é três espaços de recuo, os sete caracteres do símbolo e depois
	// a assinatura. O corte é por largura, não por separador: a última linha
	// do desenho começa com espaço e enganaria a busca por espaços.
	largura := len([]rune(tema.Simbolo[0]))
	linhas := a.linhasDoBloco(ponta)[:len(tema.Simbolo)]
	for i, linha := range linhas {
		desenho := []rune(strings.TrimPrefix(semEstilo(linha), "   "))
		if len(desenho) > largura {
			desenho = desenho[:largura]
		}
		linhas[i] = strings.TrimRight(string(desenho), " ")
	}
	return linhas
}

// TestAberturaComecaVaziaETerminaInteira — a marca não aparece pronta: ela se
// desenha. No primeiro quadro não há nada, no último há o símbolo canônico.
func TestAberturaComecaVaziaETerminaInteira(t *testing.T) {
	a := NovaAbertura(io.Discard, false)

	for i, linha := range simbolo(a, nil) {
		if strings.TrimSpace(linha) != "" {
			t.Fatalf("o quadro de partida devia estar vazio, e a linha %d tem %q", i, linha)
		}
	}

	a.acenderTudo()
	for i, linha := range simbolo(a, nil) {
		if esperado := strings.TrimRight(tema.Simbolo[i], " "); linha != esperado {
			t.Fatalf("linha %d do quadro final é %q, devia ser %q", i, linha, esperado)
		}
	}
}

// TestCaminhoDoTracoCobreOSimboloInteiro — o percurso da caneta não pode
// deixar buraco no desenho nem passar por onde não há traço.
func TestCaminhoDoTracoCobreOSimboloInteiro(t *testing.T) {
	a := NovaAbertura(io.Discard, false)
	for _, ponto := range caminhoDoTraco {
		if _, tem := tema.CorDoSimbolo(ponto[0], ponto[1]); !tem {
			t.Fatalf("o traço passa por %v, onde não há desenho", ponto)
		}
		a.aceso[ponto] = true
	}
	a.aceso[posicaoDaTessera] = true

	for i, linha := range tema.Simbolo {
		for j := range []rune(linha) {
			if _, tem := tema.CorDoSimbolo(i, j); tem && !a.aceso[[2]int{i, j}] {
				t.Fatalf("a posição %v nunca é desenhada pelo traço", [2]int{i, j})
			}
		}
	}
}

// TestPontaDaCanetaAcendeSoOndeEsta — é a ponta acesa que dá a sensação de
// caneta correndo; se ela pintasse o desenho todo, não haveria movimento.
func TestPontaDaCanetaAcendeSoOndeEsta(t *testing.T) {
	anterior := tema.Atual
	tema.Atual = tema.CorTotal
	defer func() { tema.Atual = anterior }()

	a := NovaAbertura(io.Discard, false)
	a.acenderTudo()

	ponta := [2]int{0, 0}
	comPonta := a.linhasDoBloco(&ponta)[0]
	semPonta := a.linhasDoBloco(nil)[0]
	if comPonta == semPonta {
		t.Fatal("a ponta devia pintar diferente do traço já assentado")
	}
	if semEstilo(comPonta) != semEstilo(semPonta) {
		t.Fatal("a ponta é só cor: o desenho não pode mudar")
	}
}
