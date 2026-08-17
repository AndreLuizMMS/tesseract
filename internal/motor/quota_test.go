package motor

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// prepararQuota escreve o arquivo que o statusline do usuário deixa, numa casa
// de mentira.
func prepararQuota(t *testing.T, nome string, percentual int, viraEm time.Time, idade time.Duration) {
	t.Helper()
	casa := t.TempDir()
	t.Setenv("HOME", casa)
	if err := os.MkdirAll(filepath.Join(casa, ".claude"), 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	caminho := filepath.Join(casa, ".claude", nome)
	corpo := `{"used_percentage":` + strconv.Itoa(percentual) + `,"resets_at":` + strconv.FormatInt(viraEm.Unix(), 10) + `}`
	if err := os.WriteFile(caminho, []byte(corpo), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if idade > 0 {
		velho := time.Now().Add(-idade)
		if err := os.Chtimes(caminho, velho, velho); err != nil {
			t.Fatalf("envelhecer: %v", err)
		}
	}
}

// TestQuotaLeOArquivoDoStatusline — o número aparece com o tempo até a janela
// virar.
func TestQuotaLeOArquivoDoStatusline(t *testing.T) {
	prepararQuota(t, "tesseract-quota.json", 59, time.Now().Add(2*time.Hour+47*time.Minute), 0)

	quota := LerQuota()
	if quota == nil {
		t.Fatal("o badge devia aparecer")
	}
	if quota.Percentual != 59 {
		t.Fatalf("percentual veio %d", quota.Percentual)
	}
	if quota.Vira != "2:46" && quota.Vira != "2:47" {
		t.Fatalf("o tempo até virar veio %q", quota.Vira)
	}
}

// TestQuotaSomeQuandoOArquivoEstaVelho — o Claude Code só entrega esse número
// enquanto uma sessão desenha o statusline, então arquivo velho não vale nada.
func TestQuotaSomeQuandoOArquivoEstaVelho(t *testing.T) {
	prepararQuota(t, "tesseract-quota.json", 42, time.Now().Add(time.Hour), quotaVelhaDepoisDe+time.Minute)
	if quota := LerQuota(); quota != nil {
		t.Fatalf("badge velho devia sumir, veio %#v", quota)
	}
}

// TestQuotaSemArquivoNaoQuebraNada.
func TestQuotaSemArquivoNaoQuebraNada(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if quota := LerQuota(); quota != nil {
		t.Fatalf("sem arquivo não há badge, veio %#v", quota)
	}
}

// TestQuotaAceitaOArquivoAntigo — quem já tinha o statusline do fork continua
// vendo o badge.
func TestQuotaAceitaOArquivoAntigo(t *testing.T) {
	prepararQuota(t, "squad-quota.json", 20, time.Now().Add(30*time.Minute), 0)
	quota := LerQuota()
	if quota == nil || quota.Percentual != 20 {
		t.Fatalf("o arquivo antigo devia servir, veio %#v", quota)
	}
}

// TestQuotaAcimaDe80MudaDeFaixa — a faixa é o que a barra usa para trocar de
// cor.
func TestQuotaAcimaDe80MudaDeFaixa(t *testing.T) {
	prepararQuota(t, "tesseract-quota.json", 87, time.Now().Add(time.Hour), 0)
	quota := LerQuota()
	if quota == nil {
		t.Fatal("o badge devia aparecer")
	}
	if quota.Percentual < 80 {
		t.Fatalf("percentual veio %d", quota.Percentual)
	}
}

// TestQuotaNaoPassaDeCem — número estranho no arquivo não vira badge estranho.
func TestQuotaNaoPassaDeCem(t *testing.T) {
	prepararQuota(t, "tesseract-quota.json", 250, time.Now().Add(time.Hour), 0)
	if quota := LerQuota(); quota == nil || quota.Percentual != 100 {
		t.Fatalf("percentual devia parar em 100, veio %#v", quota)
	}
}

// TestQuotaComArquivoIlegivelNaoQuebra.
func TestQuotaComArquivoIlegivelNaoQuebra(t *testing.T) {
	casa := t.TempDir()
	t.Setenv("HOME", casa)
	if err := os.MkdirAll(filepath.Join(casa, ".claude"), 0o755); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if err := os.WriteFile(filepath.Join(casa, ".claude", "tesseract-quota.json"), []byte("isso não é json"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if quota := LerQuota(); quota != nil {
		t.Fatalf("arquivo ilegível não vira badge: %#v", quota)
	}
}
