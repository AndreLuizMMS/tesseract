// Package tela é o cliente do motor: desenha o que ele manda e devolve tecla.
// Nenhuma regra de negócio mora aqui.
package tela

import (
	"encoding/json"
	"net"
	"sync"

	"github.com/andreluiz/tesseract/internal/protocolo"
)

// Cliente é a conversa da tela com o motor.
type Cliente struct {
	conexao  net.Conn
	escritor *json.Encoder
	leitor   *json.Decoder
	caneta   sync.Mutex
}

// Conectar abre a conversa com o motor.
func Conectar(caminhoSocket string) (*Cliente, error) {
	conexao, err := net.Dial("unix", caminhoSocket)
	if err != nil {
		return nil, err
	}
	return &Cliente{
		conexao:  conexao,
		escritor: json.NewEncoder(conexao),
		leitor:   json.NewDecoder(conexao),
	}, nil
}

// Enviar manda um pedido ao motor.
func (c *Cliente) Enviar(tipo string, dados any) error {
	envelope, err := protocolo.Empacotar(tipo, dados)
	if err != nil {
		return err
	}
	c.caneta.Lock()
	defer c.caneta.Unlock()
	return c.escritor.Encode(envelope)
}

// Receber espera a próxima mensagem do motor.
func (c *Cliente) Receber() (protocolo.Mensagem, error) {
	var envelope protocolo.Mensagem
	err := c.leitor.Decode(&envelope)
	return envelope, err
}

// Fechar encerra a conversa. O motor continua rodando — nada morre.
func (c *Cliente) Fechar() error { return c.conexao.Close() }
