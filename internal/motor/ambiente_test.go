package motor

import (
	"os"
	"strings"
	"testing"
)

// TestHerdarCaminhoDoLogin — o motor precisa enxergar as ferramentas que o
// usuário tem no terminal dele, não só o PATH curto do serviço.
func TestHerdarCaminhoDoLogin(t *testing.T) {
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })

	os.Setenv("PATH", "/apenas/isso")
	HerdarCaminhoDoLogin()

	depois := os.Getenv("PATH")
	if depois == "/apenas/isso" {
		t.Skip("o shell desta máquina não devolveu PATH nenhum")
	}
	if !strings.Contains(depois, "/apenas/isso") {
		t.Fatalf("o PATH do serviço devia continuar lá dentro: %q", depois)
	}
	if len(strings.Split(depois, ":")) < 3 {
		t.Fatalf("o PATH herdado veio curto demais: %q", depois)
	}
}

// TestUltimaLinha — o rc do usuário pode imprimir coisas antes.
func TestUltimaLinha(t *testing.T) {
	if veio := ultimaLinha("banner do rc\n/usr/bin:/bin\n"); veio != "/usr/bin:/bin" {
		t.Fatalf("veio %q", veio)
	}
	if veio := ultimaLinha("  /usr/bin  "); veio != "/usr/bin" {
		t.Fatalf("veio %q", veio)
	}
}
