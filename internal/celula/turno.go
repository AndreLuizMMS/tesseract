package celula

import (
	"crypto/sha256"
	"strings"
	"sync"
)

// Um agente que diz quando está trabalhando é levado a sério. Só os que não
// dizem nada são julgados pela tela mudando — e aí spinner piscando e cursor se
// mexendo contariam como trabalho, que é de onde vem o alarme falso.
const (
	// leiturasParaArmar é quantas leituras seguidas de trabalho a célula
	// precisa antes de terminar valer como resposta.
	leiturasParaArmar = 3
	// leiturasParaEncerrar é quantas leituras seguidas de silêncio a célula
	// precisa antes do turno contar como encerrado. A meio segundo por leitura,
	// isso são três segundos — o bastante para cobrir a pausa que o agente dá
	// no meio de uma resposta.
	leiturasParaEncerrar = 6
)

// Marcadores é o que um agente escreve na própria interface: Trabalho enquanto
// o turno está em andamento, Pergunta enquanto ele espera um sim ou não.
//
// São listas porque os agentes trocam esses textos entre versões, e porque a
// mesma célula precisa reconhecer a versão velha e a nova sem escolher.
type Marcadores struct {
	Trabalho []string
	Pergunta []string
}

// algumPresente diz se algum dos textos aparece na tela, ignorando maiúsculas.
func algumPresente(tela string, textos []string) bool {
	if len(textos) == 0 {
		return false
	}
	minuscula := strings.ToLower(tela)
	for _, texto := range textos {
		if texto == "" {
			continue
		}
		if strings.Contains(minuscula, strings.ToLower(texto)) {
			return true
		}
	}
	return false
}

// Turno acompanha o estado de quem conversa. Estado de turno só faz sentido
// para agente: um shell não "responde".
type Turno struct {
	mu         sync.Mutex
	marcadores Marcadores
	resumoTela [sha256.Size]byte
	trabalho   int
	silencio   int
	armado     bool
	estado     Estado
}

// NovoTurno prepara o acompanhamento de um agente com os marcadores dele.
func NovoTurno(marcadores Marcadores) *Turno {
	return &Turno{marcadores: marcadores, estado: Trabalhando}
}

// Observar recebe a tela da célula e devolve em que estado ela está.
func (t *Turno) Observar(tela string) Estado {
	t.mu.Lock()
	defer t.mu.Unlock()

	mudou := t.telaMudou(tela)
	pergunta := algumPresente(tela, t.marcadores.Pergunta)

	trabalhando := mudou
	if len(t.marcadores.Trabalho) > 0 {
		trabalhando = algumPresente(tela, t.marcadores.Trabalho)
	}

	if trabalhando {
		t.trabalho++
		t.silencio = 0
		if t.trabalho >= leiturasParaArmar {
			t.armado = true
		}
		// Trabalho recomeçou: o que não foi lido perdeu a validade, e nada
		// está travado.
		t.estado = Trabalhando
		return t.estado
	}

	t.trabalho = 0

	// Pergunta na tela é o agente parado, não o agente trabalhando: ele fica
	// assim até alguém responder. Isso não é um turno encerrado.
	if pergunta {
		t.silencio = 0
		t.estado = Aprovar
		return t.estado
	}

	t.silencio++
	if t.armado && t.silencio >= leiturasParaEncerrar {
		t.armado = false
		t.estado = Respondeu
	}
	return t.estado
}

// Visto marca que alguém olhou a célula: o que estava esperando leitura já foi
// lido.
func (t *Turno) Visto() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.estado == Respondeu {
		t.estado = Trabalhando
	}
}

// Estado é o estado de turno agora.
func (t *Turno) Estado() Estado {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.estado
}

// telaMudou compara a tela com a leitura anterior. Guarda só o resumo porque a
// tela é grande e isso roda duas vezes por segundo para cada célula.
func (t *Turno) telaMudou(tela string) bool {
	resumo := sha256.Sum256([]byte(tela))
	if resumo == t.resumoTela {
		return false
	}
	t.resumoTela = resumo
	return true
}
