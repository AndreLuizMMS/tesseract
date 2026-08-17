package celula

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func init() {
	Registrar(Descritor{
		Tipo:         "cursor",
		Ordem:        20,
		AceitaPrompt: true,
		Conversa:     true,
	}, func() Celula { return &Cursor{} })
}

// marcadoresDoCursor: o Cursor CLI não anuncia trabalho com um texto fixo, então
// o que sobra é a tela mudando. A pergunta bloqueante, essa ele escreve.
var marcadoresDoCursor = Marcadores{
	Pergunta: []string{"Allow this command?", "Run this command?"},
}

// Cursor é o Cursor CLI rodando no diretório do projeto.
type Cursor struct {
	Agente
}

func (c *Cursor) Nascer(cfg Config) error {
	perfil := cfg.Perfis["cursor"]
	programa := perfil.Programa
	if programa == "" {
		programa = "cursor-agent"
	}
	c.comandoRenomear = perfil.ComandoRenomear
	if c.comandoRenomear == "" {
		c.comandoRenomear = "/rename-chat"
	}
	c.lerNome = nomeDaConversaDoCursor
	c.acharConversa = conversaNovaDoCursor

	args := append([]string{}, perfil.Args...)
	// Mesma regra do Claude: só retoma o que existe no disco do agente.
	if cfg.Conversa != "" && temConversaDoCursor(cfg.Conversa) {
		args = append(args, "--resume", cfg.Conversa)
	}
	return c.nascer(cfg, perfil, programa, args, marcadoresDoCursor)
}

// temConversaDoCursor diz se a conversa ainda existe no disco do agente.
func temConversaDoCursor(conversa string) bool {
	casa, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	achados, err := filepath.Glob(filepath.Join(casa, ".cursor", "chats", "*", conversa, "meta.json"))
	return err == nil && len(achados) > 0
}

// fichaDeConversa é o que o Cursor CLI guarda sobre cada conversa.
type fichaDeConversa struct {
	Cwd       string `json:"cwd"`
	Title     string `json:"title"`
	CriadaEm  int64  `json:"createdAtMs"`
	MexidaEm  int64  `json:"updatedAtMs"`
	caminho   string
	identidad string
}

// fichasDoCursor lê as conversas que o Cursor tem para um diretório.
func fichasDoCursor(diretorio string) []fichaDeConversa {
	casa, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	// O Cursor guarda as conversas sob um hash do espaço de trabalho que não dá
	// para recalcular daqui, então a busca é por todas e o filtro é o próprio
	// campo de diretório de cada uma.
	metas, err := filepath.Glob(filepath.Join(casa, ".cursor", "chats", "*", "*", "meta.json"))
	if err != nil {
		return nil
	}
	var fichas []fichaDeConversa
	for _, meta := range metas {
		conteudo, err := os.ReadFile(meta)
		if err != nil {
			continue
		}
		var ficha fichaDeConversa
		if err := json.Unmarshal(conteudo, &ficha); err != nil {
			continue
		}
		if ficha.Cwd != diretorio {
			continue
		}
		ficha.caminho = meta
		ficha.identidad = filepath.Base(filepath.Dir(meta))
		fichas = append(fichas, ficha)
	}
	return fichas
}

// conversaNovaDoCursor descobre qual conversa o Cursor abriu para esta célula:
// a que nasceu depois que a célula subiu.
func conversaNovaDoCursor(diretorio string, desde time.Time) string {
	limite := desde.Add(-2 * time.Second).UnixMilli()
	var escolhida string
	var maisNova int64
	for _, ficha := range fichasDoCursor(diretorio) {
		if ficha.CriadaEm < limite || ficha.CriadaEm < maisNova {
			continue
		}
		maisNova, escolhida = ficha.CriadaEm, ficha.identidad
	}
	return escolhida
}

// nomeDaConversaDoCursor lê o nome que o Cursor deu à conversa.
func nomeDaConversaDoCursor(diretorio, conversa string) (string, error) {
	fichas := fichasDoCursor(diretorio)
	if len(fichas) == 0 {
		return "", fmt.Errorf("o Cursor ainda não guardou nada desta pasta")
	}
	if conversa != "" {
		for _, ficha := range fichas {
			if ficha.identidad == conversa {
				return ficha.Title, nil
			}
		}
		return "", fmt.Errorf("a conversa ainda não tem nome")
	}
	var escolhida fichaDeConversa
	for _, ficha := range fichas {
		if ficha.MexidaEm >= escolhida.MexidaEm {
			escolhida = ficha
		}
	}
	return escolhida.Title, nil
}
