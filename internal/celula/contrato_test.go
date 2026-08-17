package celula

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/motor/historico"
)

// prepararTipo monta o que cada tipo precisa para nascer num diretório de
// teste, sem depender de agente de verdade nem de stack de pé.
func prepararTipo(t *testing.T, tipo, dir string) Config {
	t.Helper()
	cfg := Config{ID: "c-" + tipo, Diretorio: dir, Nome: tipo, Colunas: 60, Linhas: 12}

	switch tipo {
	case "claude", "cursor":
		// Um agente de mentira, que aceita qualquer argumento e fica de pé: o
		// contrato é da célula, não do agente.
		cfg.Programa = agenteDeMentira(t, dir)
	case "logs":
		compose := filepath.Join(dir, "docker-compose.yml")
		if err := os.WriteFile(compose, []byte("services:\n  web:\n    image: nginx\n"), 0o644); err != nil {
			t.Fatalf("preparar compose: %v", err)
		}
		cfg.Alvo = "web"
	case "md":
		arquivo := filepath.Join(dir, "leia.md")
		if err := os.WriteFile(arquivo, []byte("# Título\n\ntexto do arquivo\n"), 0o644); err != nil {
			t.Fatalf("preparar markdown: %v", err)
		}
		cfg.Alvo = arquivo
	}
	return cfg
}

// agenteDeMentira é um programa que ignora os argumentos e fica vivo, para os
// tipos de agente nascerem sem depender do agente de verdade.
func agenteDeMentira(t *testing.T, dir string) string {
	t.Helper()
	caminho := filepath.Join(dir, "agente-de-mentira")
	corpo := "#!/bin/sh\nwhile true; do sleep 1; done\n"
	if err := os.WriteFile(caminho, []byte(corpo), 0o755); err != nil {
		t.Fatalf("preparar agente: %v", err)
	}
	return caminho
}

// TestTodoTipoCumpreOContrato percorre o registro inteiro: um tipo novo entra
// aqui de graça, e se ele não responder a nascer, desenhar, receber tecla e
// informar estados, este teste quebra.
func TestTodoTipoCumpreOContrato(t *testing.T) {
	tipos := Tipos()
	if len(tipos) < 5 {
		t.Fatalf("o registro devia ter os cinco tipos do V1, tem %v", tipos)
	}

	for _, tipo := range tipos {
		t.Run(tipo, func(t *testing.T) {
			dir := t.TempDir()
			cfg := prepararTipo(t, tipo, dir)

			registro, err := historico.Abrir(filepath.Join(dir, "hist.log"), historico.TetoPadrao)
			if err != nil {
				t.Fatalf("abrir histórico: %v", err)
			}
			defer registro.Fechar()
			cfg.Historico = registro

			celula, err := Nova(tipo)
			if err != nil {
				t.Fatalf("fabricar: %v", err)
			}

			// Nasce.
			if err := celula.Nascer(cfg); err != nil {
				t.Fatalf("nascer: %v", err)
			}
			defer celula.Matar()

			// Informa os estados que tem, e o estado de agora é um deles.
			estados := celula.Estados()
			if len(estados) == 0 {
				t.Fatal("o tipo não declara nenhum estado")
			}
			time.Sleep(200 * time.Millisecond)
			atual := celula.Estado()
			declarado := false
			for _, estado := range estados {
				if estado == atual {
					declarado = true
				}
			}
			if !declarado {
				t.Fatalf("o estado atual %q não está entre os declarados %v", atual, estados)
			}

			// Desenha.
			quadro := celula.Desenhar()
			if quadro.Linhas == nil && quadro.CursorX == 0 && quadro.CursorY == 0 && !quadro.AoVivo {
				t.Fatal("desenhar não devolveu quadro nenhum")
			}

			// Recebe tecla sem explodir (as de só leitura ignoram).
			if err := celula.Tecla(Toque{Codigo: 'a', Texto: "a"}); err != nil {
				t.Fatalf("tecla: %v", err)
			}

			// Aceita tamanho e rolagem.
			if err := celula.Redimensionar(80, 20); err != nil {
				t.Fatalf("redimensionar: %v", err)
			}
			celula.Rolar(3, false)
			celula.Rolar(0, true)

			// Morre.
			if err := celula.Matar(); err != nil {
				t.Fatalf("matar: %v", err)
			}
		})
	}
}

// TestFichasDosTiposSaoCoerentes — o formulário monta os campos a partir das
// fichas, então elas precisam estar completas.
func TestFichasDosTiposSaoCoerentes(t *testing.T) {
	for _, ficha := range Descritores() {
		if ficha.Tipo == "" {
			t.Fatal("ficha sem tipo")
		}
		if ficha.CompletaArquivo && ficha.RotuloAlvo == "" {
			t.Errorf("%s: completa arquivo num campo que não existe", ficha.Tipo)
		}
		if _, existe := Ficha(ficha.Tipo); !existe {
			t.Errorf("%s: registrado mas sem ficha", ficha.Tipo)
		}
	}
	if ficha, _ := Ficha("md"); ficha.AceitaPrompt {
		t.Error("markdown não recebe prompt: a célula só lê")
	}
	if ficha, _ := Ficha("claude"); !ficha.AceitaPrompt || !ficha.Conversa {
		t.Error("claude aceita prompt e tem conversa")
	}
	if ficha, _ := Ficha("logs"); ficha.RotuloAlvo == "" {
		t.Error("a célula de log precisa perguntar qual serviço")
	}
}

// TestAgenteSemTranscricaoNaoTentaRetomar é o que impede a célula de morrer na
// largada depois de uma queda: uma conversa que nunca chegou ao disco recomeça
// com a mesma identidade em vez de ser retomada.
func TestAgenteSemTranscricaoNaoTentaRetomar(t *testing.T) {
	if temTranscricao(t.TempDir(), "99b7c1fb-cb36-4485-b29c-324c994d4607") {
		t.Fatal("uma conversa que nunca existiu não pode parecer retomável")
	}
	if temTranscricao(t.TempDir(), "") {
		t.Fatal("conversa sem identidade não é retomável")
	}
	if temConversaDoCursor("conversa-que-nao-existe") {
		t.Fatal("conversa inexistente do Cursor não pode parecer retomável")
	}
}
