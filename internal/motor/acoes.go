package motor

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/andreluiz/tesseract/internal/celula"
	"github.com/andreluiz/tesseract/internal/docker"
	"github.com/andreluiz/tesseract/internal/protocolo"
)

// Retomar sobe de novo uma célula parada ou caída. Nada reinicia sozinho:
// reinício automático esconde causa raiz e produz loop de crash silencioso.
func (m *Motor) Retomar(idCelula string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.projetos {
		for _, c := range p.celulas {
			if c.id != idCelula {
				continue
			}
			if c.viva != nil {
				estado := c.viva.Estado()
				if estado != celula.Caiu && estado != celula.Parada && estado != celula.Orfa {
					return fmt.Errorf("a célula %s já está rodando", c.nome)
				}
				_ = c.viva.Matar()
			}
			if c.registro != nil {
				_ = c.registro.Fechar()
			}
			if err := m.acordar(p, c); err != nil {
				return err
			}
			m.marcarSujo()
			return nil
		}
	}
	return fmt.Errorf("célula %s não existe", idCelula)
}

// TrocarAba anda para a aba do lado dentro da célula. É o que permite uma
// sessão ter claude, cursor e shell sem escolher nada na criação.
func (m *Motor) TrocarAba(idCelula string, passo int) error {
	c := m.acharCelula(idCelula)
	if c == nil || c.viva == nil {
		return fmt.Errorf("célula %s não existe", idCelula)
	}
	comAbas, tem := c.viva.(celula.ComAbas)
	if !tem {
		return fmt.Errorf("a célula %s não tem abas", c.nome)
	}
	if err := comAbas.TrocarAba(passo); err != nil {
		return err
	}
	m.mu.Lock()
	c.aba = comAbas.AbaAtiva()
	m.salvar()
	m.mu.Unlock()
	m.marcarSujo()
	return nil
}

// Prompt manda trabalho para a célula sem o usuário entrar nela.
func (m *Motor) Prompt(idCelula, texto string) error {
	c := m.acharCelula(idCelula)
	if c == nil || c.viva == nil {
		return fmt.Errorf("célula %s não existe", idCelula)
	}
	comPrompt, aceita := c.viva.(celula.ComPrompt)
	if !aceita {
		return fmt.Errorf("a célula %s não recebe prompt", c.nome)
	}
	if err := comPrompt.Prompt(texto); err != nil {
		return err
	}
	m.marcarSujo()
	return nil
}

// RenomearEPropagar troca o rótulo da célula e, quando ela tem conversa, manda
// o nome para dentro do agente. Os dois nomes deixam de divergir.
func (m *Motor) RenomearEPropagar(idCelula, nome string) error {
	if err := m.Renomear(idCelula, nome); err != nil {
		return err
	}
	c := m.acharCelula(idCelula)
	if c == nil || c.viva == nil {
		return nil
	}
	if comConversa, tem := c.viva.(celula.ComConversa); tem {
		if err := comConversa.PropagarNome(nome); err != nil {
			return fmt.Errorf("o rótulo mudou, mas o agente não aceitou o nome: %w", err)
		}
	}
	return nil
}

// AdotarNomeDoAgente puxa para a célula o nome que o agente deu à conversa.
func (m *Motor) AdotarNomeDoAgente(idCelula string) error {
	c := m.acharCelula(idCelula)
	if c == nil || c.viva == nil {
		return fmt.Errorf("célula %s não existe", idCelula)
	}
	comConversa, tem := c.viva.(celula.ComConversa)
	if !tem {
		return fmt.Errorf("a célula %s não tem conversa com nome", c.nome)
	}
	nome, err := comConversa.NomeDaConversa()
	if err != nil {
		return err
	}
	return m.Renomear(idCelula, nome)
}

// AbrirNoEditor abre o diretório do projeto no editor configurado.
func (m *Motor) AbrirNoEditor(idProjeto string) error {
	m.mu.Lock()
	editor := m.config.Editor
	caminho := ""
	for _, p := range m.projetos {
		if p.id == idProjeto {
			caminho = p.caminho
		}
	}
	m.mu.Unlock()

	if caminho == "" {
		return fmt.Errorf("projeto %s não existe", idProjeto)
	}
	if editor == "" {
		return fmt.Errorf("nenhum editor configurado")
	}
	campos := strings.Fields(editor)
	comando := exec.Command(campos[0], append(campos[1:], caminho)...)
	if err := comando.Start(); err != nil {
		return fmt.Errorf("não consegui abrir %s: %w", editor, err)
	}
	go func() { _ = comando.Wait() }()
	return nil
}

// IrParaLinha leva a leitura da célula até uma linha do histórico — é onde a
// busca termina.
func (m *Motor) IrParaLinha(idCelula string, linha int) error {
	c := m.acharCelula(idCelula)
	if c == nil || c.viva == nil {
		return fmt.Errorf("célula %s não existe", idCelula)
	}
	total, err := m.registroDe(c).Linhas()
	if err != nil {
		return err
	}
	// Rolar conta linhas para trás a partir do fim, e a busca conta a partir do
	// começo.
	c.viva.Rolar(0, true)
	c.viva.Rolar(max(total-linha, 0), false)
	m.marcarSujo()
	return nil
}

// Docker age na stack do projeto, ou num serviço dela. Listar é o que o painel
// pede ao abrir; log vira célula no mosaico.
func (m *Motor) Docker(pedido protocolo.Docker) (protocolo.Servicos, error) {
	m.mu.Lock()
	var alvo *projeto
	for _, p := range m.projetos {
		if p.id == pedido.Projeto {
			alvo = p
		}
	}
	m.mu.Unlock()

	if alvo == nil {
		return protocolo.Servicos{}, fmt.Errorf("projeto %s não existe", pedido.Projeto)
	}
	if alvo.compose == "" {
		return protocolo.Servicos{}, fmt.Errorf("o projeto %s não tem arquivo de compose na raiz", alvo.caminho)
	}

	resposta := protocolo.Servicos{
		Projeto: alvo.id, Arquivo: alvo.compose,
		Acao: pedido.Acao, Servico: pedido.Servico,
	}
	switch pedido.Acao {
	case "listar":
	case "log":
		if pedido.Servico == "" {
			return resposta, fmt.Errorf("qual serviço?")
		}
		_, err := m.Criar(protocolo.Criar{
			Caminho: alvo.caminho,
			Tipo:    "logs",
			Nome:    "logs " + pedido.Servico,
			Alvo:    pedido.Servico,
		})
		return resposta, err
	default:
		if err := docker.Agir(alvo.caminho, alvo.compose, pedido.Acao, pedido.Servico); err != nil {
			resposta.Erro = err.Error()
			return resposta, nil
		}
	}

	servicos, err := docker.Servicos(alvo.caminho, alvo.compose)
	if err != nil {
		resposta.Erro = err.Error()
		return resposta, nil
	}
	for _, servico := range servicos {
		resposta.Lista = append(resposta.Lista, protocolo.Servico{
			Nome:   servico.Nome,
			Estado: servico.Estado,
			Porta:  servico.Porta,
			Saude:  servico.Saude,
			Uptime: servico.Uptime,
		})
	}

	m.mu.Lock()
	alvo.docker = docker.Resumo(servicos)
	m.mu.Unlock()
	m.marcarSujo()
	return resposta, nil
}
