package tela

import (
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

func novoFormularioDeTeste() *Formulario {
	return NovoFormulario(nil, "")
}

func digitar(f *Formulario, texto string) {
	for _, letra := range texto {
		f.Tecla(teclado.Nada, string(letra), false)
	}
}

func rotulos(f *Formulario) []string {
	var lista []string
	for _, c := range f.campos {
		lista = append(lista, c.rotulo)
	}
	return lista
}

// TestFormularioComecaNaCasa — o caminho já vem apontando para a casa, que é de
// onde todo projeto sai.
func TestFormularioComecaNaCasa(t *testing.T) {
	f := novoFormularioDeTeste()
	valor := f.campos[0].valor
	if valor != casaComBarra() {
		t.Fatalf("o campo devia começar na casa, veio %q", valor)
	}
	if !strings.HasSuffix(valor, "/") {
		t.Fatalf("o caminho devia terminar em barra para o tab completar de dentro: %q", valor)
	}
}

// TestFormularioNaoPerguntaOTipo — criar uma sessão não obriga a escolher entre
// claude, cursor e shell.
func TestFormularioNaoPerguntaOTipo(t *testing.T) {
	f := novoFormularioDeTeste()
	for _, rotulo := range rotulos(f) {
		if rotulo == "TIPO" {
			t.Fatalf("o formulário não pergunta mais o tipo: %v", rotulos(f))
		}
	}
	esperados := []string{"PROJETO", "NOME", "MD", "PROMPT"}
	if strings.Join(rotulos(f), " ") != strings.Join(esperados, " ") {
		t.Fatalf("campos vieram %v, esperado %v", rotulos(f), esperados)
	}
}

// TestFormularioCriaSemDizerOTipo — o pedido sai sem tipo, e quem decide é o
// motor.
func TestFormularioCriaSemDizerOTipo(t *testing.T) {
	f := novoFormularioDeTeste()
	for range 40 {
		f.Tecla(teclado.Nada, "", true) // limpa o caminho da casa
	}
	digitar(f, "/home/dev/novo")
	f.Tecla(teclado.CampoProximo, "", false) // NOME
	digitar(f, "refatora auth")

	pedido, aberto := f.Tecla(teclado.Confirmar, "", false)
	if aberto {
		t.Fatal("confirmar fecha o formulário")
	}
	if pedido == nil {
		t.Fatal("confirmar tinha que devolver o pedido de criação")
	}
	if pedido.Tipo != "" {
		t.Fatalf("o formulário não escolhe tipo: %#v", pedido)
	}
	if pedido.Caminho != "/home/dev/novo" || pedido.Nome != "refatora auth" {
		t.Fatalf("pedido veio errado: %#v", pedido)
	}
}

// TestFormularioMandaOMarkdownComoAlvo — arquivo preenchido vira célula de
// leitura, e quem traduz isso é o motor.
func TestFormularioMandaOMarkdownComoAlvo(t *testing.T) {
	f := novoFormularioDeTeste()
	f.atual = 2 // MD
	digitar(f, "docs/spec-m7.md")

	pedido, _ := f.Tecla(teclado.Confirmar, "", false)
	if pedido == nil {
		t.Fatal("o pedido devia sair")
	}
	if pedido.Alvo != "docs/spec-m7.md" {
		t.Fatalf("o arquivo devia ir no alvo: %#v", pedido)
	}
}

// TestFormularioCompletaCaminhoDePasta — no campo do projeto, tab só olha
// pasta.
func TestFormularioCompletaCaminhoDePasta(t *testing.T) {
	f := novoFormularioDeTeste()
	digitar(f, "dev/cor")

	pedido, quer := f.PedeCompletar(teclado.Completar)
	if !quer {
		t.Fatal("o campo de projeto aceita completar")
	}
	if !pedido.SoDiretorio || !strings.HasSuffix(pedido.Caminho, "dev/cor") {
		t.Fatalf("pedido de completar veio errado: %#v", pedido)
	}

	f.Completou(protocolo.Completado{Caminho: "/home/andreluiz/dev/cortz-", Quantidade: 3})
	if f.campos[0].valor != "/home/andreluiz/dev/cortz-" {
		t.Fatalf("o campo não recebeu o complemento: %q", f.campos[0].valor)
	}
	if !strings.Contains(f.aviso, "3 pastas casam") {
		t.Fatalf("o aviso devia contar quantas casam, veio %q", f.aviso)
	}
}

// TestFormularioCompletaArquivoDentroDoProjeto — no campo do markdown, tab
// procura dentro do projeto que está sendo criado.
func TestFormularioCompletaArquivoDentroDoProjeto(t *testing.T) {
	f := novoFormularioDeTeste()
	f.campos[0].valor = "/home/dev/cortz"
	f.atual = 2
	digitar(f, "READ")

	pedido, quer := f.PedeCompletar(teclado.Completar)
	if !quer {
		t.Fatal("o campo do markdown aceita completar")
	}
	if pedido.SoDiretorio {
		t.Fatal("aqui o que interessa é arquivo, não pasta")
	}
	if pedido.Caminho != "/home/dev/cortz/READ" {
		t.Fatalf("o caminho devia ser relativo ao projeto: %q", pedido.Caminho)
	}

	f.Completou(protocolo.Completado{Caminho: "/home/dev/cortz/README", Quantidade: 2})
	if !strings.Contains(f.aviso, "2 arquivos casam") {
		t.Fatalf("o aviso devia falar de arquivo, veio %q", f.aviso)
	}
}

// TestFormularioRecusaProjetoVazio — o caminho é obrigatório, e o formulário
// continua aberto com o que foi digitado.
func TestFormularioRecusaProjetoVazio(t *testing.T) {
	f := novoFormularioDeTeste()
	f.campos[0].valor = ""
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
	f := novoFormularioDeTeste()
	f.campos[0].valor = "abc"
	f.Tecla(teclado.Nada, "", true)
	if f.campos[0].valor != "ab" {
		t.Fatalf("apagar deixou %q", f.campos[0].valor)
	}
}

// TestFormularioCancelaComEsc.
func TestFormularioCancelaComEsc(t *testing.T) {
	f := novoFormularioDeTeste()
	pedido, aberto := f.Tecla(teclado.Cancelar, "", false)
	if pedido != nil || aberto {
		t.Fatal("esc cancela sem criar nada")
	}
}

// TestFormularioAndaEntreOsCampos.
func TestFormularioAndaEntreOsCampos(t *testing.T) {
	f := novoFormularioDeTeste()
	f.Tecla(teclado.CampoProximo, "", false)
	if f.atual != 1 {
		t.Fatalf("↓ devia ir para o campo seguinte, está no %d", f.atual)
	}
	f.Tecla(teclado.CampoAnterior, "", false)
	if f.atual != 0 {
		t.Fatalf("↑ devia voltar, está no %d", f.atual)
	}
	f.Tecla(teclado.CampoAnterior, "", false)
	if f.atual != len(f.campos)-1 {
		t.Fatalf("↑ no primeiro campo dá a volta, está no %d", f.atual)
	}
}
