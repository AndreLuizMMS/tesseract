package tela

import (
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

func tiposDeTeste() []protocolo.TipoCelula {
	return []protocolo.TipoCelula{
		{Tipo: "claude", AceitaPrompt: true, Conversa: true},
		{Tipo: "bash"},
		{Tipo: "logs", RotuloAlvo: "SERVIÇO"},
		{Tipo: "md", RotuloAlvo: "MD", CompletaArquivo: true},
	}
}

func digitar(f *Formulario, texto string) {
	for _, letra := range texto {
		f.Tecla(teclado.Nada, string(letra), false)
	}
}

// TestFormularioNasceComOProjetoFocado — o campo já vem preenchido com onde o
// usuário está.
func TestFormularioNasceComOProjetoFocado(t *testing.T) {
	f := NovoFormulario(tiposDeTeste(), "/home/dev/cortz")
	if f.campos[0].valor != "/home/dev/cortz" {
		t.Fatalf("o projeto focado devia vir preenchido, veio %q", f.campos[0].valor)
	}
	if f.campos[1].escolhido() != "claude" {
		t.Fatalf("o primeiro tipo devia vir escolhido, veio %q", f.campos[1].escolhido())
	}
}

// TestFormularioCriaProjetoECelulaNoMesmoGesto — não existe criar projeto
// separado de criar célula.
func TestFormularioCriaProjetoECelulaNoMesmoGesto(t *testing.T) {
	f := NovoFormulario(tiposDeTeste(), "")
	digitar(f, "/home/dev/novo")
	f.Tecla(teclado.CampoProximo, "", false) // TIPO
	f.Tecla(teclado.OpcaoProxima, "", false) // bash
	f.Tecla(teclado.CampoProximo, "", false) // NOME
	digitar(f, "testes")

	pedido, aberto := f.Tecla(teclado.Confirmar, "", false)
	if aberto {
		t.Fatal("confirmar fecha o formulário")
	}
	if pedido == nil {
		t.Fatal("confirmar tinha que devolver o pedido de criação")
	}
	if pedido.Caminho != "/home/dev/novo" || pedido.Tipo != "bash" || pedido.Nome != "testes" {
		t.Fatalf("pedido veio errado: %#v", pedido)
	}
}

// TestFormularioPedeOAlvoConformeOTipo — o quarto campo depende do tipo, e o
// formulário descobre isso pela ficha que o motor mandou, não por saber o que
// cada tipo é.
func TestFormularioPedeOAlvoConformeOTipo(t *testing.T) {
	f := NovoFormulario(tiposDeTeste(), "/home/dev/cortz")
	nomes := func() []string {
		var lista []string
		for _, c := range f.campos {
			lista = append(lista, c.rotulo)
		}
		return lista
	}

	// claude: pede prompt, não pede alvo.
	if juntos := strings.Join(nomes(), " "); !strings.Contains(juntos, "PROMPT") || strings.Contains(juntos, "SERVIÇO") {
		t.Fatalf("campos do claude: %v", nomes())
	}

	f.atual = 1
	f.Tecla(teclado.OpcaoProxima, "", false) // bash
	if juntos := strings.Join(nomes(), " "); strings.Contains(juntos, "PROMPT") {
		t.Fatalf("bash não recebe prompt: %v", nomes())
	}

	f.Tecla(teclado.OpcaoProxima, "", false) // logs
	if juntos := strings.Join(nomes(), " "); !strings.Contains(juntos, "SERVIÇO") {
		t.Fatalf("logs precisa perguntar o serviço: %v", nomes())
	}

	f.Tecla(teclado.OpcaoProxima, "", false) // md
	if juntos := strings.Join(nomes(), " "); !strings.Contains(juntos, "MD") {
		t.Fatalf("md precisa perguntar o arquivo: %v", nomes())
	}
}

// TestFormularioCompletaCaminho — tab completa e diz quantas pastas casam.
func TestFormularioCompletaCaminho(t *testing.T) {
	f := NovoFormulario(tiposDeTeste(), "/home/dev/cor")
	pedido, quer := f.PedeCompletar(teclado.Completar)
	if !quer {
		t.Fatal("o campo de projeto aceita completar")
	}
	if !pedido.SoDiretorio || pedido.Caminho != "/home/dev/cor" {
		t.Fatalf("pedido de completar veio errado: %#v", pedido)
	}

	f.Completou(protocolo.Completado{Caminho: "/home/dev/cortz-", Quantidade: 3})
	if f.campos[0].valor != "/home/dev/cortz-" {
		t.Fatalf("o campo não recebeu o complemento: %q", f.campos[0].valor)
	}
	if !strings.Contains(f.aviso, "3 pastas casam") {
		t.Fatalf("o aviso devia contar quantas casam, veio %q", f.aviso)
	}

	// O campo de nome não completa nada.
	f.atual = 2
	if _, quer := f.PedeCompletar(teclado.Completar); quer {
		t.Error("o campo de nome não completa caminho")
	}
}

// TestFormularioRecusaProjetoVazio — o caminho é obrigatório, e o formulário
// continua aberto com o que foi digitado.
func TestFormularioRecusaProjetoVazio(t *testing.T) {
	f := NovoFormulario(tiposDeTeste(), "")
	pedido, aberto := f.Tecla(teclado.Confirmar, "", false)
	if pedido != nil {
		t.Fatal("sem caminho não se cria nada")
	}
	if !aberto {
		t.Fatal("o formulário continua aberto quando o caminho falta")
	}
	if f.aviso == "" {
		t.Fatal("o formulário precisa dizer o que faltou")
	}
}

// TestFormularioApagaComBackspace cobre a edição do campo de texto.
func TestFormularioApagaComBackspace(t *testing.T) {
	f := NovoFormulario(tiposDeTeste(), "abc")
	f.Tecla(teclado.Nada, "", true)
	if f.campos[0].valor != "ab" {
		t.Fatalf("apagar deixou %q", f.campos[0].valor)
	}
}

// TestFormularioCancelaComEsc.
func TestFormularioCancelaComEsc(t *testing.T) {
	f := NovoFormulario(tiposDeTeste(), "/home/dev")
	pedido, aberto := f.Tecla(teclado.Cancelar, "", false)
	if pedido != nil || aberto {
		t.Fatal("esc cancela sem criar nada")
	}
}
