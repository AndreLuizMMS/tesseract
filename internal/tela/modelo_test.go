package tela

import (
	"testing"

	"github.com/andreluiz/tesseract/internal/protocolo"
)

// modeloDeTeste monta a tela sobre um estado, sem motor do outro lado. Só as
// teclas que mexem no foco são exercitadas aqui — as que falam com o motor têm
// teste do lado dele.
func modeloDeTeste(estado protocolo.Estado) *Modelo {
	return &Modelo{estado: estado, visao: visaoMosaico, tamanhos: map[string]Geometria{}, largura: 120, altura: 30}
}

func TestSetasAndamPelaGrade(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())

	m.telaNavegando("down")
	if m.foco.Celula != 1 {
		t.Fatalf("↓ devia ir para a próxima célula, foco %#v", m.foco)
	}
	// No mosaico todas as células estão à vista, então andar atravessa projeto.
	m.telaNavegando("down")
	if m.foco.Projeto != 1 || m.foco.Celula != 0 {
		t.Fatalf("↓ devia atravessar para o projeto seguinte: %#v", m.foco)
	}
	m.telaNavegando("up")
	if m.foco.Projeto != 0 || m.foco.Celula != 1 {
		t.Fatalf("↑ devia voltar para a célula anterior: %#v", m.foco)
	}
	m.foco = Foco{}
	m.telaNavegando("up")
	if m.foco.Projeto != 2 {
		t.Fatalf("↑ na primeira célula dá a volta: %#v", m.foco)
	}

	m.foco = Foco{}
	m.telaNavegando("right")
	if m.foco.Projeto != 1 || m.foco.Celula != 0 {
		t.Fatalf("→ devia ir para o próximo projeto: %#v", m.foco)
	}
	m.telaNavegando("left")
	if m.foco.Projeto != 0 {
		t.Fatalf("← devia voltar: %#v", m.foco)
	}
}

func TestNumeroVaiDiretoAoProjeto(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())
	m.telaNavegando("3")
	if m.foco.Projeto != 2 {
		t.Fatalf("3 devia levar ao terceiro projeto: %#v", m.foco)
	}
	m.telaNavegando("9")
	if m.foco.Projeto != 2 {
		t.Fatalf("número além do que existe fica no último: %#v", m.foco)
	}
}

// TestEspacoPulaParaQuemChamou — atravessa projeto para achar quem pede
// atenção.
func TestEspacoPulaParaQuemChamou(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())
	// Da primeira célula (que já respondeu), o pulo vai para a próxima que
	// chama: a do segundo projeto, que pede aprovação.
	m.telaNavegando("space")
	if m.foco.Projeto != 1 || m.foco.Celula != 0 {
		t.Fatalf("espaço devia atravessar para o projeto que chama: %#v", m.foco)
	}
	// De lá, dá a volta e retorna para a primeira.
	m.telaNavegando("space")
	if m.foco.Projeto != 0 || m.foco.Celula != 0 {
		t.Fatalf("espaço devia dar a volta: %#v", m.foco)
	}
}

// TestEspacoSemNinguemChamandoNaoMexe.
func TestEspacoSemNinguemChamandoNaoMexe(t *testing.T) {
	estado := gradeDeTeste()
	for i := range estado.Projetos {
		for j := range estado.Projetos[i].Celulas {
			estado.Projetos[i].Celulas[j].Estado = "trabalhando"
		}
	}
	m := modeloDeTeste(estado)
	m.foco = Foco{Projeto: 1}
	m.telaNavegando("space")
	if m.foco.Projeto != 1 {
		t.Fatalf("sem ninguém chamando, o foco fica onde está: %#v", m.foco)
	}
}

// TestTelaCheiaLigaEDesliga.
func TestTelaCheiaLigaEDesliga(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())
	m.telaNavegando("o")
	if !m.foco.Cheia {
		t.Fatal("o devia pôr em tela cheia")
	}
	m.telaNavegando("o")
	if m.foco.Cheia {
		t.Fatal("o de novo devia sair da tela cheia")
	}
	m.telaNavegando("o")
	m.telaNavegando("esc")
	if m.foco.Cheia {
		t.Fatal("esc também sai da tela cheia")
	}
}

// TestAjudaAbreEFecha.
func TestAjudaAbreEFecha(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())
	m.telaNavegando("?")
	if !m.ajuda {
		t.Fatal("? devia abrir a ajuda")
	}
	if m.Modo().String() != "NAVEGAR" {
		t.Fatal("a ajuda não muda o dono do teclado")
	}
}

// TestModoMostraQuemTemOTeclado — nunca há dois donos ao mesmo tempo.
func TestModoMostraQuemTemOTeclado(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())
	if m.Modo().String() != "NAVEGAR" {
		t.Fatalf("o padrão é NAVEGAR, veio %s", m.Modo())
	}
	m.digitando = true
	if m.Modo().String() != "DIGITAR" {
		t.Fatalf("digitando é DIGITAR, veio %s", m.Modo())
	}
	m.formulario = NovoFormulario(m.estado.Tipos, "")
	if m.Modo().String() != "FORMULÁRIO" {
		t.Fatalf("com formulário aberto o teclado é dele, veio %s", m.Modo())
	}
	m.painel = NovoPainelDocker("p1", "cortz")
	if m.Modo().String() != "DOCKER" {
		t.Fatalf("com o painel aberto o teclado é dele, veio %s", m.Modo())
	}
}

// TestFocoNuncaSaiDoQueExiste — estado que encolhe não deixa a tela apontando
// para o vazio.
func TestFocoNuncaSaiDoQueExiste(t *testing.T) {
	m := modeloDeTeste(gradeDeTeste())
	m.foco = Foco{Projeto: 2, Celula: 5}
	m.estado = protocolo.Estado{Projetos: gradeDeTeste().Projetos[:1]}
	m.foco = Ajustar(m.estado, m.foco)
	if m.foco.Projeto != 0 || m.foco.Celula != 1 {
		t.Fatalf("o foco devia ter sido puxado para dentro do que existe: %#v", m.foco)
	}

	m.estado = protocolo.Estado{}
	m.foco = Ajustar(m.estado, m.foco)
	if m.foco.Projeto != 0 || m.foco.Celula != 0 {
		t.Fatalf("sem projeto nenhum, o foco zera: %#v", m.foco)
	}
	if m.celulaFocada() != nil {
		t.Fatal("sem célula, não há célula focada")
	}
}
