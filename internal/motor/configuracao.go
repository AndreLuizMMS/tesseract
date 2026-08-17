package motor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/andreluiz/tesseract/internal/motor/historico"
)

// PerfilAgente é como um tipo de agente sobe nesta máquina.
type PerfilAgente struct {
	Programa        string   `json:"programa,omitempty"`
	Args            []string `json:"args,omitempty"`
	ComandoRenomear string   `json:"comandoRenomear,omitempty"`
	// MarcadoresTrabalho e MarcadoresPergunta trocam o que a célula procura na
	// tela do agente para saber em que estado de turno ele está. Vazios, valem
	// os do tipo — que é o normal.
	MarcadoresTrabalho []string `json:"marcadoresTrabalho,omitempty"`
	MarcadoresPergunta []string `json:"marcadoresPergunta,omitempty"`
}

// Configuracao é o que o usuário pode mudar sem tocar em código.
type Configuracao struct {
	// Editor é o comando que `ctrl-e` roda no diretório do projeto: `cursor
	// /caminho/do/projeto` abre a IDE ali.
	Editor string `json:"editor,omitempty"`
	// Som e Notificar ligam e desligam os dois avisos, separadamente.
	Som       bool `json:"som"`
	Notificar bool `json:"notificar"`
	// ComandoNotificacao troca o notificador detectado por outro. O programa
	// recebe o título e o corpo como os dois últimos argumentos.
	ComandoNotificacao string `json:"comandoNotificacao,omitempty"`
	// TetoHistorico é o tamanho máximo do histórico de cada célula, em bytes.
	TetoHistorico int64 `json:"tetoHistorico,omitempty"`
	// Agentes é o perfil de cada tipo de agente.
	Agentes map[string]PerfilAgente `json:"agentes,omitempty"`
}

// ConfiguracaoPadrao é o que vale quando não há arquivo de configuração — e o
// que vale para cada campo que o arquivo não trouxer.
func ConfiguracaoPadrao() Configuracao {
	return Configuracao{
		Editor:        "cursor",
		Som:           true,
		Notificar:     true,
		TetoHistorico: historico.TetoPadrao,
		Agentes:       map[string]PerfilAgente{},
	}
}

// CaminhoConfiguracao é onde o arquivo mora.
func CaminhoConfiguracao() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "tesseract", "config.json")
	}
	casa, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "tesseract-config.json")
	}
	return filepath.Join(casa, ".config", "tesseract", "config.json")
}

// CarregarConfiguracao lê a configuração. Arquivo ausente não é erro: o padrão
// serve. Arquivo ilegível é erro, porque calar significaria ignorar o que o
// usuário escreveu.
func CarregarConfiguracao(caminho string) (Configuracao, error) {
	config := ConfiguracaoPadrao()
	conteudo, err := os.ReadFile(caminho)
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(conteudo, &config); err != nil {
		return ConfiguracaoPadrao(), err
	}
	if config.TetoHistorico <= 0 {
		config.TetoHistorico = historico.TetoPadrao
	}
	if config.Agentes == nil {
		config.Agentes = map[string]PerfilAgente{}
	}
	return config, nil
}
