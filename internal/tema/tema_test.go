package tema

import (
	"strings"
	"testing"
)

// verdesECianos são as cores que NÃO podem virar estado. Verde é posse do
// teclado, ciano é estrutura.
var verdesECianos = []string{
	BrandDeep, BrandCore, BrandLive, BrandPhosphor,
	FluxDeep, FluxCore, Flux,
}

func TestEstadoNuncaUsaVerdeNemCiano(t *testing.T) {
	for estado, m := range mapa {
		for _, proibida := range verdesECianos {
			if m.Cor == proibida {
				t.Fatalf("estado %q usa %s, que é posse do teclado ou estrutura", estado, proibida)
			}
		}
	}
}

func TestSoAprovarOcupaALinhaInteira(t *testing.T) {
	for estado, m := range mapa {
		if m.Invertido != (estado == Aprovar) {
			t.Fatalf("estado %q: invertido=%v — só aprovar preenche a linha", estado, m.Invertido)
		}
	}
}

func TestCadaEstadoTemGlifoProprio(t *testing.T) {
	vistos := map[string]Estado{}
	for estado, m := range mapa {
		if outro, repetido := vistos[m.Glifo]; repetido {
			t.Fatalf("glifo %q em %q e %q — sem cor os dois viram o mesmo sinal", m.Glifo, estado, outro)
		}
		vistos[m.Glifo] = estado
	}
}

func TestSemCorNaoEmiteEscapeDeCor(t *testing.T) {
	anterior := Atual
	Atual = SemCor
	defer func() { Atual = anterior }()

	saida := Do(Aprovar).Linha(20) + Do(Respondeu).Linha(0) + Modo(Digitar).Selo()
	if strings.Contains(saida, "38;2;") || strings.Contains(saida, "48;2;") {
		t.Fatalf("NO_COLOR: saiu escape de cor em %q", saida)
	}
	if !strings.Contains(saida, "⏵") || !strings.Contains(saida, "⬤") {
		t.Fatalf("NO_COLOR: os glifos precisam sobreviver — %q", saida)
	}
}

func TestTodoTokenTemDestinoEmDezesseisCores(t *testing.T) {
	for _, m := range mapa {
		if _, ok := ansi16[m.Cor]; !ok {
			t.Fatalf("cor de estado %s sem equivalente na ANSI 16", m.Cor)
		}
	}
}
