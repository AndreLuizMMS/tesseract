package motor

import (
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/protocolo"
)

// TestGradeParadaNaoGeraQuadro prende o silêncio do motor: com o vigia de pé e
// nenhuma célula produzindo nada, a versão não sobe — e sem versão nova nenhuma
// tela recebe retrato para redesenhar.
func TestGradeParadaNaoGeraQuadro(t *testing.T) {
	m := motorDeTeste(t)
	if _, err := m.Criar(protocolo.Criar{Caminho: t.TempDir(), Tipo: "bash"}); err != nil {
		t.Fatalf("criar: %v", err)
	}
	go m.Vigiar()

	// Espera o shell terminar de escrever o prompt: enquanto ele fala, subir de
	// versão é o certo. Três janelas caladas seguidas é o bastante para saber
	// que ele parou, e não que só respirou no meio.
	caladas := 0
	for range 40 {
		antes := m.Versao()
		time.Sleep(200 * time.Millisecond)
		if antes == m.Versao() {
			caladas++
		} else {
			caladas = 0
		}
		if caladas == 3 {
			break
		}
	}
	if caladas < 3 {
		t.Fatal("o shell nunca sossegou")
	}

	antes := m.Versao()
	time.Sleep(1500 * time.Millisecond)
	if depois := m.Versao(); depois != antes {
		t.Fatalf("grade parada gerou %d quadro(s) à toa", depois-antes)
	}
}
