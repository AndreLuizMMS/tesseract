package motor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/protocolo"
)

func prepararPastas(t *testing.T) string {
	t.Helper()
	raiz := t.TempDir()
	for _, nome := range []string{"cortz-web", "cortz-api", "doxar", ".escondida"} {
		if err := os.Mkdir(filepath.Join(raiz, nome), 0o755); err != nil {
			t.Fatalf("preparar: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(raiz, "cortz-anotacoes.md"), []byte("#"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	return raiz
}

func TestCompletarUmCandidatoVaiAteOFim(t *testing.T) {
	raiz := prepararPastas(t)
	veio := Completar(protocolo.Completar{Caminho: filepath.Join(raiz, "dox"), SoDiretorio: true})
	if veio.Quantidade != 1 {
		t.Fatalf("esperava 1 candidato, veio %d", veio.Quantidade)
	}
	if veio.Caminho != filepath.Join(raiz, "doxar")+"/" {
		t.Fatalf("caminho completado veio %q", veio.Caminho)
	}
}

func TestCompletarVariosParaNoPrefixoComum(t *testing.T) {
	raiz := prepararPastas(t)
	veio := Completar(protocolo.Completar{Caminho: filepath.Join(raiz, "cor"), SoDiretorio: true})
	if veio.Quantidade != 2 {
		t.Fatalf("esperava 2 pastas casando, veio %d", veio.Quantidade)
	}
	if veio.Caminho != filepath.Join(raiz, "cortz-") {
		t.Fatalf("devia parar no prefixo comum, veio %q", veio.Caminho)
	}
}

func TestCompletarSoDiretorioIgnoraArquivo(t *testing.T) {
	raiz := prepararPastas(t)
	comArquivo := Completar(protocolo.Completar{Caminho: filepath.Join(raiz, "cortz-a")})
	if comArquivo.Quantidade != 2 {
		t.Fatalf("sem filtro, o arquivo e a pasta casam: veio %d", comArquivo.Quantidade)
	}
	soPasta := Completar(protocolo.Completar{Caminho: filepath.Join(raiz, "cortz-a"), SoDiretorio: true})
	if soPasta.Quantidade != 1 || !strings.HasSuffix(soPasta.Caminho, "cortz-api/") {
		t.Fatalf("com filtro só a pasta casa: %#v", soPasta)
	}
}

func TestCompletarSemCandidatoDevolveOQueFoiDigitado(t *testing.T) {
	raiz := prepararPastas(t)
	digitado := filepath.Join(raiz, "zzz")
	veio := Completar(protocolo.Completar{Caminho: digitado, SoDiretorio: true})
	if veio.Quantidade != 0 || veio.Caminho != digitado {
		t.Fatalf("sem candidato o campo continua como está: %#v", veio)
	}
}

func TestCompletarEscondidaSoQuandoPedida(t *testing.T) {
	raiz := prepararPastas(t)
	tudo := Completar(protocolo.Completar{Caminho: raiz + "/", SoDiretorio: true})
	if tudo.Quantidade != 3 {
		t.Fatalf("pasta escondida não devia aparecer sozinha: veio %d", tudo.Quantidade)
	}
	pedida := Completar(protocolo.Completar{Caminho: filepath.Join(raiz, "."), SoDiretorio: true})
	if pedida.Quantidade != 1 {
		t.Fatalf("pedindo o ponto, a escondida casa: veio %d", pedida.Quantidade)
	}
}
