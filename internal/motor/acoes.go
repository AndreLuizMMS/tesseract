package motor

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

// RenomearAutomatico manda o agente escolher e escrever o próprio nome da
// conversa. O nome ainda não existe: quem aplica na célula é o vigia, quando
// o turno termina (ver AdotarNomeDoAgente).
func (m *Motor) RenomearAutomatico(idCelula string) error {
	c := m.acharCelula(idCelula)
	if c == nil || c.viva == nil {
		return fmt.Errorf("célula %s não existe", idCelula)
	}
	comConversa, tem := c.viva.(celula.ComConversa)
	if !tem {
		return fmt.Errorf("a célula %s não tem conversa com nome", c.nome)
	}
	if err := comConversa.PedirNomeAutomatico(); err != nil {
		return err
	}
	m.mu.Lock()
	c.aguardandoNome = true
	m.mu.Unlock()
	return nil
}

// AdotarNomeDoAgente puxa para a célula o nome que o agente deu à conversa e
// desarma a espera, com sucesso ou sem — tentar de novo sozinho não ajuda.
func (m *Motor) AdotarNomeDoAgente(idCelula string) error {
	c := m.acharCelula(idCelula)
	if c == nil || c.viva == nil {
		return fmt.Errorf("célula %s não existe", idCelula)
	}
	m.mu.Lock()
	c.aguardandoNome = false
	m.mu.Unlock()

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

// AbrirNoEditor abre o diretório do projeto na IDE configurada.
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
	ambiente := comTela(os.Environ())
	nomeDoBinario, argumentos := campos[0], append(campos[1:], caminho)
	// No WSL o binário Linux da IDE não tem tela para desenhar e morre — quem
	// abre a pasta é sempre a instalação do lado Windows, no PATH de lá. É ela
	// quem decide se reaproveita janela aberta ou sobe uma nova: o socket que a
	// IDE deixa no WSL continua respondendo mesmo com a janela já fechada, então
	// não dá pra usar "socket vivo" como prova de janela aberta.
	if cmdExe, uri, ok := abrirPeloWindows(campos[0], caminho); ok {
		nomeDoBinario, argumentos = cmdExe, []string{"/c", "start", "", campos[0], "--folder-uri", uri}
	} else if cli, janela := ideJaAberta(campos[0]); cli != "" {
		// Fora do WSL (host remoto por SSH, por exemplo), o socket é o único
		// sinal que existe, e aí ele é confiável — a IDE remota não deixa o
		// servidor pendurado depois que a janela local fecha.
		nomeDoBinario = cli
		ambiente = append(ambiente, "VSCODE_IPC_HOOK_CLI="+janela)
	}
	comando := exec.Command(nomeDoBinario, argumentos...)
	comando.Dir = caminho
	comando.Env = ambiente
	saida := &strings.Builder{}
	comando.Stdout, comando.Stderr = saida, saida
	if err := comando.Start(); err != nil {
		return fmt.Errorf("não consegui abrir %s: %w", editor, err)
	}

	// A IDE que abre janela continua viva; a que não abriu morre na hora. Meio
	// segundo separa as duas, e é o que faz o erro chegar na tela em vez de o
	// atalho parecer não ter feito nada.
	morreu := make(chan error, 1)
	go func() { morreu <- comando.Wait() }()
	select {
	case err := <-morreu:
		if err != nil {
			return fmt.Errorf("%s não abriu: %s", nomeDoBinario, primeiraLinha(saida.String(), err))
		}
	case <-time.After(500 * time.Millisecond):
	}
	return nil
}

// caminhoDoCmdExe é fixo por ser onde o Windows sempre instala o próprio
// interpretador de comando — não tem variação de máquina para descobrir.
const caminhoDoCmdExe = "/mnt/c/Windows/System32/cmd.exe"

// abrirPeloWindows monta o comando que sobe a IDE do lado Windows quando o
// binário Linux dela não tem como abrir janela nenhuma (é o caso sem nenhuma
// janela já aberta). Só se aplica dentro do WSL: fora dele não há Windows do
// outro lado para pedir socorro.
func abrirPeloWindows(nomeEditor, caminho string) (cmdExe string, folderURI string, ok bool) {
	if _, err := os.Stat(caminhoDoCmdExe); err != nil {
		return "", "", false
	}
	distro := distroWSL()
	if distro == "" {
		return "", "", false
	}
	return caminhoDoCmdExe, "vscode-remote://wsl+" + distro + caminho, true
}

// distroWSL pergunta ao Windows o nome da distro registrada. Com mais de uma
// instalada, fica ambíguo — toma a primeira da lista; ambiente ponytail: uma
// distro só, então não há o que escolher de verdade.
func distroWSL() string {
	saida, err := exec.Command(caminhoDoCmdExe, "/c", "wsl.exe", "-l", "-q").Output()
	if err != nil {
		return ""
	}
	for _, linha := range strings.Split(string(saida), "\n") {
		linha = strings.TrimSpace(strings.Map(func(r rune) rune {
			if r == 0 {
				return -1
			}
			return r
		}, linha))
		if linha != "" {
			return linha
		}
	}
	return ""
}

// ideJaAberta procura o CLI remoto da IDE e o socket da janela viva. Só devolve
// os dois juntos: CLI sem janela aberta não abre nada, e janela sem CLI não tem
// por onde receber o pedido.
func ideJaAberta(nome string) (cli string, janela string) {
	casa, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	achados, _ := filepath.Glob(filepath.Join(casa, "."+nome+"-server", "bin", "*", "bin", "remote-cli", nome))
	if len(achados) == 0 {
		return "", ""
	}
	janela = janelaViva(os.Getenv("XDG_RUNTIME_DIR"))
	if janela == "" {
		return "", ""
	}
	return maisNovo(achados), janela
}

// janelaViva é o socket da janela que ainda responde. As IDEs deixam para trás
// o socket de toda janela que já fechou, então só o teste de conexão separa.
func janelaViva(dirDeExecucao string) string {
	if dirDeExecucao == "" {
		return ""
	}
	sockets, _ := filepath.Glob(filepath.Join(dirDeExecucao, "vscode-ipc-*.sock"))
	for _, s := range ordenadosDoMaisNovo(sockets) {
		if conexao, err := net.DialTimeout("unix", s, 200*time.Millisecond); err == nil {
			_ = conexao.Close()
			return s
		}
	}
	return ""
}

// maisNovo escolhe o caminho mexido por último — a instalação mais recente.
func maisNovo(caminhos []string) string {
	ordenados := ordenadosDoMaisNovo(caminhos)
	if len(ordenados) == 0 {
		return ""
	}
	return ordenados[0]
}

func ordenadosDoMaisNovo(caminhos []string) []string {
	idade := func(c string) int64 {
		info, err := os.Stat(c)
		if err != nil {
			return 0
		}
		return info.ModTime().Unix()
	}
	ordenados := append([]string(nil), caminhos...)
	sort.Slice(ordenados, func(i, j int) bool { return idade(ordenados[i]) > idade(ordenados[j]) })
	return ordenados
}

// comTela completa o ambiente com o servidor gráfico quando ele falta. O motor
// roda como serviço, e serviço não herda DISPLAY nenhum — sem isso a IDE sobe
// sem tela para desenhar e morre calada.
func comTela(ambiente []string) []string {
	tem := func(nome string) bool {
		for _, linha := range ambiente {
			if strings.HasPrefix(linha, nome+"=") && len(linha) > len(nome)+1 {
				return true
			}
		}
		return false
	}
	if !tem("DISPLAY") {
		if _, err := os.Stat("/tmp/.X11-unix/X0"); err == nil {
			ambiente = append(ambiente, "DISPLAY=:0")
		}
	}
	if casa := os.Getenv("XDG_RUNTIME_DIR"); casa != "" && !tem("WAYLAND_DISPLAY") {
		if _, err := os.Stat(filepath.Join(casa, "wayland-0")); err == nil {
			ambiente = append(ambiente, "WAYLAND_DISPLAY=wayland-0")
		}
	}
	return ambiente
}

// primeiraLinha resume o que o editor reclamou, para o aviso caber no rodapé.
func primeiraLinha(saida string, padrao error) string {
	for _, linha := range strings.Split(saida, "\n") {
		if linha = strings.TrimSpace(linha); linha != "" {
			return linha
		}
	}
	return padrao.Error()
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
