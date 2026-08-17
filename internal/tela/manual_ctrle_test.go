package tela

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/andreluiz/tesseract/internal/motor"
	"github.com/andreluiz/tesseract/internal/protocolo"
)

// TestManualCtrlEAbreIDE é a validação de ponta a ponta do atalho: tecla ctrl+e
// na tela real, socket real, motor real. A IDE é um espião com a mesma anatomia
// da instalação remota do Cursor, e o teste cobra dele o caminho do projeto e o
// socket da janela viva — que é exatamente o que abre a pasta na janela aberta.
//
// Só roda com MANUAL=1: sobe motor e célula de verdade.
//
//	MANUAL=1 go test ./internal/tela -run TestManualCtrlEAbreIDE -v
//
// Para mirar na IDE de verdade, com uma janela do Cursor aberta:
//
//	MANUAL=1 EDITOR_DO_TESTE=cursor SOCKET_DA_JANELA=/run/user/1000/vscode-ipc-....sock ...
func TestManualCtrlEAbreIDE(t *testing.T) {
	if os.Getenv("MANUAL") == "" {
		t.Skip("manual")
	}

	// Um XDG_RUNTIME_DIR só deste motor, para não brigar com o socket do motor
	// que já roda na máquina. O socket da janela da IDE entra aqui dentro.
	execucao := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", execucao)

	editor := os.Getenv("EDITOR_DO_TESTE")
	registro := filepath.Join(t.TempDir(), "chamada.txt")
	if editor == "" {
		editor = "espiao"
		t.Setenv("HOME", t.TempDir())
		criarIDEEspia(t, editor, registro)
	}

	if vivo := os.Getenv("SOCKET_DA_JANELA"); vivo != "" {
		if err := os.Symlink(vivo, filepath.Join(execucao, "vscode-ipc-teste.sock")); err != nil {
			t.Fatal(err)
		}
	} else {
		escuta, err := net.Listen("unix", filepath.Join(execucao, "vscode-ipc-teste.sock"))
		if err != nil {
			t.Fatal(err)
		}
		defer escuta.Close()
	}

	projeto := os.Getenv("PROJETO_DO_TESTE")
	if projeto == "" {
		projeto = t.TempDir()
	}

	config := motor.ConfiguracaoPadrao()
	config.Editor = editor
	m := motor.Novo(t.TempDir(), config)
	if err := m.Iniciar(); err != nil {
		t.Fatal(err)
	}
	defer m.Encerrar()

	servidor, err := motor.Servir(m, motor.CaminhoSocket())
	if err != nil {
		t.Fatal(err)
	}
	defer servidor.Fechar()

	if _, err := m.Criar(protocolo.Criar{Caminho: projeto, Tipo: "bash"}); err != nil {
		t.Fatal(err)
	}

	cliente, err := Conectar(motor.CaminhoSocket())
	if err != nil {
		t.Fatal(err)
	}
	defer cliente.Fechar()

	modelo := NovoModelo(cliente, m.Retrato())
	modelo.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})

	// O pedido atravessa o socket; o erro, se houver, volta pelo mesmo caminho.
	time.Sleep(2 * time.Second)
	if modelo.erro != "" {
		t.Fatalf("ctrl+e reclamou: %s", modelo.erro)
	}

	if os.Getenv("EDITOR_DO_TESTE") != "" {
		t.Log("ctrl+e foi até o fim sem erro — confira a janela da IDE")
		return
	}
	chamada, err := os.ReadFile(registro)
	if err != nil {
		t.Fatalf("a IDE não foi chamada: %v", err)
	}
	texto := string(chamada)
	if !strings.Contains(texto, "PASTA="+projeto) {
		t.Fatalf("a IDE não recebeu a pasta do projeto: %s", texto)
	}
	if !strings.Contains(texto, "JANELA="+filepath.Join(execucao, "vscode-ipc-teste.sock")) {
		t.Fatalf("a IDE não recebeu o socket da janela viva: %s", texto)
	}
	t.Logf("ctrl+e chegou na IDE: %s", strings.TrimSpace(texto))
}

// criarIDEEspia monta a mesma anatomia da instalação remota da IDE
// (~/.<nome>-server/bin/<versão>/bin/remote-cli/<nome>) com um script no lugar
// do CLI, para o teste ler o que o motor mandou.
func criarIDEEspia(t *testing.T, nome, registro string) {
	t.Helper()
	casa, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(casa, "."+nome+"-server", "bin", "abc123", "bin", "remote-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		"printf 'PASTA=%s\\nJANELA=%s\\n' \"$1\" \"$VSCODE_IPC_HOOK_CLI\" > " + registro + "\n"
	if err := os.WriteFile(filepath.Join(dir, nome), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
