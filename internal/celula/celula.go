// Package celula guarda o contrato de uma célula e o registro dos tipos que
// existem. Um tipo novo é um arquivo aqui dentro mais uma linha no registro —
// a tela, o teclado e o motor não são tocados.
package celula

import (
	"fmt"
	"sort"
	"sync"

	"github.com/andreluiz/tesseract/internal/motor/historico"
)

// Estado é a situação da célula, como aparece no marcador.
type Estado string

const (
	Trabalhando Estado = "trabalhando" // processo vivo produzindo
	Respondeu   Estado = "respondeu"   // devolveu a vez, tem resposta esperando
	Aprovar     Estado = "aprovar"     // travou numa pergunta e não anda sem resposta
	Caiu        Estado = "caiu"        // o processo morreu sozinho
	Parada      Estado = "parada"      // sem processo, célula preservada
	Orfa        Estado = "orfa"        // o diretório do projeto sumiu do disco
)

// Marcador é o símbolo do estado na barra da célula.
func (e Estado) Marcador() string {
	switch e {
	case Trabalhando:
		return "▸"
	case Respondeu:
		return "⬤"
	case Aprovar:
		return "⏵"
	case Caiu:
		return "✖"
	case Parada:
		return "○"
	case Orfa:
		return "⚠"
	}
	return "·"
}

// Config é tudo o que uma célula precisa para nascer.
type Config struct {
	ID        string               // identidade estável, sobrevive ao processo
	Diretorio string               // raiz do projeto
	Nome      string               // rótulo escolhido pelo usuário
	Alvo      string               // arquivo do md, serviço do logs
	Prompt    string               // trabalho inicial, só para agente
	Historico *historico.Historico // onde tudo o que a célula produz é gravado
	Colunas   int                  // largura da área reservada pela tela
	Linhas    int                  // altura da área reservada pela tela
	Avisar    func()               // chamado quando a célula tem algo novo para mostrar

	// Conversa é a identidade da conversa a reatar. Vazia, o agente começa
	// uma nova e avisa qual é por AoDescobrirConversa.
	Conversa            string
	AoDescobrirConversa func(id string)

	// Perfil do agente, quando o tipo é de agente. Vazio, o tipo usa o seu
	// padrão.
	Programa        string
	Args            []string
	ComandoRenomear string
	Marcadores      Marcadores
}

// Toque é uma tecla chegando na célula. Vem a tecla, não os bytes dela: quem
// traduz é a tela interna, que conhece os modos do terminal lá dentro.
type Toque struct {
	Codigo rune   // a tecla; especiais usam os códigos do emulador
	Texto  string // o que a tecla imprime, quando imprime algo
	Mod    int    // ctrl, alt, shift somados
	Colar  string // texto colado de uma vez, sem passar tecla por tecla
}

// Quadro é o que a tela desenha: o retrato da célula agora.
type Quadro struct {
	Linhas  []string
	CursorX int
	CursorY int
	Rolagem int  // quantas linhas acima do vivo a leitura está
	AoVivo  bool // falso quando o usuário rolou para trás
}

// Celula é o contrato. Um tipo declara quatro coisas: como nasce, como se
// desenha, o que faz com uma tecla, e quais estados tem.
type Celula interface {
	Nascer(Config) error
	Desenhar() Quadro
	Tecla(Toque) error
	Estado() Estado
	Estados() []Estado

	// Serviço: a tela avisa o tamanho e a rolagem; o motor mata.
	Redimensionar(colunas, linhas int) error
	Rolar(delta int, aoVivo bool)
	Matar() error
}

// Descritor é a ficha do tipo: o que o formulário de criação precisa perguntar
// para uma célula desse tipo nascer. Sem isso o formulário teria que conhecer
// os tipos por dentro, e tipo novo mexeria na tela.
type Descritor struct {
	Tipo string
	// Ordem coloca o tipo na fila do formulário. Menor vem antes.
	Ordem int
	// RotuloAlvo é o nome do quarto campo quando o tipo precisa de um alvo
	// (arquivo do md, serviço do logs). Vazio quando o tipo não pede nada.
	RotuloAlvo string
	// CompletaArquivo diz que o alvo é um caminho no disco e aceita tab.
	CompletaArquivo bool
	// AceitaPrompt marca os tipos que sobem já trabalhando.
	AceitaPrompt bool
	// Conversa marca os tipos que têm conversa com nome próprio — os que
	// respondem a renomear para dentro e a adotar nome de fora.
	Conversa bool
}

type entrada struct {
	descritor Descritor
	fabrica   func() Celula
}

var (
	mu       sync.RWMutex
	registro = map[string]entrada{}
)

// Registrar declara um tipo de célula. Chamado no init do arquivo do tipo, e é
// a única linha que um tipo novo acrescenta fora do próprio arquivo.
func Registrar(descritor Descritor, fabrica func() Celula) {
	mu.Lock()
	defer mu.Unlock()
	if _, repetido := registro[descritor.Tipo]; repetido {
		panic("tipo de célula registrado duas vezes: " + descritor.Tipo)
	}
	registro[descritor.Tipo] = entrada{descritor: descritor, fabrica: fabrica}
}

// Nova fabrica uma célula do tipo pedido.
func Nova(tipo string) (Celula, error) {
	mu.RLock()
	defer mu.RUnlock()
	achado, existe := registro[tipo]
	if !existe {
		return nil, fmt.Errorf("tipo de célula desconhecido: %q", tipo)
	}
	return achado.fabrica(), nil
}

// Descritores lista as fichas dos tipos registrados, na ordem em que o
// formulário deve mostrá-los.
func Descritores() []Descritor {
	mu.RLock()
	defer mu.RUnlock()
	fichas := make([]Descritor, 0, len(registro))
	for _, achado := range registro {
		fichas = append(fichas, achado.descritor)
	}
	sort.Slice(fichas, func(i, j int) bool {
		if fichas[i].Ordem != fichas[j].Ordem {
			return fichas[i].Ordem < fichas[j].Ordem
		}
		return fichas[i].Tipo < fichas[j].Tipo
	})
	return fichas
}

// Ficha devolve a ficha de um tipo.
func Ficha(tipo string) (Descritor, bool) {
	mu.RLock()
	defer mu.RUnlock()
	achado, existe := registro[tipo]
	return achado.descritor, existe
}

// Tipos lista os tipos registrados, na ordem do formulário.
func Tipos() []string {
	tipos := make([]string, 0)
	for _, ficha := range Descritores() {
		tipos = append(tipos, ficha.Tipo)
	}
	return tipos
}
