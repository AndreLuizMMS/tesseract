package protocolo

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

// caso é uma mensagem do contrato com um valor de exemplo bem preenchido.
type caso struct {
	nome  string
	tipo  string
	valor any
	vazio func() any
}

func casos() []caso {
	return []caso{
		{"tecla", TipoTecla, Tecla{Celula: "c1", Codigo: 'l', Texto: "l"}, func() any { return new(Tecla) }},
		{"tecla colada", TipoTecla, Tecla{Celula: "c1", Colar: "ls -la\n"}, func() any { return new(Tecla) }},
		{"tamanho", TipoTamanho, Tamanho{Celula: "c1", Colunas: 120, Linhas: 40}, func() any { return new(Tamanho) }},
		{"rolar", TipoRolar, Rolar{Celula: "c1", Delta: -3, AoVivo: true}, func() any { return new(Rolar) }},
		{"criar", TipoCriar, Criar{Caminho: "/dev/x", Tipo: "bash", Nome: "testes", Alvo: "api", Prompt: "oi"}, func() any { return new(Criar) }},
		{"matar", TipoMatar, Matar{Celula: "c1"}, func() any { return new(Matar) }},
		{"renomear", TipoRenomear, Renomear{Celula: "c1", Nome: "fix nav"}, func() any { return new(Renomear) }},
		{"adotar", TipoAdotar, Adotar{Celula: "c1"}, func() any { return new(Adotar) }},
		{"retomar", TipoRetomar, Retomar{Celula: "c1"}, func() any { return new(Retomar) }},
		{"aba", TipoAba, Aba{Celula: "c1", Passo: 1}, func() any { return new(Aba) }},
		{"prompt", TipoPrompt, Prompt{Celula: "c1", Texto: "cobre o menu mobile"}, func() any { return new(Prompt) }},
		{"completar", TipoCompletar, Completar{Caminho: "~/dev/cor", SoDiretorio: true}, func() any { return new(Completar) }},
		{"completado", TipoCompletado, Completado{Caminho: "/home/a/dev/cortz", Quantidade: 3}, func() any { return new(Completado) }},
		{"tela", TipoTela, Tela{Tela: "lista"}, func() any { return new(Tela) }},
		{"buscar", TipoBuscar, Buscar{Celula: "c1", Termo: "erro"}, func() any { return new(Buscar) }},
		{"achados", TipoAchados, Achados{Celula: "c1", Termo: "erro", Linhas: []Achado{{Linha: 12, Texto: "erro: x"}}}, func() any { return new(Achados) }},
		{"servicos", TipoServicos, Servicos{
			Projeto: "p1", Arquivo: "/dev/cortz/docker-compose.yml", Acao: "sobe", Servico: "api",
			Lista: []Servico{{Nome: "api", Estado: "up", Porta: ":3000", Saude: "saudável", Uptime: "2h14m"}},
		}, func() any { return new(Servicos) }},
		{"editor", TipoEditor, Editor{Projeto: "p1"}, func() any { return new(Editor) }},
		{"ir para linha", TipoIrParaLinha, IrParaLinha{Celula: "c1", Linha: 42}, func() any { return new(IrParaLinha) }},
		{"docker", TipoDocker, Docker{Projeto: "p1", Acao: "sobe", Servico: "api"}, func() any { return new(Docker) }},
		{"status", TipoStatus, Resumo{}, func() any { return new(Resumo) }},
		{"parar", TipoParar, Resumo{}, func() any { return new(Resumo) }},
		{"resumo", TipoResumo, Resumo{Texto: "motor rodando · 1 projeto"}, func() any { return new(Resumo) }},
		{"erro", TipoErro, Erro{Mensagem: "caminho não existe"}, func() any { return new(Erro) }},
		{"estado", TipoEstado, Estado{
			Aviso: "estado anterior preservado",
			Tela:  "mosaico",
			Quota: &Quota{Percentual: 59, Vira: "2:47"},
			Tipos: []TipoCelula{{Tipo: "md", RotuloAlvo: "MD", CompletaArquivo: true}},
			Projetos: []Projeto{{
				ID: "p1", Caminho: "/dev/cortz", Nome: "cortz", Cor: 2,
				TemCompose: true, Docker: "4/5",
				Celulas: []Celula{{
					ID: "c1", Tipo: "bash", Nome: "testes", Estado: "trabalhando",
					Linhas: []string{"$ pnpm test", "  ok"}, CursorX: 2, CursorY: 1,
					Rolagem: 4, AoVivo: false,
					Abas: []string{"claude", "cursor", "bash"}, Aba: "claude",
				}},
			}},
		}, func() any { return new(Estado) }},
	}
}

// TestIdaEVolta garante que toda mensagem do contrato atravessa o socket e
// volta igual. É o que impede a tela e o motor de discordarem em silêncio.
func TestIdaEVolta(t *testing.T) {
	for _, c := range casos() {
		t.Run(c.nome, func(t *testing.T) {
			env, err := Empacotar(c.tipo, c.valor)
			if err != nil {
				t.Fatalf("empacotar: %v", err)
			}

			var fio bytes.Buffer
			if err := json.NewEncoder(&fio).Encode(env); err != nil {
				t.Fatalf("escrever no fio: %v", err)
			}
			if bytes.Count(fio.Bytes(), []byte("\n")) != 1 {
				t.Fatalf("mensagem precisa ocupar exatamente uma linha, veio %q", fio.String())
			}

			var chegou Mensagem
			if err := json.NewDecoder(&fio).Decode(&chegou); err != nil {
				t.Fatalf("ler do fio: %v", err)
			}
			if chegou.Tipo != c.tipo {
				t.Fatalf("tipo virou %q, esperado %q", chegou.Tipo, c.tipo)
			}

			destino := c.vazio()
			if err := json.Unmarshal(chegou.Dados, destino); err != nil {
				t.Fatalf("desempacotar: %v", err)
			}
			volta := reflect.ValueOf(destino).Elem().Interface()
			if !reflect.DeepEqual(volta, c.valor) {
				t.Fatalf("valor mudou na viagem:\nfoi   %#v\nvoltou %#v", c.valor, volta)
			}
		})
	}
}

// TestTodoTipoTemCaso é a garantia mecânica de que ninguém acrescenta uma
// mensagem ao contrato sem provar que ela volta igual.
func TestTodoTipoTemCaso(t *testing.T) {
	todos := []string{
		TipoTecla, TipoTamanho, TipoRolar, TipoCriar, TipoMatar, TipoRenomear,
		TipoAdotar, TipoRetomar, TipoAba, TipoPrompt, TipoCompletar, TipoTela, TipoBuscar,
		TipoDocker, TipoEditor, TipoIrParaLinha, TipoStatus, TipoParar, TipoEstado, TipoCompletado,
		TipoAchados, TipoServicos, TipoResumo, TipoErro,
	}
	cobertos := map[string]bool{}
	for _, c := range casos() {
		cobertos[c.tipo] = true
	}
	for _, tipo := range todos {
		if !cobertos[tipo] {
			t.Errorf("a mensagem %q não tem caso de ida e volta", tipo)
		}
	}
}

// TestDesempacotarTipado cobre o atalho genérico usado pelo motor e pela tela.
func TestDesempacotarTipado(t *testing.T) {
	env, err := Empacotar(TipoTecla, Tecla{Celula: "c9", Codigo: 'c', Mod: 4})
	if err != nil {
		t.Fatalf("empacotar: %v", err)
	}
	tecla, err := Desempacotar[Tecla](env)
	if err != nil {
		t.Fatalf("desempacotar: %v", err)
	}
	if tecla.Celula != "c9" || tecla.Codigo != 'c' || tecla.Mod != 4 {
		t.Fatalf("tecla voltou errada: %#v", tecla)
	}
}
