package celula

import "os"

func init() {
	Registrar(Descritor{Tipo: "bash", Ordem: 30}, func() Celula { return &Bash{} })
}

// Bash é um shell rodando na raiz do projeto.
type Bash struct {
	Processo
}

func (b *Bash) Nascer(cfg Config) error {
	return b.iniciar(cfg, shellDoUsuario(), nil, ambienteDeTerminal())
}

// shellDoUsuario respeita a escolha da conta e cai no bash quando não há.
func shellDoUsuario() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/bash"
}

// ambienteDeTerminal é o ambiente do motor com o terminal declarado, porque a
// tela interna emula um xterm colorido.
func ambienteDeTerminal() []string {
	ambiente := make([]string, 0, len(os.Environ())+2)
	for _, par := range os.Environ() {
		switch {
		case len(par) > 5 && par[:5] == "TERM=":
		case len(par) > 10 && par[:10] == "COLORTERM=":
		default:
			ambiente = append(ambiente, par)
		}
	}
	return append(ambiente, "TERM=xterm-256color", "COLORTERM=truecolor")
}
