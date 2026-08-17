package motor

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/protocolo"
)

// TestPromptChegaNaCelulaSemEntrarNela — a tecla de prompt manda trabalho para
// o agente sem o usuário entrar na célula.
func TestPromptChegaNaCelulaSemEntrarNela(t *testing.T) {
	dirEstado := t.TempDir()
	dirProjeto := t.TempDir()
	registro := filepath.Join(t.TempDir(), "recebido.txt")
	programa := agenteQueAnotaOQueRecebe(t, dirProjeto, registro)

	m := Novo(dirEstado, configComAgenteDeMentira(programa))
	if err := m.Iniciar(); err != nil {
		t.Fatalf("iniciar: %v", err)
	}
	defer m.Encerrar()

	id, err := m.Criar(protocolo.Criar{Caminho: dirProjeto, Tipo: "claude"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	if err := m.Prompt(id, "cobre o menu mobile também"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	// O texto inteiro e o enter no fim: sem o enter, o turno não começa. O
	// terminal entrega o enter como fim de linha.
	if !esperarPor(t, 5*time.Second, func() bool {
		conteudo, _ := os.ReadFile(registro)
		return strings.Contains(string(conteudo), "cobre o menu mobile também\n")
	}) {
		conteudo, _ := os.ReadFile(registro)
		t.Fatalf("o prompt não chegou inteiro ao agente, recebido: %q", conteudo)
	}
}

// TestPromptEmCelulaQueNaoRecebeFalhaClaro — shell não recebe prompt.
func TestPromptEmCelulaQueNaoRecebeFalhaClaro(t *testing.T) {
	m := motorDeTeste(t)
	id, err := m.Criar(protocolo.Criar{Caminho: t.TempDir(), Tipo: "bash"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	err = m.Prompt(id, "oi")
	if err == nil {
		t.Fatal("célula sem prompt devia recusar")
	}
	if !strings.Contains(err.Error(), "prompt") {
		t.Fatalf("mensagem pouco clara: %v", err)
	}
}

// TestRenomearAutomaticoMandaComandoPuro — o Ctrl+R não pergunta nome: manda o
// /rename puro pro agente escolher, e arma a espera de adoção.
func TestRenomearAutomaticoMandaComandoPuro(t *testing.T) {
	dirEstado := t.TempDir()
	dirProjeto := t.TempDir()
	registro := filepath.Join(t.TempDir(), "recebido.txt")
	programa := agenteQueAnotaOQueRecebe(t, dirProjeto, registro)

	m := Novo(dirEstado, configComAgenteDeMentira(programa))
	if err := m.Iniciar(); err != nil {
		t.Fatalf("iniciar: %v", err)
	}
	defer m.Encerrar()

	id, err := m.Criar(protocolo.Criar{Caminho: dirProjeto, Tipo: "claude"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	nomeAntes := m.Retrato().Projetos[0].Celulas[0].Nome
	if err := m.RenomearAutomatico(id); err != nil {
		t.Fatalf("renomear automático: %v", err)
	}

	// O comando puro, sem nome nenhum, chegou ao agente…
	if !esperarPor(t, 3*time.Second, func() bool {
		conteudo, _ := os.ReadFile(registro)
		return strings.Contains(string(conteudo), "/rename\n")
	}) {
		conteudo, _ := os.ReadFile(registro)
		t.Fatalf("o comando não chegou puro, recebido: %q", conteudo)
	}
	// …e o rótulo não muda até o agente responder com um nome de verdade.
	if m.Retrato().Projetos[0].Celulas[0].Nome != nomeAntes {
		t.Fatal("o rótulo não podia mudar antes do agente responder")
	}
}

// TestRenomearAutomaticoEmCelulaSemConversaFalhaClaro — bash, logs e md não
// têm conversa com nome, então não há o que pedir.
func TestRenomearAutomaticoEmCelulaSemConversaFalhaClaro(t *testing.T) {
	m := motorDeTeste(t)
	id, err := m.Criar(protocolo.Criar{Caminho: t.TempDir(), Tipo: "bash"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	if err := m.RenomearAutomatico(id); err == nil {
		t.Fatal("célula sem conversa devia recusar")
	}
}

// TestAdotarNomeAvisaQuandoAConversaNaoTemNome — em vez de renomear para vazio.
func TestAdotarNomeAvisaQuandoAConversaNaoTemNome(t *testing.T) {
	dirEstado := t.TempDir()
	dirProjeto := t.TempDir()
	programa := agenteQueAnotaOQueRecebe(t, dirProjeto, filepath.Join(t.TempDir(), "lixo.txt"))

	m := Novo(dirEstado, configComAgenteDeMentira(programa))
	if err := m.Iniciar(); err != nil {
		t.Fatalf("iniciar: %v", err)
	}
	defer m.Encerrar()

	id, err := m.Criar(protocolo.Criar{Caminho: dirProjeto, Tipo: "claude"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	nomeAntes := m.Retrato().Projetos[0].Celulas[0].Nome

	err = m.AdotarNomeDoAgente(id)
	if err == nil {
		t.Fatal("sem nome de conversa, adotar tinha que avisar")
	}
	if m.Retrato().Projetos[0].Celulas[0].Nome != nomeAntes {
		t.Fatal("o nome da célula não podia mudar")
	}
}

// TestRetomarSobeCelulaCaida — nada reinicia sozinho; a tecla de retomar é que
// sobe de novo.
func TestRetomarSobeCelulaCaida(t *testing.T) {
	m := motorDeTeste(t)
	id, err := m.Criar(protocolo.Criar{Caminho: t.TempDir(), Tipo: "bash"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	colarEEntrar(t, m, id, "exit")
	if !esperarPor(t, 5*time.Second, func() bool {
		return m.Retrato().Projetos[0].Celulas[0].Estado == "caiu"
	}) {
		t.Fatalf("a célula devia ter caído, está %q", m.Retrato().Projetos[0].Celulas[0].Estado)
	}
	// Fica caída: nada reinicia sozinho.
	time.Sleep(1500 * time.Millisecond)
	if m.Retrato().Projetos[0].Celulas[0].Estado != "caiu" {
		t.Fatal("célula caída não pode reiniciar sozinha")
	}

	if err := m.Retomar(id); err != nil {
		t.Fatalf("retomar: %v", err)
	}
	if !esperarPor(t, 5*time.Second, func() bool {
		return m.Retrato().Projetos[0].Celulas[0].Estado == "trabalhando"
	}) {
		t.Fatalf("depois de retomar, a célula está %q", m.Retrato().Projetos[0].Celulas[0].Estado)
	}
}

// TestRetomarCelulaViva Recusa — retomar o que está rodando não faz sentido.
func TestRetomarCelulaVivaRecusa(t *testing.T) {
	m := motorDeTeste(t)
	id, err := m.Criar(protocolo.Criar{Caminho: t.TempDir(), Tipo: "bash"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	if err := m.Retomar(id); err == nil {
		t.Fatal("retomar uma célula viva devia avisar")
	}
}

// TestBuscarNoHistoricoDaCelula — a busca acha o que a célula escreveu.
func TestBuscarNoHistoricoDaCelula(t *testing.T) {
	m := motorDeTeste(t)
	id, err := m.Criar(protocolo.Criar{Caminho: t.TempDir(), Tipo: "bash"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	colarEEntrar(t, m, id, "echo agulha-no-historico")
	if !esperarPor(t, 3*time.Second, func() bool {
		achados, _ := m.Buscar(id, "agulha-no-historico")
		return len(achados) > 0
	}) {
		t.Fatal("a busca não achou o que a célula escreveu")
	}

	achados, err := m.Buscar(id, "agulha-no-historico")
	if err != nil {
		t.Fatalf("buscar: %v", err)
	}
	if achados[0].Linha < 1 {
		t.Fatalf("a ocorrência veio sem número de linha: %#v", achados[0])
	}
	if err := m.IrParaLinha(id, achados[0].Linha); err != nil {
		t.Fatalf("ir para a linha: %v", err)
	}
}

// TestEditorSemConfiguracaoAvisa.
func TestEditorSemConfiguracaoAvisa(t *testing.T) {
	config := configuracaoDeTeste()
	config.Editor = ""
	m := Novo(t.TempDir(), config)
	if err := m.Iniciar(); err != nil {
		t.Fatalf("iniciar: %v", err)
	}
	defer m.Encerrar()

	if _, err := m.Criar(protocolo.Criar{Caminho: t.TempDir(), Tipo: "bash"}); err != nil {
		t.Fatalf("criar: %v", err)
	}
	idProjeto := m.Retrato().Projetos[0].ID
	if err := m.AbrirNoEditor(idProjeto); err == nil {
		t.Fatal("sem editor configurado, tinha que avisar")
	}
}

// TestDockerRecusaProjetoSemCompose — projeto sem compose não tem painel.
func TestDockerRecusaProjetoSemCompose(t *testing.T) {
	m := motorDeTeste(t)
	if _, err := m.Criar(protocolo.Criar{Caminho: t.TempDir(), Tipo: "bash"}); err != nil {
		t.Fatalf("criar: %v", err)
	}
	idProjeto := m.Retrato().Projetos[0].ID
	if _, err := m.Docker(protocolo.Docker{Projeto: idProjeto, Acao: "listar"}); err == nil {
		t.Fatal("projeto sem compose não tem painel")
	}
}

// TestDockerRecusaAcaoDesconhecida — a lista de ações é fechada, e é assim que
// nada destrutivo entra.
func TestDockerRecusaAcaoDesconhecida(t *testing.T) {
	m := motorDeTeste(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if _, err := m.Criar(protocolo.Criar{Caminho: dir, Tipo: "bash"}); err != nil {
		t.Fatalf("criar: %v", err)
	}
	idProjeto := m.Retrato().Projetos[0].ID

	resposta, err := m.Docker(protocolo.Docker{Projeto: idProjeto, Acao: "down -v"})
	if err == nil && resposta.Erro == "" {
		t.Fatal("ação destrutiva não pode passar")
	}
}

// TestJanelaVivaIgnoraSocketMorto garante que o socket largado por uma janela
// que já fechou não é escolhido: só entra o que ainda aceita conexão.
func TestJanelaVivaIgnoraSocketMorto(t *testing.T) {
	dir := t.TempDir()

	morto := filepath.Join(dir, "vscode-ipc-morto.sock")
	if err := os.WriteFile(morto, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	vivo := filepath.Join(dir, "vscode-ipc-vivo.sock")
	escuta, err := net.Listen("unix", vivo)
	if err != nil {
		t.Fatal(err)
	}
	defer escuta.Close()

	if achado := janelaViva(dir); achado != vivo {
		t.Fatalf("esperava o socket vivo %s, veio %q", vivo, achado)
	}

	if achado := janelaViva(t.TempDir()); achado != "" {
		t.Fatalf("sem janela aberta não há socket, veio %q", achado)
	}
}
