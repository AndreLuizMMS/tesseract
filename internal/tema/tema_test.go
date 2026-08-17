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

func TestMascaraDoSimboloCobreODesenho(t *testing.T) {
	if len(mascaraDoSimbolo) != len(Simbolo) {
		t.Fatalf("a máscara tem %d linhas e o símbolo %d", len(mascaraDoSimbolo), len(Simbolo))
	}
	for i, linha := range Simbolo {
		if len([]rune(mascaraDoSimbolo[i])) != len([]rune(linha)) {
			t.Fatalf("linha %d: máscara %q não cobre %q", i, mascaraDoSimbolo[i], linha)
		}
	}
}

func TestSimboloPintadoNaoMexeNoDesenho(t *testing.T) {
	anterior := Atual
	Atual = CorTotal
	defer func() { Atual = anterior }()

	for i, linha := range SimboloPintado() {
		if limpo := semEscape(linha); limpo != Simbolo[i] {
			t.Fatalf("linha %d virou %q, devia continuar %q", i, limpo, Simbolo[i])
		}
	}
}

func TestPiscadaNaoTiraAAreaPreenchida(t *testing.T) {
	anterior, antesApagado := Atual, Apagado
	Atual = CorTotal
	defer func() { Atual, Apagado = anterior, antesApagado }()

	Apagado = false
	aceso := Do(Aprovar).Estilo().Render("x")
	Apagado = true
	apagado := Do(Aprovar).Estilo().Render("x")

	if aceso == apagado {
		t.Fatal("o quadro apagado devia ser diferente do aceso")
	}
	for nome, saida := range map[string]string{"aceso": aceso, "apagado": apagado} {
		if !strings.Contains(saida, "48;") {
			t.Fatalf("quadro %s perdeu o fundo — a barra tem que continuar barra: %q", nome, saida)
		}
	}
}

// semEscape tira os códigos de cor, para o teste olhar só o desenho.
func semEscape(texto string) string {
	var saida strings.Builder
	dentro := false
	for _, r := range texto {
		switch {
		case r == 0x1b:
			dentro = true
		case dentro:
			if r >= 0x40 && r <= 0x7e && r != '[' {
				dentro = false
			}
		default:
			saida.WriteRune(r)
		}
	}
	return saida.String()
}

func TestTodoTokenTemDestinoEmDezesseisCores(t *testing.T) {
	for _, m := range mapa {
		if _, ok := ansi16[m.Cor]; !ok {
			t.Fatalf("cor de estado %s sem equivalente na ANSI 16", m.Cor)
		}
	}
}
