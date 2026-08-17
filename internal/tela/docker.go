package tela

import (
	"strings"

	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/teclado"
)

// PainelDocker é o Docker do projeto focado, aberto por cima da tela. Enquanto
// ele está aberto, o teclado é dele.
type PainelDocker struct {
	Projeto    string
	Nome       string
	Arquivo    string
	Lista      []protocolo.Servico
	Escolhido  int
	Erro       string
	Carregando bool
}

// NovoPainelDocker abre o painel já pedindo a lista.
func NovoPainelDocker(idProjeto, nome string) *PainelDocker {
	return &PainelDocker{Projeto: idProjeto, Nome: nome, Carregando: true}
}

// Chegou recebe do motor a lista de serviços.
func (p *PainelDocker) Chegou(servicos protocolo.Servicos) {
	p.Carregando = false
	p.Arquivo, p.Lista, p.Erro = servicos.Arquivo, servicos.Lista, servicos.Erro
	p.Escolhido = min(max(p.Escolhido, 0), max(len(p.Lista)-1, 0))
}

// Tecla trata uma tecla do painel. Devolve o pedido a mandar ao motor, se a
// ação vira uma célula de log, e se o painel continua aberto.
func (p *PainelDocker) Tecla(tecla string) (pedido *protocolo.Docker, aberto bool) {
	acao := teclado.Buscar(teclado.PainelDocker, tecla)
	escolhido := ""
	if p.Escolhido < len(p.Lista) {
		escolhido = p.Lista[p.Escolhido].Nome
	}

	switch acao {
	case teclado.Voltar:
		return nil, false
	case teclado.ServicoAnterior:
		p.Escolhido = max(p.Escolhido-1, 0)
	case teclado.ServicoProximo:
		p.Escolhido = min(p.Escolhido+1, max(len(p.Lista)-1, 0))

	// No serviço escolhido.
	case teclado.SobeServico:
		return p.pedir("sobe", escolhido), true
	case teclado.ParaServico:
		return p.pedir("para", escolhido), true
	case teclado.ReiniciaServico:
		return p.pedir("reinicia", escolhido), true
	case teclado.RebuildaServico:
		return p.pedir("rebuilda", escolhido), true
	case teclado.LogComoCelula:
		// O log vira célula no mosaico, então o painel fecha.
		return p.pedir("log", escolhido), false

	// Na stack inteira: a maiúscula é sempre a versão da stack da minúscula.
	case teclado.SobeStack:
		return p.pedir("sobe", ""), true
	case teclado.ParaStack:
		return p.pedir("para", ""), true
	case teclado.ReiniciaStack:
		return p.pedir("reinicia", ""), true
	}
	return nil, true
}

func (p *PainelDocker) pedir(acao, servico string) *protocolo.Docker {
	if acao != "sobe" && acao != "para" && acao != "reinicia" && acao != "rebuilda" && acao != "log" {
		return nil
	}
	if servico == "" && acao == "log" {
		return nil
	}
	return &protocolo.Docker{Projeto: p.Projeto, Acao: acao, Servico: servico}
}

// Desenhar monta a caixa do painel.
func (p *PainelDocker) Desenhar(largura int) []string {
	interno := min(largura-8, 76)
	corpo := []string{
		"  " + corApagada.Render(preencher("SERVIÇO", 14)+preencher("ESTADO", 16)+preencher("PORTA", 9)+preencher("SAÚDE", 12)+"UPTIME"),
	}
	switch {
	case p.Carregando:
		corpo = append(corpo, "  "+corApagada.Render("lendo a stack…"))
	case p.Erro != "":
		corpo = append(corpo, "  "+corErro.Render(cortar(p.Erro, interno-6)))
	case len(p.Lista) == 0:
		corpo = append(corpo, "  "+corApagada.Render("a stack nunca subiu neste projeto"))
	}

	for i, servico := range p.Lista {
		marca := "  "
		if i == p.Escolhido {
			marca = corBordaFocada.Render("▸ ")
		}
		bolinha, pintar := "○", corApagada
		if strings.HasPrefix(servico.Estado, "up") {
			bolinha, pintar = "●", marcadorDe("respondeu").cor
		}
		corpo = append(corpo, marca+
			corBarra.Render(preencher(servico.Nome, 14))+
			pintar.Render(preencher(bolinha+" "+servico.Estado, 16))+
			corApagada.Render(preencher(ouTraco(servico.Porta), 9))+
			corApagada.Render(preencher(ouTraco(servico.Saude), 12))+
			corApagada.Render(ouTraco(servico.Uptime)))
	}

	corpo = append(corpo, "")
	for _, linha := range teclado.Ajudas(teclado.PainelDocker) {
		corpo = append(corpo, "  "+corBarra.Render(preencher(linha.Teclas, 22))+corApagada.Render(linha.Ajuda))
	}
	titulo := "DOCKER · " + p.Nome
	if p.Arquivo != "" {
		titulo += " · " + nomeDoArquivo(p.Arquivo)
	}
	return caixaFlutuante(titulo, corpo, interno)
}

func ouTraco(texto string) string {
	if texto == "" {
		return "—"
	}
	return texto
}

func nomeDoArquivo(caminho string) string {
	if corte := strings.LastIndex(caminho, "/"); corte >= 0 {
		return caminho[corte+1:]
	}
	return caminho
}
