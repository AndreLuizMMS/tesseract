package tela

import (
	"os"
	"testing"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

// TestVitrine não é teste: despeja as telas pintadas para virarem imagem da
// vitrine do repositório. Só roda com VITRINE apontando uma pasta. Temporário.
func TestVitrine(t *testing.T) {
	pasta := os.Getenv("VITRINE")
	if pasta == "" {
		t.Skip("sem VITRINE")
	}

	estado := gradeDeTeste()
	estado.Quota = &protocolo.Quota{Percentual: 21, Vira: "3:12"}

	telas := map[string]string{
		"mosaico": Desenhar(estado, Foco{Projeto: 0, Celula: 0}, teclado.Navegar, 118, 24, ""),
		"digitar": Desenhar(estado, Foco{Projeto: 0, Celula: 0}, teclado.Digitar, 118, 24, ""),
	}
	for nome, desenho := range telas {
		if err := os.WriteFile(pasta+"/"+nome+".ansi", []byte(desenho), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
