package tela

import (
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/protocolo"
)

func painelDeTeste() *PainelDocker {
	painel := NovoPainelDocker("p1", "doxar-api")
	painel.Chegou(protocolo.Servicos{
		Projeto: "p1", Arquivo: "/home/dev/doxar-api/docker-compose.yml",
		Lista: []protocolo.Servico{
			{Nome: "api", Estado: "up", Porta: ":3000", Saude: "saudável", Uptime: "2h14m"},
			{Nome: "postgres", Estado: "up", Porta: ":5432", Saude: "saudável", Uptime: "2h14m"},
			{Nome: "worker", Estado: "exited (1)"},
		},
	})
	return painel
}

// TestPainelEscolheSemExecutar — as setas escolhem o serviço e não executam
// ação nenhuma.
func TestPainelEscolheSemExecutar(t *testing.T) {
	painel := painelDeTeste()
	for _, tecla := range []string{"down", "down", "up"} {
		pedido, aberto := painel.Tecla(tecla)
		if pedido != nil {
			t.Fatalf("a tecla %q executou %#v", tecla, pedido)
		}
		if !aberto {
			t.Fatalf("a tecla %q fechou o painel", tecla)
		}
	}
	if painel.Escolhido != 1 {
		t.Fatalf("o escolhido é o %d", painel.Escolhido)
	}
}

// TestPainelAgeNoServicoEnaStack — a maiúscula é sempre a versão da stack
// inteira da minúscula correspondente.
func TestPainelAgeNoServicoEnaStack(t *testing.T) {
	casos := []struct {
		tecla   string
		acao    string
		servico string
	}{
		{"u", "sobe", "api"},
		{"s", "para", "api"},
		{"r", "reinicia", "api"},
		{"b", "rebuilda", "api"},
		{"U", "sobe", ""},
		{"S", "para", ""},
		{"R", "reinicia", ""},
	}
	for _, caso := range casos {
		painel := painelDeTeste()
		pedido, aberto := painel.Tecla(caso.tecla)
		if pedido == nil {
			t.Fatalf("a tecla %q não pediu nada", caso.tecla)
		}
		if pedido.Acao != caso.acao || pedido.Servico != caso.servico {
			t.Fatalf("a tecla %q pediu %#v", caso.tecla, pedido)
		}
		if !aberto {
			t.Fatalf("a tecla %q não devia fechar o painel", caso.tecla)
		}
	}
}

// TestPainelLogViraCelula — pedir o log fecha o painel e nasce uma célula.
func TestPainelLogViraCelula(t *testing.T) {
	painel := painelDeTeste()
	painel.Tecla("down") // postgres
	pedido, aberto := painel.Tecla("l")
	if pedido == nil || pedido.Acao != "log" || pedido.Servico != "postgres" {
		t.Fatalf("pedido de log veio %#v", pedido)
	}
	if aberto {
		t.Fatal("o log vira célula no mosaico, então o painel fecha")
	}
}

// TestPainelEscDevolveOTeclado.
func TestPainelEscDevolveOTeclado(t *testing.T) {
	painel := painelDeTeste()
	pedido, aberto := painel.Tecla("esc")
	if pedido != nil || aberto {
		t.Fatal("esc só fecha")
	}
}

// TestPainelIgnoraTeclaDeFora — enquanto o painel está aberto, o teclado é
// dele: tecla que não é dele não faz nada.
func TestPainelIgnoraTeclaDeFora(t *testing.T) {
	painel := painelDeTeste()
	for _, tecla := range []string{"n", "D", "q", "v", "p", "/", "?"} {
		pedido, aberto := painel.Tecla(tecla)
		if pedido != nil {
			t.Errorf("a tecla %q não é do painel, mas pediu %#v", tecla, pedido)
		}
		if !aberto {
			t.Errorf("a tecla %q fechou o painel", tecla)
		}
	}
}

// TestPainelDesenhaOEstadoDaStack.
func TestPainelDesenhaOEstadoDaStack(t *testing.T) {
	desenho := semEstilo(strings.Join(painelDeTeste().Desenhar(90), "\n"))
	for _, pedaco := range []string{"DOCKER · doxar-api", "docker-compose.yml", "api", ":3000", "saudável", "2h14m", "exited (1)"} {
		if !strings.Contains(desenho, pedaco) {
			t.Errorf("falta %q no painel:\n%s", pedaco, desenho)
		}
	}
	for _, proibido := range []string{"down -v", "apagar", "volume"} {
		if strings.Contains(desenho, proibido) {
			t.Errorf("o painel não pode oferecer %q", proibido)
		}
	}
}

// TestPainelSemStackNaoQuebra.
func TestPainelSemStackNaoQuebra(t *testing.T) {
	painel := NovoPainelDocker("p1", "novo")
	painel.Chegou(protocolo.Servicos{Projeto: "p1"})
	desenho := semEstilo(strings.Join(painel.Desenhar(90), "\n"))
	if !strings.Contains(desenho, "nunca subiu") {
		t.Errorf("stack vazia precisa dizer isso:\n%s", desenho)
	}
	pedido, _ := painel.Tecla("l")
	if pedido != nil {
		t.Error("sem serviço escolhido não há log para abrir")
	}
}
