package motor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/docker"
	"github.com/andreluiz/tesseract/internal/protocolo"
)

// agenteQueAnotaOQueRecebe é um agente de mentira que grava tudo o que chega no
// teclado dele. É o que permite provar que reatar não dispara trabalho.
func agenteQueAnotaOQueRecebe(t *testing.T, dir, registro string) string {
	t.Helper()
	caminho := filepath.Join(dir, "agente-que-anota")
	// Lê byte a byte com dd, sem buffer: o teste precisa enxergar até um único
	// caractere que tenha chegado ao teclado do agente.
	corpo := "#!/bin/sh\nwhile true; do dd bs=1 count=1 2>/dev/null >> " + registro + " || exit 0; done\n"
	if err := os.WriteFile(caminho, []byte(corpo), 0o755); err != nil {
		t.Fatalf("preparar agente: %v", err)
	}
	return caminho
}

func configComAgenteDeMentira(programa string) Configuracao {
	config := configuracaoDeTeste()
	config.Agentes = map[string]PerfilAgente{
		"claude": {Programa: programa},
		"cursor": {Programa: programa},
	}
	return config
}

// TestReconstituirNaoDisparaTrabalho é o risco que a recuperação automática
// cria, cortado na raiz: o agente acorda esperando, e nenhum byte é escrito no
// teclado dele.
func TestReconstituirNaoDisparaTrabalho(t *testing.T) {
	dirEstado := t.TempDir()
	dirProjeto := t.TempDir()
	registro := filepath.Join(t.TempDir(), "recebido.txt")
	programa := agenteQueAnotaOQueRecebe(t, dirProjeto, registro)

	primeiro := Novo(dirEstado, configComAgenteDeMentira(programa))
	if err := primeiro.Iniciar(); err != nil {
		t.Fatalf("iniciar: %v", err)
	}
	if _, err := primeiro.Criar(protocolo.Criar{Caminho: dirProjeto, Tipo: "claude", Nome: "refatora auth"}); err != nil {
		t.Fatalf("criar: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	primeiro.Encerrar()

	// Zera o que a primeira subida escreveu, para medir só a reconstituição.
	if err := os.WriteFile(registro, nil, 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	segundo := Novo(dirEstado, configComAgenteDeMentira(programa))
	if err := segundo.Iniciar(); err != nil {
		t.Fatalf("reiniciar: %v", err)
	}
	defer segundo.Encerrar()
	time.Sleep(1500 * time.Millisecond)

	recebido, err := os.ReadFile(registro)
	if err != nil {
		t.Fatalf("ler o que o agente recebeu: %v", err)
	}
	if len(recebido) > 0 {
		t.Fatalf("a reconstituição escreveu %d byte(s) no teclado do agente: %q", len(recebido), recebido)
	}

	// E a grade voltou inteira.
	retrato := segundo.Retrato()
	if len(retrato.Projetos) != 1 || len(retrato.Projetos[0].Celulas) != 1 {
		t.Fatalf("a grade não voltou: %#v", retrato)
	}
	if retrato.Projetos[0].Celulas[0].Nome != "refatora auth" {
		t.Fatalf("a célula voltou com outro nome: %q", retrato.Projetos[0].Celulas[0].Nome)
	}
}

// TestReconstituirReataAMesmaConversa — a identidade da conversa sobrevive à
// queda, que é o que faz o agente voltar de onde parou.
func TestReconstituirReataAMesmaConversa(t *testing.T) {
	dirEstado := t.TempDir()
	dirProjeto := t.TempDir()
	programa := agenteQueAnotaOQueRecebe(t, dirProjeto, filepath.Join(t.TempDir(), "lixo.txt"))

	primeiro := Novo(dirEstado, configComAgenteDeMentira(programa))
	if err := primeiro.Iniciar(); err != nil {
		t.Fatalf("iniciar: %v", err)
	}
	if _, err := primeiro.Criar(protocolo.Criar{Caminho: dirProjeto, Tipo: "claude"}); err != nil {
		t.Fatalf("criar: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	primeiro.Encerrar()

	salvo, err := Carregar(CaminhoEstado(dirEstado))
	if err != nil {
		t.Fatalf("carregar: %v", err)
	}
	conversa := salvo.Projetos[0].Celulas[0].Conversas["claude"]
	if conversa == "" {
		t.Fatalf("a identidade da conversa não foi guardada: %#v", salvo.Projetos[0].Celulas[0])
	}

	segundo := Novo(dirEstado, configComAgenteDeMentira(programa))
	if err := segundo.Iniciar(); err != nil {
		t.Fatalf("reiniciar: %v", err)
	}
	defer segundo.Encerrar()
	time.Sleep(300 * time.Millisecond)

	depois, err := Carregar(CaminhoEstado(dirEstado))
	if err != nil {
		t.Fatalf("carregar: %v", err)
	}
	if depois.Projetos[0].Celulas[0].Conversas["claude"] != conversa {
		t.Fatalf("a conversa mudou na reconstituição: %q → %q", conversa, depois.Projetos[0].Celulas[0].Conversas["claude"])
	}
}

// TestDockerNaoSobeSozinho — subir stack é decisão do usuário, inclusive
// depois de uma queda da WSL.
func TestDockerNaoSobeSozinho(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("sem docker nesta máquina")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("o docker não está respondendo")
	}

	dirEstado := t.TempDir()
	dirProjeto := t.TempDir()
	compose := filepath.Join(dirProjeto, "docker-compose.yml")
	corpo := "services:\n  web:\n    image: nginx:alpine\n    command: [\"sh\", \"-c\", \"sleep 600\"]\n"
	if err := os.WriteFile(compose, []byte(corpo), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "compose", "--file", compose, "stop").Run()
		_ = exec.Command("docker", "compose", "--file", compose, "rm", "-f").Run()
	})

	primeiro := Novo(dirEstado, configuracaoDeTeste())
	if err := primeiro.Iniciar(); err != nil {
		t.Fatalf("iniciar: %v", err)
	}
	if _, err := primeiro.Criar(protocolo.Criar{Caminho: dirProjeto, Tipo: "bash"}); err != nil {
		t.Fatalf("criar: %v", err)
	}
	primeiro.Encerrar()

	segundo := Novo(dirEstado, configuracaoDeTeste())
	if err := segundo.Iniciar(); err != nil {
		t.Fatalf("reiniciar: %v", err)
	}
	go segundo.Vigiar()
	defer segundo.Encerrar()
	time.Sleep(2 * time.Second)

	servicos, err := docker.Servicos(dirProjeto, compose)
	if err != nil {
		t.Fatalf("ler a stack: %v", err)
	}
	for _, servico := range servicos {
		if strings.HasPrefix(servico.Estado, "up") {
			t.Fatalf("a reconstituição subiu a stack sozinha: %#v", servico)
		}
	}

	// E o projeto reconhece que tem compose, para o painel existir.
	retrato := segundo.Retrato()
	if !retrato.Projetos[0].TemCompose {
		t.Fatal("o projeto tem compose na raiz e o motor não percebeu")
	}
}

// TestReconstituirCelulaDeTipoDesconhecidoNaoDerrubaOMotor — estado escrito por
// uma versão futura não pode impedir a grade de voltar.
func TestReconstituirCelulaDeTipoDesconhecidoNaoDerrubaOMotor(t *testing.T) {
	dirEstado := t.TempDir()
	dirProjeto := t.TempDir()
	estado := EstadoDesejado{Projetos: []ProjetoSalvo{{
		ID: "p1", Caminho: dirProjeto,
		Celulas: []CelulaSalva{
			{ID: "c1", Tipo: "planilha", Nome: "do futuro"},
			{ID: "c2", Tipo: "bash", Nome: "testes"},
		},
	}}}
	if err := Salvar(CaminhoEstado(dirEstado), estado); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	m := Novo(dirEstado, configuracaoDeTeste())
	if err := m.Iniciar(); err != nil {
		t.Fatalf("iniciar: %v", err)
	}
	defer m.Encerrar()

	retrato := m.Retrato()
	if len(retrato.Projetos) != 1 || len(retrato.Projetos[0].Celulas) != 2 {
		t.Fatalf("a grade devia voltar inteira, com a célula desconhecida parada: %#v", retrato)
	}
	if retrato.Aviso == "" {
		t.Error("o motor devia avisar que não soube subir uma célula")
	}
}
