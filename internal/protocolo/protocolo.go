// Package protocolo é o contrato entre o motor e a tela. Uma mensagem por
// linha, JSON puro, para que a tela nunca precise saber como o motor guarda as
// coisas — só o que ele manda desenhar.
package protocolo

import "encoding/json"

// Tipos de mensagem. A tela manda os quatro primeiros; o motor manda os dois
// últimos.
const (
	TipoTecla       = "tecla"
	TipoTamanho     = "tamanho"
	TipoRolar       = "rolar"
	TipoCriar       = "criar"
	TipoMatar       = "matar"
	TipoRenomear    = "renomear"
	TipoAdotar      = "adotar"
	TipoRetomar     = "retomar"
	TipoAba         = "aba"
	TipoPrompt      = "prompt"
	TipoCompletar   = "completar"
	TipoTela        = "tela"
	TipoBuscar      = "buscar"
	TipoDocker      = "docker"
	TipoEditor      = "editor"
	TipoIrParaLinha = "ir-para-linha"
	TipoStatus      = "status"
	TipoParar       = "parar"
	TipoEstado      = "estado"
	TipoCompletado  = "completado"
	TipoAchados     = "achados"
	TipoServicos    = "servicos"
	TipoResumo      = "resumo"
	TipoErro        = "erro"
)

// Tecla leva a tecla para dentro da célula. Vai a tecla, não os bytes dela:
// quem sabe traduzir tecla em bytes é a tela interna da célula, que conhece os
// modos do terminal lá dentro.
type Tecla struct {
	Celula string `json:"celula"`
	Codigo rune   `json:"codigo"`
	Texto  string `json:"texto,omitempty"`
	Mod    int    `json:"mod,omitempty"`
	Colar  string `json:"colar,omitempty"` // texto colado de uma vez
}

// Tamanho informa ao motor a área que a tela reservou para a célula, para que
// o processo lá dentro enxergue o terminal do tamanho certo.
type Tamanho struct {
	Celula  string `json:"celula"`
	Colunas int    `json:"colunas"`
	Linhas  int    `json:"linhas"`
}

// Rolar move a leitura do histórico da célula. AoVivo verdadeiro devolve a
// leitura para o fim, ignorando o delta.
type Rolar struct {
	Celula string `json:"celula"`
	Delta  int    `json:"delta"`
	AoVivo bool   `json:"aoVivo"`
}

// Criar nasce uma célula. Se o caminho ainda não estiver na tela, o projeto
// nasce junto.
type Criar struct {
	Caminho string `json:"caminho"`
	Tipo    string `json:"tipo"`
	Nome    string `json:"nome"`
	Alvo    string `json:"alvo"`
	Prompt  string `json:"prompt"`
}

// Matar remove a célula. O disco nunca é tocado.
type Matar struct {
	Celula string `json:"celula"`
}

// Renomear troca o rótulo da célula e, quando ela tem conversa, propaga o nome
// para dentro do agente.
type Renomear struct {
	Celula string `json:"celula"`
	Nome   string `json:"nome"`
}

// Adotar puxa para a célula o nome que o agente deu à conversa.
type Adotar struct {
	Celula string `json:"celula"`
}

// Aba troca a aba ativa de uma célula que tem várias. Passo positivo anda para
// a direita.
type Aba struct {
	Celula string `json:"celula"`
	Passo  int    `json:"passo"`
}

// Retomar sobe de novo uma célula parada ou caída.
type Retomar struct {
	Celula string `json:"celula"`
}

// Prompt manda trabalho para a célula sem entrar nela.
type Prompt struct {
	Celula string `json:"celula"`
	Texto  string `json:"texto"`
}

// Completar pede ao motor o complemento de um caminho digitado.
type Completar struct {
	Caminho     string `json:"caminho"`
	SoDiretorio bool   `json:"soDiretorio"`
}

// Completado é a resposta: o caminho completado e quantos candidatos casam.
type Completado struct {
	Caminho    string `json:"caminho"`
	Quantidade int    `json:"quantidade"`
}

// Tela lembra qual tela o usuário escolhe entre execuções.
type Tela struct {
	Tela string `json:"tela"`
}

// Buscar procura um termo no histórico da célula focada.
type Buscar struct {
	Celula string `json:"celula"`
	Termo  string `json:"termo"`
}

// Achado é uma linha do histórico que casou com a busca.
type Achado struct {
	Linha int    `json:"linha"`
	Texto string `json:"texto"`
}

// Achados é a resposta da busca.
type Achados struct {
	Celula string   `json:"celula"`
	Termo  string   `json:"termo"`
	Linhas []Achado `json:"linhas"`
}

// Docker age na stack ou num serviço do projeto. Nenhuma ação daqui é
// destrutiva: não existe derrubar com volume nem apagar nada.
type Docker struct {
	Projeto string `json:"projeto"`
	Acao    string `json:"acao"`              // sobe, para, reinicia, rebuilda, log
	Servico string `json:"servico,omitempty"` // vazio quando a ação é da stack inteira
}

// Servico é uma linha do painel Docker.
type Servico struct {
	Nome   string `json:"nome"`
	Estado string `json:"estado"`
	Porta  string `json:"porta,omitempty"`
	Saude  string `json:"saude,omitempty"`
	Uptime string `json:"uptime,omitempty"`
}

// Servicos é a resposta do motor ao pedido de listar a stack do projeto. Acao
// diz de qual pedido esta resposta veio, para a tela saber quando o trabalho
// que ela mostrou como em andamento terminou.
type Servicos struct {
	Projeto string    `json:"projeto"`
	Arquivo string    `json:"arquivo"`
	Acao    string    `json:"acao,omitempty"`
	Servico string    `json:"servico,omitempty"`
	Lista   []Servico `json:"lista"`
	Erro    string    `json:"erro,omitempty"`
}

// Editor abre o diretório do projeto no editor configurado.
type Editor struct {
	Projeto string `json:"projeto"`
}

// IrParaLinha leva a leitura da célula até uma linha do histórico, que é onde a
// busca termina.
type IrParaLinha struct {
	Celula string `json:"celula"`
	Linha  int    `json:"linha"`
}

// TipoCelula é a ficha de um tipo, como o formulário precisa conhecer. Vem do
// motor para que a tela não precise saber que tipos existem.
type TipoCelula struct {
	Tipo            string `json:"tipo"`
	RotuloAlvo      string `json:"rotuloAlvo,omitempty"`
	CompletaArquivo bool   `json:"completaArquivo,omitempty"`
	AceitaPrompt    bool   `json:"aceitaPrompt,omitempty"`
	Conversa        bool   `json:"conversa,omitempty"`
}

// Resumo é a resposta ao pedido de status, em texto pronto para o terminal.
type Resumo struct {
	Texto string `json:"texto"`
}

// Erro é o que o motor devolve quando um pedido não pôde ser atendido.
type Erro struct {
	Mensagem string `json:"mensagem"`
}

// Celula é o retrato de uma célula pronto para desenhar.
type Celula struct {
	ID      string   `json:"id"`
	Tipo    string   `json:"tipo"`
	Nome    string   `json:"nome"`
	Estado  string   `json:"estado"`
	Linhas  []string `json:"linhas"`
	CursorX int      `json:"cursorX"`
	CursorY int      `json:"cursorY"`
	Rolagem int      `json:"rolagem"`
	AoVivo  bool     `json:"aoVivo"`
	// Abas e Aba existem nas células que têm mais de um agente por dentro. A
	// tela desenha as abas no lugar do tipo.
	Abas []string `json:"abas,omitempty"`
	Aba  string   `json:"aba,omitempty"`
}

// Projeto é uma coluna do mosaico.
type Projeto struct {
	ID         string `json:"id"`
	Caminho    string `json:"caminho"`
	Nome       string `json:"nome"`
	Cor        int    `json:"cor"`
	TemCompose bool   `json:"temCompose"`
	// Docker é o resumo curto da stack para a tira: vazio sem compose,
	// "parado", ou "4/5" quando há serviços de pé.
	Docker  string   `json:"docker,omitempty"`
	Celulas []Celula `json:"celulas"`
}

// Estado é o retrato inteiro. O motor manda isto sempre que algo muda.
type Estado struct {
	Projetos []Projeto    `json:"projetos"`
	Tipos    []TipoCelula `json:"tipos"`
	Tela     string       `json:"tela,omitempty"`
	Quota    *Quota       `json:"quota,omitempty"`
	Aviso    string       `json:"aviso"`
}

// Quota é o consumo da janela de 5 horas mostrado na barra de título.
type Quota struct {
	Percentual int    `json:"percentual"`
	Vira       string `json:"vira"` // quanto falta para a janela virar, já formatado
}

// Mensagem é o envelope que trafega no socket.
type Mensagem struct {
	Tipo  string          `json:"tipo"`
	Dados json.RawMessage `json:"dados,omitempty"`
}

// Empacotar embrulha um valor no envelope do seu tipo.
func Empacotar(tipo string, dados any) (Mensagem, error) {
	bruto, err := json.Marshal(dados)
	if err != nil {
		return Mensagem{}, err
	}
	return Mensagem{Tipo: tipo, Dados: bruto}, nil
}

// Desempacotar tira o valor de dentro do envelope.
func Desempacotar[T any](m Mensagem) (T, error) {
	var v T
	err := json.Unmarshal(m.Dados, &v)
	return v, err
}
