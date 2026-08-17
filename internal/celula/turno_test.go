package celula

import "testing"

// marcadoresDeTeste imita um agente que anuncia quando está trabalhando e
// quando está esperando resposta.
var marcadoresDeTeste = Marcadores{
	Trabalho: []string{"esc to interrupt", "tokens)"},
	Pergunta: []string{"No, and tell Claude what to do differently"},
}

// telaComSpinner devolve a tela de um agente parado, com o spinner num quadro
// diferente e o cursor em outro lugar — a tela muda, mas nada está acontecendo.
func telaComSpinner(quadro int) string {
	giro := []string{"⠋", "⠙", "⠹", "⠸"}[quadro%4]
	cursor := []string{"█", " "}[quadro%2]
	return "conversa antiga\n" + giro + " pronto para o próximo pedido\n> " + cursor
}

// TestSpinnerNaoDisparaResposta é a regra do alarme falso: a tela mudando
// sozinha não é trabalho, então não existe turno para encerrar.
func TestSpinnerNaoDisparaResposta(t *testing.T) {
	turno := NovoTurno(marcadoresDeTeste)
	for quadro := range 40 {
		if estado := turno.Observar(telaComSpinner(quadro)); estado == Respondeu {
			t.Fatalf("spinner piscando disparou resposta no quadro %d", quadro)
		}
	}
}

// TestTrabalhoSeguidoDeSilencioDisparaResposta é o caminho feliz.
func TestTrabalhoSeguidoDeSilencioDisparaResposta(t *testing.T) {
	turno := NovoTurno(marcadoresDeTeste)

	for i := range leiturasParaArmar {
		estado := turno.Observar("escrevendo o arquivo…\nesc to interrupt")
		if estado != Trabalhando {
			t.Fatalf("leitura %d de trabalho devia estar trabalhando, veio %q", i, estado)
		}
	}

	for i := range leiturasParaEncerrar - 1 {
		if estado := turno.Observar("terminei. cubro o menu mobile também?"); estado == Respondeu {
			t.Fatalf("declarou o turno encerrado cedo demais, no silêncio %d", i)
		}
	}
	if estado := turno.Observar("terminei. cubro o menu mobile também?"); estado != Respondeu {
		t.Fatalf("depois do silêncio inteiro devia ter respondido, veio %q", estado)
	}
}

// TestTrabalhoCurtoNaoArma — um piscar de trabalho não conta como turno.
func TestTrabalhoCurtoNaoArma(t *testing.T) {
	turno := NovoTurno(marcadoresDeTeste)
	turno.Observar("esc to interrupt")
	for range 20 {
		if estado := turno.Observar("nada acontecendo aqui"); estado == Respondeu {
			t.Fatal("um piscar de trabalho não pode virar resposta")
		}
	}
}

// TestPerguntaViraAprovarENaoRespondeu — agente travado numa pergunta bloqueia
// o trabalho; agente que terminou o turno só tem algo para ler.
func TestPerguntaViraAprovarENaoRespondeu(t *testing.T) {
	turno := NovoTurno(marcadoresDeTeste)
	for range leiturasParaArmar {
		turno.Observar("editando…\nesc to interrupt")
	}

	pergunta := "Do you want to make this edit?\n1. Yes\n2. No, and tell Claude what to do differently"
	for i := range leiturasParaEncerrar * 3 {
		estado := turno.Observar(pergunta)
		if estado != Aprovar {
			t.Fatalf("leitura %d: esperava aprovar, veio %q", i, estado)
		}
	}

	// Respondida a pergunta, o agente volta a trabalhar e o turno segue.
	for range leiturasParaArmar {
		turno.Observar("aplicando…\nesc to interrupt")
	}
	for range leiturasParaEncerrar {
		turno.Observar("pronto, apliquei.")
	}
	if estado := turno.Estado(); estado != Respondeu {
		t.Fatalf("depois da pergunta respondida e do trabalho terminado, esperava respondeu, veio %q", estado)
	}
}

// TestVistoLimpaOChamado — quem foi lido para de chamar.
func TestVistoLimpaOChamado(t *testing.T) {
	turno := NovoTurno(marcadoresDeTeste)
	for range leiturasParaArmar {
		turno.Observar("esc to interrupt")
	}
	for range leiturasParaEncerrar {
		turno.Observar("terminei")
	}
	if turno.Estado() != Respondeu {
		t.Fatalf("esperava respondeu, veio %q", turno.Estado())
	}
	turno.Visto()
	if estado := turno.Estado(); estado == Respondeu {
		t.Fatal("depois de olhar a célula, ela não pode continuar chamando")
	}
}

// TestQualquerMarcadorDeTrabalhoServe — o agente troca o texto entre versões, e
// a célula reconhece os dois.
func TestQualquerMarcadorDeTrabalhoServe(t *testing.T) {
	for _, marcador := range []string{"esc to interrupt", "Cogitating… (4s · ↓ 18 tokens)"} {
		turno := NovoTurno(marcadoresDeTeste)
		for range leiturasParaArmar {
			if estado := turno.Observar("trabalhando\n" + marcador); estado != Trabalhando {
				t.Fatalf("o marcador %q não foi reconhecido", marcador)
			}
		}
		for range leiturasParaEncerrar {
			turno.Observar("terminei")
		}
		if turno.Estado() != Respondeu {
			t.Fatalf("com o marcador %q o turno não encerrou", marcador)
		}
	}
}

// TestSemMarcadorUsaATelaMudando cobre o agente que não fala nada sobre si.
func TestSemMarcadorUsaATelaMudando(t *testing.T) {
	turno := NovoTurno(Marcadores{})
	for i := range leiturasParaArmar {
		if estado := turno.Observar("linha " + string(rune('a'+i))); estado != Trabalhando {
			t.Fatalf("tela mudando é o único sinal que sobra: veio %q", estado)
		}
	}
	// A primeira leitura de "tela parada" ainda é uma mudança de tela; o
	// silêncio começa a contar da seguinte.
	for range leiturasParaEncerrar + 1 {
		turno.Observar("tela parada")
	}
	if estado := turno.Estado(); estado != Respondeu {
		t.Fatalf("tela que parou de mudar encerra o turno, veio %q", estado)
	}
}
