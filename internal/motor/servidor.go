package motor

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/andreluiz/tesseract/internal/protocolo"
)

// cadenciaQuadro é de quanto em quanto tempo o motor olha se tem novidade para
// mandar. Vinte e cinco quadros por segundo é fluido e barato.
const cadenciaQuadro = 40 * time.Millisecond

// ErrMotorJaRodando é a recusa de subir um segundo motor sobre o mesmo socket.
// Não é falha: quem pediu já tem o que queria.
var ErrMotorJaRodando = errors.New("já existe um motor rodando")

// Servidor é a porta do motor: uma tela conecta, manda tecla e recebe retrato.
type Servidor struct {
	motor   *Motor
	escuta  net.Listener
	caminho string
	parar   chan struct{}
	umaVez  sync.Once
}

// Servir abre o socket unix. Um socket largado por um motor que morreu é
// removido; um socket com motor vivo do outro lado devolve erro.
func Servir(m *Motor, caminho string) (*Servidor, error) {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o700); err != nil {
		return nil, err
	}
	if conexao, err := net.Dial("unix", caminho); err == nil {
		conexao.Close()
		return nil, ErrMotorJaRodando
	}
	_ = os.Remove(caminho)

	escuta, err := net.Listen("unix", caminho)
	if err != nil {
		return nil, err
	}
	s := &Servidor{motor: m, escuta: escuta, caminho: caminho, parar: make(chan struct{})}
	go s.aceitar()
	return s, nil
}

// Parado fecha quando alguém pediu `tess stop`.
func (s *Servidor) Parado() <-chan struct{} { return s.parar }

// Fechar solta o socket.
func (s *Servidor) Fechar() error {
	err := s.escuta.Close()
	_ = os.Remove(s.caminho)
	return err
}

func (s *Servidor) aceitar() {
	for {
		conexao, err := s.escuta.Accept()
		if err != nil {
			return
		}
		go s.atender(conexao)
	}
}

// atender cuida de uma tela: lê os pedidos dela numa ponta e empurra o retrato
// na outra.
func (s *Servidor) atender(conexao net.Conn) {
	defer conexao.Close()

	var caneta sync.Mutex
	escritor := json.NewEncoder(conexao)
	responder := func(tipo string, dados any) {
		envelope, err := protocolo.Empacotar(tipo, dados)
		if err != nil {
			return
		}
		caneta.Lock()
		defer caneta.Unlock()
		_ = escritor.Encode(envelope)
	}

	fim := make(chan struct{})
	go s.empurrarRetratos(responder, fim)
	defer close(fim)

	leitor := json.NewDecoder(conexao)
	for {
		var envelope protocolo.Mensagem
		if err := leitor.Decode(&envelope); err != nil {
			if err != io.EOF {
				return
			}
			return
		}
		s.atenderPedido(envelope, responder)
	}
}

// empurrarRetratos manda o estado inteiro assim que ele muda. A tela nunca
// pergunta: ela desenha o que chega.
func (s *Servidor) empurrarRetratos(responder func(string, any), fim <-chan struct{}) {
	relogio := time.NewTicker(cadenciaQuadro)
	defer relogio.Stop()

	responder(protocolo.TipoEstado, s.motor.Retrato())
	visto := s.motor.Versao()
	for {
		select {
		case <-fim:
			return
		case <-relogio.C:
			agora := s.motor.Versao()
			if agora == visto {
				continue
			}
			visto = agora
			responder(protocolo.TipoEstado, s.motor.Retrato())
		}
	}
}

func (s *Servidor) atenderPedido(envelope protocolo.Mensagem, responder func(string, any)) {
	falhou := func(err error) {
		if err != nil {
			responder(protocolo.TipoErro, protocolo.Erro{Mensagem: err.Error()})
		}
	}

	switch envelope.Tipo {
	case protocolo.TipoTecla:
		pedido, err := protocolo.Desempacotar[protocolo.Tecla](envelope)
		if err != nil {
			falhou(err)
			return
		}
		falhou(s.motor.Tecla(pedido))

	case protocolo.TipoTamanho:
		pedido, err := protocolo.Desempacotar[protocolo.Tamanho](envelope)
		if err != nil {
			falhou(err)
			return
		}
		falhou(s.motor.Tamanho(pedido.Celula, pedido.Colunas, pedido.Linhas))

	case protocolo.TipoRolar:
		pedido, err := protocolo.Desempacotar[protocolo.Rolar](envelope)
		if err != nil {
			falhou(err)
			return
		}
		falhou(s.motor.Rolar(pedido.Celula, pedido.Delta, pedido.AoVivo))

	case protocolo.TipoCriar:
		pedido, err := protocolo.Desempacotar[protocolo.Criar](envelope)
		if err != nil {
			falhou(err)
			return
		}
		_, err = s.motor.Criar(pedido)
		falhou(err)

	case protocolo.TipoMatar:
		pedido, err := protocolo.Desempacotar[protocolo.Matar](envelope)
		if err != nil {
			falhou(err)
			return
		}
		falhou(s.motor.Matar(pedido.Celula))

	case protocolo.TipoRenomear:
		pedido, err := protocolo.Desempacotar[protocolo.Renomear](envelope)
		if err != nil {
			falhou(err)
			return
		}
		falhou(s.motor.RenomearAutomatico(pedido.Celula))

	case protocolo.TipoAba:
		pedido, err := protocolo.Desempacotar[protocolo.Aba](envelope)
		if err != nil {
			falhou(err)
			return
		}
		falhou(s.motor.TrocarAba(pedido.Celula, pedido.Passo))

	case protocolo.TipoRetomar:
		pedido, err := protocolo.Desempacotar[protocolo.Retomar](envelope)
		if err != nil {
			falhou(err)
			return
		}
		falhou(s.motor.Retomar(pedido.Celula))

	case protocolo.TipoPrompt:
		pedido, err := protocolo.Desempacotar[protocolo.Prompt](envelope)
		if err != nil {
			falhou(err)
			return
		}
		falhou(s.motor.Prompt(pedido.Celula, pedido.Texto))

	case protocolo.TipoEditor:
		pedido, err := protocolo.Desempacotar[protocolo.Editor](envelope)
		if err != nil {
			falhou(err)
			return
		}
		falhou(s.motor.AbrirNoEditor(pedido.Projeto))

	case protocolo.TipoIrParaLinha:
		pedido, err := protocolo.Desempacotar[protocolo.IrParaLinha](envelope)
		if err != nil {
			falhou(err)
			return
		}
		falhou(s.motor.IrParaLinha(pedido.Celula, pedido.Linha))

	case protocolo.TipoBuscar:
		pedido, err := protocolo.Desempacotar[protocolo.Buscar](envelope)
		if err != nil {
			falhou(err)
			return
		}
		achados, err := s.motor.Buscar(pedido.Celula, pedido.Termo)
		if err != nil {
			falhou(err)
			return
		}
		resposta := protocolo.Achados{Celula: pedido.Celula, Termo: pedido.Termo}
		for _, achado := range achados {
			resposta.Linhas = append(resposta.Linhas, protocolo.Achado{Linha: achado.Linha, Texto: achado.Texto})
		}
		responder(protocolo.TipoAchados, resposta)

	case protocolo.TipoDocker:
		pedido, err := protocolo.Desempacotar[protocolo.Docker](envelope)
		if err != nil {
			falhou(err)
			return
		}
		// O painel é um pedido demorado: subir serviço baixa imagem. Ele sai
		// da fila do socket para a tela não travar enquanto isso.
		go func() {
			servicos, err := s.motor.Docker(pedido)
			if err != nil {
				falhou(err)
				return
			}
			responder(protocolo.TipoServicos, servicos)
		}()

	case protocolo.TipoCompletar:
		pedido, err := protocolo.Desempacotar[protocolo.Completar](envelope)
		if err != nil {
			falhou(err)
			return
		}
		responder(protocolo.TipoCompletado, Completar(pedido))

	case protocolo.TipoTela:
		pedido, err := protocolo.Desempacotar[protocolo.Tela](envelope)
		if err != nil {
			falhou(err)
			return
		}
		s.motor.TrocarTela(pedido.Tela)

	case protocolo.TipoStatus:
		responder(protocolo.TipoResumo, protocolo.Resumo{Texto: s.motor.Resumo()})

	case protocolo.TipoParar:
		responder(protocolo.TipoResumo, protocolo.Resumo{Texto: "motor desligado"})
		s.umaVez.Do(func() { close(s.parar) })
	}
}
