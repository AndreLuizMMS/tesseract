package motor

import (
	"os"
	"os/exec"
	"strings"
	"time"
)

// prazoDoLogin limita a leitura do ambiente do shell: se o rc do usuário
// travar, o motor sobe assim mesmo.
const prazoDoLogin = 5 * time.Second

// HerdarCaminhoDoLogin põe no motor o PATH que o usuário tem no terminal dele.
//
// Sem isso, o motor rodando como serviço enxerga só o PATH curto do systemd —
// e os agentes instalados na conta (claude, cursor-agent) e as ferramentas que
// eles chamam (node, pnpm) simplesmente não existiriam. Roda uma vez, na
// subida, e falha em silêncio: um PATH curto é pior, não fatal.
func HerdarCaminhoDoLogin() {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	comando := exec.Command(shell, "-lic", "printf '%s' \"$PATH\"")
	comando.Env = append(os.Environ(), "TESSERACT_LENDO_AMBIENTE=1")
	pronto := make(chan []byte, 1)
	go func() {
		saida, err := comando.Output()
		if err != nil {
			pronto <- nil
			return
		}
		pronto <- saida
	}()

	select {
	case saida := <-pronto:
		if caminho := ultimaLinha(string(saida)); strings.Contains(caminho, "/") {
			os.Setenv("PATH", caminho)
		}
	case <-time.After(prazoDoLogin):
		if comando.Process != nil {
			_ = comando.Process.Kill()
		}
	}
}

// ultimaLinha é o que sobra depois do que o rc do usuário resolveu imprimir.
func ultimaLinha(saida string) string {
	linhas := strings.Split(strings.TrimSpace(saida), "\n")
	return strings.TrimSpace(linhas[len(linhas)-1])
}
