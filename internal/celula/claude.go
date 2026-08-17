package celula

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	Registrar(Descritor{
		Tipo:         "claude",
		Ordem:        10,
		AceitaPrompt: true,
		Conversa:     true,
	}, func() Celula { return &Claude{} })
}

// marcadoresDoClaude é o que o Claude Code escreve na própria interface: o
// primeiro enquanto o turno está em andamento, o segundo enquanto ele espera
// um sim ou não.
var marcadoresDoClaude = Marcadores{
	// O primeiro é o das versões que anunciam a interrupção; o segundo é o
	// contador de tokens que a barra de trabalho mostra enquanto o turno corre.
	Trabalho: []string{"esc to interrupt", "tokens)"},
	Pergunta: []string{
		"No, and tell Claude what to do differently",
		"Do you want to proceed?",
		"1. Yes, I trust this folder",
	},
}

// Claude é o Claude Code rodando no diretório do projeto.
type Claude struct {
	Agente
}

func (c *Claude) Nascer(cfg Config) error {
	programa := cfg.Programa
	if programa == "" {
		programa = "claude"
	}
	c.comandoRenomear = cfg.ComandoRenomear
	if c.comandoRenomear == "" {
		c.comandoRenomear = "/rename"
	}
	c.lerNome = nomeDaConversaDoClaude

	// A conversa tem identidade própria: com ela o agente reata de onde parou
	// depois de uma queda da WSL, e é por ela que o nome da conversa é lido.
	args := append([]string{}, cfg.Args...)
	if cfg.Conversa == "" {
		cfg.Conversa = novaIdentidadeDeConversa()
		if cfg.AoDescobrirConversa != nil {
			cfg.AoDescobrirConversa(cfg.Conversa)
		}
	}
	// Retomar só vale se a conversa chegou a existir em disco. Uma célula que
	// nasceu e nunca recebeu um pedido não tem transcrição, e pedir para
	// retomá-la faria o agente morrer na largada — ela recomeça com a mesma
	// identidade.
	if temTranscricao(cfg.Diretorio, cfg.Conversa) {
		args = append(args, "--resume", cfg.Conversa)
	} else {
		args = append(args, "--session-id", cfg.Conversa)
	}

	return c.nascer(cfg, programa, args, marcadoresDoClaude)
}

// temTranscricao diz se a conversa já existe no disco do agente.
func temTranscricao(diretorio, conversa string) bool {
	if conversa == "" {
		return false
	}
	pasta, err := pastaDoClaude(diretorio)
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(pasta, conversa+".jsonl"))
	return err == nil && !info.IsDir()
}

// pastaDoClaude é onde o Claude Code guarda a transcrição das conversas de um
// diretório: o caminho vira o nome da pasta com as barras trocadas por traço.
func pastaDoClaude(diretorio string) (string, error) {
	casa, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	apelido := strings.ReplaceAll(strings.TrimSuffix(diretorio, "/"), "/", "-")
	apelido = strings.ReplaceAll(apelido, ".", "-")
	return filepath.Join(casa, ".claude", "projects", apelido), nil
}

// nomeDaConversaDoClaude lê na transcrição o nome da conversa. O nome escolhido
// à mão tem prioridade sobre o título que o agente gerou sozinho.
func nomeDaConversaDoClaude(diretorio, conversa string) (string, error) {
	pasta, err := pastaDoClaude(diretorio)
	if err != nil {
		return "", err
	}
	arquivo := filepath.Join(pasta, conversa+".jsonl")
	if conversa == "" {
		arquivo, err = transcricaoMaisRecente(pasta)
		if err != nil {
			return "", err
		}
	}

	aberto, err := os.Open(arquivo)
	if err != nil {
		return "", fmt.Errorf("a conversa ainda não tem transcrição")
	}
	defer aberto.Close()

	var escolhido, automatico string
	leitor := bufio.NewScanner(aberto)
	leitor.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for leitor.Scan() {
		var linha struct {
			CustomTitle string `json:"customTitle"`
			AITitle     string `json:"aiTitle"`
		}
		if err := json.Unmarshal(leitor.Bytes(), &linha); err != nil {
			continue
		}
		if linha.CustomTitle != "" {
			escolhido = linha.CustomTitle
		}
		if linha.AITitle != "" {
			automatico = linha.AITitle
		}
	}
	if escolhido != "" {
		return escolhido, nil
	}
	return automatico, nil
}

// transcricaoMaisRecente serve para a conversa cuja identidade se perdeu.
func transcricaoMaisRecente(pasta string) (string, error) {
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		return "", fmt.Errorf("a conversa ainda não tem transcrição")
	}
	var escolhida string
	var maisNova time.Time
	for _, entrada := range entradas {
		if entrada.IsDir() || !strings.HasSuffix(entrada.Name(), ".jsonl") {
			continue
		}
		info, err := entrada.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(maisNova) {
			maisNova, escolhida = info.ModTime(), filepath.Join(pasta, entrada.Name())
		}
	}
	if escolhida == "" {
		return "", fmt.Errorf("a conversa ainda não tem transcrição")
	}
	return escolhida, nil
}
