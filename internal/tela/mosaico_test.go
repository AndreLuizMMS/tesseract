package tela

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/andreluiz/tesseract/internal/motor/historico"
	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
	"github.com/andreluiz/tesseract/internal/tema"
)

var atualizarGolden = flag.Bool("atualizar", false, "regrava os arquivos de referência do desenho")

// semEstilo tira os códigos de cor para o arquivo de referência ser legível e
// a comparação falhar por conteúdo, não por tom de cinza.
func semEstilo(desenho string) string {
	linhas := strings.Split(desenho, "\n")
	for i, linha := range linhas {
		linhas[i] = strings.TrimRight(historico.LimparCodigos(linha), " ")
	}
	return strings.Join(linhas, "\n")
}

func conferirGolden(t *testing.T, nome, desenho string) {
	t.Helper()
	caminho := filepath.Join("testdata", nome)
	limpo := semEstilo(desenho)
	if *atualizarGolden {
		if err := os.WriteFile(caminho, []byte(limpo), 0o644); err != nil {
			t.Fatalf("gravar referência: %v", err)
		}
		return
	}
	esperado, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("ler referência (rode com -atualizar para criar): %v", err)
	}
	if string(esperado) != limpo {
		t.Errorf("o desenho mudou.\n--- esperado ---\n%s\n--- veio ---\n%s", esperado, limpo)
	}
}

// gradeDeTeste é o mesmo estado usado pelo mosaico e pela lista.
func gradeDeTeste() protocolo.Estado {
	return protocolo.Estado{
		Tipos: []protocolo.TipoCelula{{Tipo: "sessao", AceitaPrompt: true}, {Tipo: "md", RotuloAlvo: "MD"}},
		Projetos: []protocolo.Projeto{
			{
				ID: "p1", Nome: "doxar-api", Caminho: "/home/dev/doxar-api", Cor: 0,
				TemCompose: true, Docker: "4/5",
				Celulas: []protocolo.Celula{
					{ID: "c1", Tipo: "sessao", Nome: "refatora auth", Estado: "respondeu", AoVivo: true,
						Abas: []string{"claude", "cursor", "bash"}, Aba: "claude",
						Linhas: []string{"Movi a validação de token", "pro guard.", "Qual você prefere?"}},
					{ID: "c2", Tipo: "sessao", Nome: "testes", Estado: "trabalhando", AoVivo: true,
						Abas: []string{"claude", "cursor", "bash"}, Aba: "bash",
						Linhas: []string{"$ go test ./...", "ok"}},
				},
			},
			{
				ID: "p2", Nome: "cortz-web", Caminho: "/home/dev/cortz-web", Cor: 1,
				Celulas: []protocolo.Celula{
					{ID: "c3", Tipo: "sessao", Nome: "fix nav", Estado: "aprovar", AoVivo: true,
						Abas: []string{"claude", "cursor", "bash"}, Aba: "claude",
						Linhas: []string{"posso mexer no Header?"}},
				},
			},
			{
				ID: "p3", Nome: "api-legado", Caminho: "/home/dev/api-legado", Cor: 2,
				Celulas: []protocolo.Celula{
					{ID: "c4", Tipo: "md", Nome: "spec-m7.md", Estado: "parada", AoVivo: true,
						Linhas: []string{"# Módulo 7"}},
				},
			},
		},
	}
}

// TestMosaicoMostraTodasAsCelulas — a regra nova: nenhuma célula vira tira,
// todas aparecem ao mesmo tempo, de todos os projetos.
func TestMosaicoMostraTodasAsCelulas(t *testing.T) {
	estado := gradeDeTeste()
	completo := Desenhar(estado, Foco{Projeto: 0, Celula: 0}, teclado.Navegar, 120, 30, "")
	conferirGolden(t, "mosaico-foco-0.txt", completo)

	desenho := semEstilo(completo)
	for _, projeto := range estado.Projetos {
		if !strings.Contains(desenho, strings.ToUpper(projeto.Nome)) {
			t.Errorf("o projeto %q devia estar na tela", projeto.Nome)
		}
		for _, celula := range projeto.Celulas {
			if !strings.Contains(desenho, celula.Nome) {
				t.Errorf("a célula %q devia estar na tela", celula.Nome)
			}
			for _, linha := range celula.Linhas {
				if !strings.Contains(desenho, linha) {
					t.Errorf("o conteúdo %q da célula %q devia aparecer", linha, celula.Nome)
				}
			}
		}
	}
}

// TestMosaicoNaoTemMaisTira — o texto do projeto não focado não some mais.
func TestMosaicoNaoTemMaisTira(t *testing.T) {
	estado := gradeDeTeste()
	comFocoNoPrimeiro := semEstilo(Desenhar(estado, Foco{Projeto: 0}, teclado.Navegar, 120, 30, ""))
	comFocoNoUltimo := semEstilo(Desenhar(estado, Foco{Projeto: 2}, teclado.Navegar, 120, 30, ""))
	conferirGolden(t, "mosaico-foco-2.txt", Desenhar(estado, Foco{Projeto: 2}, teclado.Navegar, 120, 30, ""))

	for _, pedaco := range []string{"CORTZ-WEB", "fix nav", "posso mexer no Header?", "spec-m7.md"} {
		if !strings.Contains(comFocoNoPrimeiro, pedaco) {
			t.Errorf("com o foco no primeiro projeto, %q continua visível", pedaco)
		}
	}
	for _, pedaco := range []string{"DOXAR-API", "refatora auth", "testes"} {
		if !strings.Contains(comFocoNoUltimo, pedaco) {
			t.Errorf("com o foco no último projeto, %q continua visível", pedaco)
		}
	}
}

// TestCelulasDoMesmoProjetoFicamLadoALado — um projeto não é obrigado a ficar
// na vertical: as células dele dividem a largura.
func TestCelulasDoMesmoProjetoFicamLadoALado(t *testing.T) {
	estado := gradeDeTeste()
	d := Dispor(estado, Foco{Projeto: 0}, 140, 30)

	if len(d.todasAsFaixas()) != 3 {
		t.Fatalf("três projetos, três fileiras: %#v", d.todasAsFaixas())
	}
	if len(d.todasAsFaixas()[0].celulas) != 2 {
		t.Fatalf("as duas células do primeiro projeto deviam dividir a fileira: %#v", d.todasAsFaixas()[0])
	}

	primeira, segunda := d.miolos["c1"], d.miolos["c2"]
	if primeira.Colunas < larguraApertadaDeCelula || segunda.Colunas < larguraApertadaDeCelula {
		t.Fatalf("as células deviam dividir a largura da coluna: %#v %#v", primeira, segunda)
	}
	if primeira.Linhas != segunda.Linhas {
		t.Fatalf("células da mesma fileira têm a mesma altura: %#v %#v", primeira, segunda)
	}
	if sozinha := d.miolos["c3"]; sozinha.Colunas <= primeira.Colunas {
		t.Fatalf("a célula sozinha devia ocupar a coluna inteira: %#v", sozinha)
	}
}

// TestFileiraQuebraQuandoNaoCabeNaLargura — muitas células no mesmo projeto
// viram mais de uma fileira em vez de espremer.
func TestFileiraQuebraQuandoNaoCabeNaLargura(t *testing.T) {
	estado := protocolo.Estado{Projetos: []protocolo.Projeto{{ID: "p1", Nome: "grande", Caminho: "/dev/grande"}}}
	for i := range 6 {
		estado.Projetos[0].Celulas = append(estado.Projetos[0].Celulas, protocolo.Celula{
			ID: "c" + strconv.Itoa(i), Tipo: "sessao", Nome: "cel" + strconv.Itoa(i),
			Estado: "trabalhando", AoVivo: true,
		})
	}

	d := Dispor(estado, Foco{}, 120, 40)
	if len(d.todasAsFaixas()) < 2 {
		t.Fatalf("seis células em 120 colunas não cabem numa fileira só: %#v", d.todasAsFaixas())
	}
	for _, f := range d.todasAsFaixas() {
		if len(f.celulas) > 3 {
			t.Fatalf("fileira com %d células fica ilegível", len(f.celulas))
		}
	}
	for id, miolo := range d.Miolos() {
		if miolo.Colunas < larguraMinimaDeCelula-2 {
			t.Fatalf("a célula %s ficou com %d colunas", id, miolo.Colunas)
		}
	}
}

// TestProjetosViramColunasQuandoAAlturaAcaba — com projetos demais para a
// altura, a tela cresce para o lado em vez de jogar célula para fora.
func TestProjetosViramColunasQuandoAAlturaAcaba(t *testing.T) {
	estado := protocolo.Estado{}
	for i := range 6 {
		estado.Projetos = append(estado.Projetos, protocolo.Projeto{
			ID: "p" + strconv.Itoa(i), Nome: "projeto" + strconv.Itoa(i), Caminho: "/dev/p",
			Celulas: []protocolo.Celula{{
				ID: "c" + strconv.Itoa(i), Tipo: "sessao", Nome: "cel" + strconv.Itoa(i),
				Estado: "trabalhando", AoVivo: true,
			}},
		})
	}

	d := Dispor(estado, Foco{}, 120, 35)
	if len(d.colunas) != 2 {
		t.Fatalf("seis projetos em 35 linhas deviam abrir duas colunas: %d", len(d.colunas))
	}
	if d.escondidas != 0 {
		t.Fatalf("com as duas colunas nenhuma célula devia ficar de fora: %d", d.escondidas)
	}
	if len(d.Miolos()) != 6 {
		t.Fatalf("as seis células deviam ter tamanho avisado ao motor: %#v", d.Miolos())
	}
	for id, miolo := range d.Miolos() {
		if miolo.Colunas < larguraApertadaDeCelula || miolo.Linhas < alturaMinimaDeCelula-2 {
			t.Fatalf("a célula %s ficou pequena demais: %#v", id, miolo)
		}
	}
}

// TestMosaicoAvisaOQueNaoCoube — com células demais, some o desenho, nunca a
// contagem.
func TestMosaicoAvisaOQueNaoCoube(t *testing.T) {
	estado := protocolo.Estado{}
	for i := range 12 {
		estado.Projetos = append(estado.Projetos, protocolo.Projeto{
			ID: "p" + strconv.Itoa(i), Nome: "projeto" + strconv.Itoa(i), Caminho: "/dev/p",
			Celulas: []protocolo.Celula{{
				ID: "c" + strconv.Itoa(i), Tipo: "sessao", Nome: "cel" + strconv.Itoa(i),
				Estado: "trabalhando", AoVivo: true,
			}},
		})
	}

	desenho := semEstilo(Desenhar(estado, Foco{Projeto: 0}, teclado.Navegar, 100, 20, ""))
	if !strings.Contains(desenho, "fora da tela") {
		t.Errorf("com células demais, a tela precisa dizer quantas ficaram de fora:\n%s", desenho)
	}
	if !strings.Contains(desenho, "cel0") {
		t.Errorf("a célula focada tem que estar na tela:\n%s", desenho)
	}

	desenhoFinal := semEstilo(Desenhar(estado, Foco{Projeto: 11}, teclado.Navegar, 100, 20, ""))
	if !strings.Contains(desenhoFinal, "cel11") {
		t.Errorf("a janela devia seguir o foco:\n%s", desenhoFinal)
	}
}

// TestAbasAparecemNoLugarDoTipo — a célula com abas mostra as abas, com a ativa
// em destaque.
func TestAbasAparecemNoLugarDoTipo(t *testing.T) {
	estado := gradeDeTeste()
	desenho := semEstilo(Desenhar(estado, Foco{Projeto: 0, Celula: 0}, teclado.Navegar, 140, 30, ""))
	for _, aba := range []string{"claude", "cursor", "bash"} {
		if !strings.Contains(desenho, aba) {
			t.Errorf("a aba %q devia aparecer na borda da célula:\n%s", aba, desenho)
		}
	}
	if !strings.Contains(desenho, "refatora auth") {
		t.Error("o nome da célula continua ao lado das abas")
	}
}

// TestMosaicoEmDigitarApagaOResto — três sinais redundantes: barra apagada,
// selo e borda grossa e verde.
func TestMosaicoEmDigitarApagaOResto(t *testing.T) {
	estado := gradeDeTeste()
	desenho := semEstilo(Desenhar(estado, Foco{Projeto: 0, Celula: 1}, teclado.Digitar, 120, 30, ""))
	if !strings.Contains(desenho, "▓ DIGITAR ▓") {
		t.Error("falta o selo de DIGITAR na barra")
	}
	if !strings.Contains(desenho, "┏") || !strings.Contains(desenho, "┃") {
		t.Error("a borda da célula focada devia engrossar")
	}
	if !strings.Contains(desenho, "ctrl-l devolve o teclado") {
		t.Error("o rodapé devia encolher para a linha do ctrl-l")
	}
	if strings.Contains(desenho, "NAVEGAR") {
		t.Error("a barra não pode dizer NAVEGAR em modo DIGITAR")
	}
}

// TestCelulaFicaVerdeAoDigitar — em DIGITAR a célula que tem o teclado é a
// única acesa, e ela fica verde.
func TestCelulaFicaVerdeAoDigitar(t *testing.T) {
	estado := gradeDeTeste()
	comTeclado := Desenhar(estado, Foco{Projeto: 0, Celula: 0}, teclado.Digitar, 120, 30, "")
	semTeclado := Desenhar(estado, Foco{Projeto: 0, Celula: 0}, teclado.Navegar, 120, 30, "")

	// O escape sai do próprio estilo, e não de um número escrito à mão: o
	// tema pode rebaixar a cor conforme o terminal, e o teste continua valendo.
	if prefixo := strings.SplitN(corDigitando.Render("x"), "x", 2)[0]; prefixo != "" &&
		!strings.Contains(comTeclado, prefixo) {
		t.Error("a célula focada devia ficar verde em DIGITAR")
	}
	if strings.Contains(semTeclado, "┏") {
		t.Error("fora de DIGITAR a borda não engrossa")
	}
	if !strings.Contains(comTeclado, "┏") {
		t.Error("em DIGITAR a borda da célula focada engrossa")
	}
}

// TestTelaCheiaMostraSoACelulaFocada — é assim que se copia um bloco de texto
// sem pegar os vizinhos.
func TestTelaCheiaMostraSoACelulaFocada(t *testing.T) {
	estado := gradeDeTeste()
	desenho := semEstilo(Desenhar(estado, Foco{Projeto: 0, Celula: 0, Cheia: true}, teclado.Navegar, 120, 30, ""))
	if !strings.Contains(desenho, "refatora auth") {
		t.Error("a célula focada devia estar na tela")
	}
	if strings.Contains(desenho, "go test") {
		t.Error("em tela cheia, a célula vizinha não pode aparecer")
	}
	if strings.Contains(desenho, "CORTZ-WEB") {
		t.Error("em tela cheia, os outros projetos não aparecem")
	}
}

// TestGeometriaBateComODesenho — o tamanho avisado ao motor é o mesmo que a
// tela desenha, senão o processo lá dentro enxerga um terminal torto.
func TestGeometriaBateComODesenho(t *testing.T) {
	estado := gradeDeTeste()
	for _, tamanho := range [][2]int{{120, 30}, {80, 20}, {200, 50}, {60, 14}} {
		largura, altura := tamanho[0], tamanho[1]
		d := Dispor(estado, Foco{Projeto: 0}, largura, altura)
		desenho := strings.Split(Desenhar(estado, Foco{Projeto: 0}, teclado.Navegar, largura, altura, ""), "\n")
		if len(desenho) != altura {
			t.Errorf("%dx%d: o desenho tem %d linhas", largura, altura, len(desenho))
		}
		for _, linha := range desenho {
			if visivel := lipgloss.Width(semEstilo(linha)); visivel > largura {
				t.Errorf("%dx%d: linha com %d colunas: %q", largura, altura, visivel, linha)
			}
		}
		for _, c := range d.colunas {
			usado := 0
			for _, f := range c.faixas {
				if f.abre {
					usado++
				}
				usado += f.altura
			}
			if usado > altura-2 {
				t.Errorf("%dx%d: a coluna soma %d linhas, o corpo tem %d", largura, altura, usado, altura-2)
			}
		}
		for id, miolo := range d.Miolos() {
			if miolo.Colunas < 1 || miolo.Linhas < 1 {
				t.Errorf("%dx%d: miolo inválido da célula %s: %#v", largura, altura, id, miolo)
			}
		}
	}
}

// TestOrigemDoCursorCaiDentroDaCelula — o cursor precisa pousar no miolo certo.
func TestOrigemDoCursorCaiDentroDaCelula(t *testing.T) {
	estado := gradeDeTeste()
	foco := Foco{Projeto: 0, Celula: 1}
	x, y, tem := OrigemNoMosaico(estado, foco, 140, 30, "c2")
	if !tem {
		t.Fatal("a célula focada precisa ter origem")
	}
	if x < 1 || y < 2 {
		t.Fatalf("origem estranha: %d,%d", x, y)
	}

	linhas := strings.Split(semEstilo(Desenhar(estado, foco, teclado.Navegar, 140, 30, "")), "\n")
	if y >= len(linhas) {
		t.Fatalf("a origem caiu fora da tela: linha %d de %d", y, len(linhas))
	}
	if !strings.Contains(linhas[y-1], "testes") {
		t.Fatalf("a linha acima da origem devia ser a borda da célula, veio %q", linhas[y-1])
	}
}

// TestCelulaNaoFocadaPerdeACor: durante a navegação só a célula selecionada
// fica acesa; o resto vira cinza para o olho achar onde está.
func TestCelulaNaoFocadaPerdeACor(t *testing.T) {
	celula := protocolo.Celula{
		ID: "c1", Tipo: "claude", Nome: "um", Estado: "parada",
		AoVivo: true, Linhas: []string{"\x1b[31mvermelho\x1b[0m"},
	}
	if focada := strings.Join(caixaDaCelula(celula, teclado.Navegar, true, 30, 4), ""); !strings.Contains(focada, "\x1b[31m") {
		t.Fatal("a célula focada tinha que manter a cor do conteúdo")
	}
	apagada := strings.Join(caixaDaCelula(celula, teclado.Navegar, false, 30, 4), "")
	if strings.Contains(apagada, "\x1b[31m") {
		t.Fatal("a célula não focada tinha que sair sem a cor do conteúdo")
	}
	if !strings.Contains(historico.LimparCodigos(apagada), "vermelho") {
		t.Fatal("apagar a cor não pode comer o texto")
	}
}

// TestSeloDoModoFicaNoCantoDireito — só um selo invertido visível por vez, e
// ele mora onde o olho procura estado de janela.
func TestSeloDoModoFicaNoCantoDireito(t *testing.T) {
	estado := gradeDeTeste()
	desenho := semEstilo(Desenhar(estado, Foco{Projeto: 0, Celula: 0}, teclado.Digitar, 120, 30, ""))
	// O selo mora na linha de estado, que é a segunda do cabeçalho — a
	// primeira é a faixa da marca.
	estado_ := strings.Split(desenho, "\n")[alturaDaBarra-1]

	selo := strings.Index(estado_, "▓ DIGITAR ▓")
	if selo < 0 {
		t.Fatalf("o selo do modo sumiu da linha de estado: %q", estado_)
	}
	if selo < len([]rune(estado_))/2 {
		t.Fatalf("o selo devia estar na metade direita, e está na coluna %d de %d", selo, len(estado_))
	}
}

// TestBarraDeAvisoSoPiscaComCelulaTravada — o relógio existe enquanto alguém
// espera você, e para sozinho quando ninguém mais espera.
func TestBarraDeAvisoSoPiscaComCelulaTravada(t *testing.T) {
	m := &Modelo{estado: gradeDeTeste()}
	if !m.temCelulaTravada() {
		t.Fatal("a grade de teste tem uma célula em aprovar")
	}
	if m.piscarOAviso() == nil {
		t.Fatal("com célula travada o relógio devia começar")
	}

	for i := range m.estado.Projetos {
		for j := range m.estado.Projetos[i].Celulas {
			m.estado.Projetos[i].Celulas[j].Estado = "parada"
		}
	}
	if m.piscarOAviso() != nil {
		t.Fatal("sem célula travada o relógio devia parar")
	}
	if m.piscando {
		t.Fatal("o relógio parado não pode ficar marcado como ligado")
	}
}

// TestBarraDeAvisoMudaEntreOsQuadros — o teste que pega o estilo congelado: se
// o marcador guardar a cor pronta em vez de perguntar ao tema, os dois quadros
// saem idênticos e a barra nunca pisca na tela de verdade.
func TestBarraDeAvisoMudaEntreOsQuadros(t *testing.T) {
	antes := tema.Apagado
	defer func() { tema.Apagado = antes }()

	estado := gradeDeTeste()
	tema.Apagado = false
	aceso := Desenhar(estado, Foco{Projeto: 0, Celula: 0}, teclado.Navegar, 120, 30, "")
	tema.Apagado = true
	apagado := Desenhar(estado, Foco{Projeto: 0, Celula: 0}, teclado.Navegar, 120, 30, "")

	if aceso == apagado {
		t.Fatal("o quadro apagado devia desenhar diferente do aceso")
	}
	if semEstilo(aceso) != semEstilo(apagado) {
		t.Fatal("a piscada é só de cor: o texto da tela não pode mudar")
	}
}

// TestCabecalhoTemAMarcaNoMeio — a faixa é o topo da tela e o eixo do olho: a
// marca fica no meio dela, com a régua atravessando dos dois lados.
func TestCabecalhoTemAMarcaNoMeio(t *testing.T) {
	faixa := semEstilo(faixaDaMarca(teclado.Navegar, 120))
	if lipgloss.Width(faixa) != 120 {
		t.Fatalf("a faixa tem que ocupar a largura inteira, e tem %d", lipgloss.Width(faixa))
	}
	// Em runas, não em bytes: a régua é feita de caracteres de três bytes, e
	// contar errado aqui daria um desvio inventado.
	marca := len([]rune(faixa[:max(strings.Index(faixa, tema.Glifo), 0)]))
	if !strings.Contains(faixa, tema.Glifo) {
		t.Fatalf("a marca sumiu da faixa: %q", faixa)
	}
	// Centro com folga de uma casa para cada lado, porque a sobra ímpar da
	// divisão cai num dos lados.
	if desvio := marca - len([]rune(faixa))/2; desvio > 12 || desvio < -12 {
		t.Fatalf("a marca devia estar no meio, e está %d colunas fora", desvio)
	}
	if !strings.HasPrefix(faixa, "─") || !strings.HasSuffix(faixa, "─") {
		t.Fatalf("a régua devia atravessar a faixa de ponta a ponta: %q", faixa)
	}
}

// TestQuatroCelulasViramGrade2x2 — um projeto com quatro sessões não vira uma
// tira de quatro; vira duas fileiras de duas.
func TestQuatroCelulasViramGrade2x2(t *testing.T) {
	estado := protocolo.Estado{Projetos: []protocolo.Projeto{{ID: "p1", Nome: "regula-mais", Caminho: "/dev/rm"}}}
	for i := range 4 {
		estado.Projetos[0].Celulas = append(estado.Projetos[0].Celulas, protocolo.Celula{
			ID: "c" + strconv.Itoa(i), Tipo: "sessao", Nome: "sessao" + strconv.Itoa(i),
			Estado: "trabalhando", AoVivo: true,
		})
	}

	d := Dispor(estado, Foco{}, 200, 40)
	faixas := d.todasAsFaixas()
	if len(faixas) != 2 {
		t.Fatalf("quatro células deviam ocupar duas fileiras: %#v", faixas)
	}
	for _, f := range faixas {
		if len(f.celulas) != 2 {
			t.Fatalf("cada fileira devia ter duas células: %#v", f)
		}
	}
}

// TestGradeQuadradaCedeParaAAltura — sem altura para duas fileiras, as quatro
// células voltam para uma tira só em vez de sair da tela.
func TestGradeQuadradaCedeParaAAltura(t *testing.T) {
	estado := protocolo.Estado{Projetos: []protocolo.Projeto{{ID: "p1", Nome: "apertado", Caminho: "/dev/ap"}}}
	for i := range 4 {
		estado.Projetos[0].Celulas = append(estado.Projetos[0].Celulas, protocolo.Celula{
			ID: "c" + strconv.Itoa(i), Tipo: "sessao", Nome: "sessao" + strconv.Itoa(i),
			Estado: "trabalhando", AoVivo: true,
		})
	}

	d := Dispor(estado, Foco{}, 200, 12)
	if faixas := d.todasAsFaixas(); len(faixas) != 1 || len(faixas[0].celulas) != 4 {
		t.Fatalf("sem altura, as quatro células ficam numa fileira só: %#v", faixas)
	}
	if d.escondidas != 0 {
		t.Fatalf("nenhuma célula devia ficar fora da tela: %d", d.escondidas)
	}
}
