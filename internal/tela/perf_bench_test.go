package tela

import (
	"strconv"
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

func estadoDeCarga(projetos, celulas, colunas, linhas int) protocolo.Estado {
	var e protocolo.Estado
	for p := range projetos {
		proj := protocolo.Projeto{
			ID: "p" + strconv.Itoa(p), Caminho: "/home/user/projeto" + strconv.Itoa(p),
			Nome: "projeto" + strconv.Itoa(p), Cor: p, TemCompose: true, Docker: "3/4",
		}
		for c := range celulas {
			cel := protocolo.Celula{
				ID: "c" + strconv.Itoa(p) + "-" + strconv.Itoa(c), Tipo: "sessao",
				Nome: "celula " + strconv.Itoa(c), Estado: "trabalhando", AoVivo: true,
			}
			for l := range linhas {
				cel.Linhas = append(cel.Linhas,
					"\x1b[38;2;120;200;180m"+strings.Repeat("x", colunas/2)+
						"\x1b[0m \x1b[1mlinha "+strconv.Itoa(l)+"\x1b[0m")
			}
			proj.Celulas = append(proj.Celulas, cel)
		}
		e.Projetos = append(e.Projetos, proj)
	}
	return e
}

func BenchmarkDesenharMosaico(b *testing.B) {
	estado := estadoDeCarga(3, 4, 100, 24)
	b.ReportAllocs()
	for b.Loop() {
		_ = Desenhar(estado, Foco{}, teclado.Navegar, 200, 55, "")
	}
}

func BenchmarkDispor(b *testing.B) {
	estado := estadoDeCarga(3, 4, 100, 24)
	b.ReportAllocs()
	for b.Loop() {
		_ = Dispor(estado, Foco{}, 200, 55)
	}
}
