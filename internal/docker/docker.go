// Package docker fala com o Docker Compose do projeto por linha de comando.
// Nenhuma ação daqui é destrutiva: não existe derrubar com volume, não existe
// apagar nada.
package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// arquivosDeCompose é a ordem de procura, só na raiz do projeto. Sem busca
// recursiva: projeto sem compose na raiz simplesmente não tem painel.
var arquivosDeCompose = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

// prazoDeLeitura limita quanto tempo esperamos o Docker responder uma consulta.
const prazoDeLeitura = 20 * time.Second

// prazoDeAcao é maior porque subir serviço baixa imagem.
const prazoDeAcao = 5 * time.Minute

// Servico é uma linha do painel.
type Servico struct {
	Nome   string `json:"nome"`
	Estado string `json:"estado"` // up, exited (1), created…
	Porta  string `json:"porta"`  // vazio quando o serviço não publica nada
	Saude  string `json:"saude"`  // vazio quando não há healthcheck
	Uptime string `json:"uptime"`
}

// Detectar acha o arquivo de compose na raiz do projeto. Devolve vazio quando
// não há nenhum.
func Detectar(diretorio string) string {
	for _, nome := range arquivosDeCompose {
		caminho := filepath.Join(diretorio, nome)
		if info, err := os.Stat(caminho); err == nil && !info.IsDir() {
			return caminho
		}
	}
	return ""
}

// Servicos lista o que a stack do projeto tem, de pé ou não.
func Servicos(diretorio, arquivo string) ([]Servico, error) {
	ctx, cancelar := context.WithTimeout(context.Background(), prazoDeLeitura)
	defer cancelar()

	saida, err := rodar(ctx, diretorio, arquivo, "ps", "--all", "--format", "json")
	if err != nil {
		return nil, err
	}
	return LerServicos(saida)
}

// Resumo é o que a tira do projeto mostra: quantos serviços estão de pé.
func Resumo(servicos []Servico) string {
	if len(servicos) == 0 {
		return "parado"
	}
	dePe := 0
	for _, servico := range servicos {
		if strings.HasPrefix(servico.Estado, "up") {
			dePe++
		}
	}
	if dePe == 0 {
		return "parado"
	}
	return strconv.Itoa(dePe) + "/" + strconv.Itoa(len(servicos))
}

// Agir executa uma ação no serviço, ou na stack inteira quando o serviço vem
// vazio. Ação desconhecida é recusada — é assim que nada destrutivo entra.
func Agir(diretorio, arquivo, acao, servico string) error {
	var argumentos []string
	switch acao {
	case "sobe":
		argumentos = []string{"up", "-d"}
	case "para":
		argumentos = []string{"stop"}
	case "reinicia":
		argumentos = []string{"restart"}
	case "rebuilda":
		argumentos = []string{"up", "-d", "--build"}
	default:
		return fmt.Errorf("ação desconhecida no Docker: %q", acao)
	}
	if servico != "" {
		argumentos = append(argumentos, servico)
	}

	ctx, cancelar := context.WithTimeout(context.Background(), prazoDeAcao)
	defer cancelar()
	_, err := rodar(ctx, diretorio, arquivo, argumentos...)
	return err
}

// ComandoDeLog é o comando que uma célula de log roda para acompanhar um
// serviço. Sai daqui para o painel e a célula não discordarem.
func ComandoDeLog(arquivo, servico string) (string, []string) {
	return "docker", []string{"compose", "--file", arquivo, "logs", "--follow", "--tail", "200", servico}
}

// LerServicos entende a saída de `docker compose ps --format json`, que vem um
// objeto por linha.
func LerServicos(saida []byte) ([]Servico, error) {
	var servicos []Servico
	leitor := bufio.NewScanner(strings.NewReader(string(saida)))
	leitor.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for leitor.Scan() {
		linha := strings.TrimSpace(leitor.Text())
		if linha == "" {
			continue
		}
		// Algumas versões entregam o array inteiro numa linha só.
		if strings.HasPrefix(linha, "[") {
			var lote []bruto
			if err := json.Unmarshal([]byte(linha), &lote); err != nil {
				return nil, err
			}
			for _, item := range lote {
				servicos = append(servicos, item.traduzir())
			}
			continue
		}
		var item bruto
		if err := json.Unmarshal([]byte(linha), &item); err != nil {
			return nil, err
		}
		servicos = append(servicos, item.traduzir())
	}
	return servicos, leitor.Err()
}

// bruto é o pedaço da saída do compose que interessa.
type bruto struct {
	Service    string `json:"Service"`
	State      string `json:"State"`
	Status     string `json:"Status"`
	Health     string `json:"Health"`
	ExitCode   int    `json:"ExitCode"`
	Publishers []struct {
		PublishedPort int    `json:"PublishedPort"`
		TargetPort    int    `json:"TargetPort"`
		Protocol      string `json:"Protocol"`
	} `json:"Publishers"`
}

func (b bruto) traduzir() Servico {
	servico := Servico{
		Nome:   b.Service,
		Estado: estadoLegivel(b),
		Porta:  portaPublicada(b),
		Saude:  saude(b),
		Uptime: uptime(b),
	}
	return servico
}

func estadoLegivel(b bruto) string {
	switch b.State {
	case "running":
		return "up"
	case "exited":
		return "exited (" + strconv.Itoa(b.ExitCode) + ")"
	case "":
		return "—"
	}
	return b.State
}

// portaPublicada devolve a primeira porta que o serviço abre para fora.
func portaPublicada(b bruto) string {
	for _, publicada := range b.Publishers {
		if publicada.PublishedPort > 0 {
			return ":" + strconv.Itoa(publicada.PublishedPort)
		}
	}
	return ""
}

// saude lê o healthcheck, do campo próprio ou do texto do status.
func saude(b bruto) string {
	if b.Health != "" {
		return traduzirSaude(b.Health)
	}
	abre := strings.Index(b.Status, "(")
	fecha := strings.Index(b.Status, ")")
	if abre >= 0 && fecha > abre {
		return traduzirSaude(b.Status[abre+1 : fecha])
	}
	return ""
}

func traduzirSaude(bruta string) string {
	switch strings.TrimSpace(strings.ToLower(bruta)) {
	case "healthy":
		return "saudável"
	case "unhealthy":
		return "doente"
	case "starting", "health: starting":
		return "subindo"
	}
	return ""
}

// uptime tira do status o tempo de pé, sem o "Up" e sem a saúde.
func uptime(b bruto) string {
	if !strings.HasPrefix(b.Status, "Up") {
		return ""
	}
	texto := strings.TrimSpace(strings.TrimPrefix(b.Status, "Up"))
	if abre := strings.Index(texto, "("); abre >= 0 {
		texto = strings.TrimSpace(texto[:abre])
	}
	return encurtarTempo(texto)
}

// encurtarTempo troca "2 hours 14 minutes" por "2h14m", que é o que cabe na
// coluna.
func encurtarTempo(texto string) string {
	partes := strings.Fields(texto)
	var curto strings.Builder
	for i := 0; i+1 < len(partes); i += 2 {
		numero := partes[i]
		unidade := partes[i+1]
		switch {
		case strings.HasPrefix(unidade, "second"):
			curto.WriteString(numero + "s")
		case strings.HasPrefix(unidade, "minute"):
			curto.WriteString(numero + "m")
		case strings.HasPrefix(unidade, "hour"):
			curto.WriteString(numero + "h")
		case strings.HasPrefix(unidade, "day"):
			curto.WriteString(numero + "d")
		case strings.HasPrefix(unidade, "week"):
			curto.WriteString(numero + "sem")
		case strings.HasPrefix(unidade, "month"):
			curto.WriteString(numero + "mes")
		default:
			curto.WriteString(numero + unidade)
		}
	}
	if curto.Len() == 0 {
		return texto
	}
	return curto.String()
}

// rodar executa o docker compose do projeto e devolve a saída.
func rodar(ctx context.Context, diretorio, arquivo string, argumentos ...string) ([]byte, error) {
	todos := append([]string{"compose", "--file", arquivo}, argumentos...)
	comando := exec.CommandContext(ctx, "docker", todos...)
	comando.Dir = diretorio
	saida, err := comando.Output()
	if err != nil {
		var comErro *exec.ExitError
		if errors.As(err, &comErro) && len(comErro.Stderr) > 0 {
			return nil, fmt.Errorf("docker compose %s: %s", strings.Join(argumentos, " "), primeiraLinha(comErro.Stderr))
		}
		return nil, fmt.Errorf("docker compose %s: %w", strings.Join(argumentos, " "), err)
	}
	return saida, nil
}

func primeiraLinha(saida []byte) string {
	texto := strings.TrimSpace(string(saida))
	if corte := strings.Index(texto, "\n"); corte > 0 {
		return texto[:corte]
	}
	return texto
}
