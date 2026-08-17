package motor

import (
	"strings"
	"testing"
)

// TestComandoDeNotificacaoRespeitaOCustomizado — quem apontou outro notificador
// manda nele.
func TestComandoDeNotificacaoRespeitaOCustomizado(t *testing.T) {
	comando := comandoDeNotificacao("/usr/bin/meu-aviso --urgente", "Tesseract", "claude · cortz respondeu")
	if comando == nil {
		t.Fatal("o comando customizado devia ser usado")
	}
	if comando.Path != "/usr/bin/meu-aviso" {
		t.Fatalf("programa veio %q", comando.Path)
	}
	argumentos := comando.Args
	if len(argumentos) < 2 {
		t.Fatalf("argumentos vieram %v", argumentos)
	}
	// O título e o corpo são sempre os dois últimos argumentos.
	if argumentos[len(argumentos)-2] != "Tesseract" || argumentos[len(argumentos)-1] != "claude · cortz respondeu" {
		t.Fatalf("título e corpo não foram os últimos: %v", argumentos)
	}
	if argumentos[1] != "--urgente" {
		t.Fatalf("a bandeira do usuário se perdeu: %v", argumentos)
	}
}

// TestNotificacaoSempreTemUmCaminho — nesta máquina (WSL) sempre há como
// avisar, nem que seja pelo PowerShell.
func TestNotificacaoSempreTemUmCaminho(t *testing.T) {
	comando := comandoDeNotificacao("", "Tesseract", "claude · cortz respondeu")
	if comando == nil {
		t.Skip("máquina sem nenhum notificador")
	}
	if !strings.Contains(strings.Join(comando.Args, " "), "cortz") {
		t.Fatalf("o corpo do aviso não chegou ao comando: %v", comando.Args)
	}
}

// TestToastEscapaAspas — nome de célula com apóstrofo não quebra o script do
// PowerShell.
func TestToastEscapaAspas(t *testing.T) {
	script := scriptDeToast("Tesseract", "o agente d'água respondeu")
	if strings.Contains(script, "d'água") {
		t.Fatalf("o apóstrofo devia ter sido escapado: %s", script)
	}
	if !strings.Contains(script, "d''água") {
		t.Fatalf("escape errado: %s", script)
	}
	if !strings.Contains(script, "ShowBalloonTip") {
		t.Fatalf("o script não mostra nada: %s", script)
	}
}

// TestAvisoDesligadoNaoChamaNada — os dois avisos são desligáveis, e o motor
// respeita isso.
func TestAvisoDesligadoNaoChamaNada(t *testing.T) {
	m := Novo(t.TempDir(), Configuracao{Som: false, Notificar: false})
	// Sem processo nenhum para vigiar, isto só não pode explodir.
	m.anunciar("claude · cortz respondeu")
}
