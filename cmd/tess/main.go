// Comando ts: abre a tela do Tesseract, conectando ao motor — e sobe o motor
// se ele não estiver rodando.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	_ "github.com/andreluiz/tesseract/internal/celula"
	"github.com/andreluiz/tesseract/internal/motor"
	"github.com/andreluiz/tesseract/internal/protocolo"
	"github.com/andreluiz/tesseract/internal/tela"
	"github.com/andreluiz/tesseract/internal/tema"
)

// esperaDoMotor é quanto tempo a tela dá para o motor abrir o socket antes de
// desistir.
const esperaDoMotor = 5 * time.Second

const uso = `⧉ ts — mosaico de agentes em terminal

  ts                 abre a tela, subindo o motor se preciso
  ts novo <dir>      adiciona um projeto sem abrir a tela
  ts status          estado do motor e resumo de projetos e células
  ts stop            desliga o motor e todas as células
  ts reset           apaga o estado salvo e derruba tudo, preservando a configuração
  ts motor           roda o motor em primeiro plano (é o que o systemd chama)
`

func main() {
	comando := ""
	if len(os.Args) > 1 {
		comando = os.Args[1]
	}

	var err error
	switch comando {
	case "":
		err = abrirTela()
	case "motor":
		err = rodarMotor()
	case "novo":
		err = adicionarProjeto(os.Args[2:])
	case "status":
		err = mostrarStatus()
	case "stop":
		err = pararMotor()
	case "reset":
		err = apagarEstado()
	case "-h", "--help", "ajuda", "help":
		fmt.Print(uso)
	default:
		fmt.Fprintf(os.Stderr, "comando desconhecido: %s\n\n%s", comando, uso)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

// abrirTela conecta ao motor, subindo-o se preciso, e desenha.
func abrirTela() error {
	// A abertura roda enquanto o motor é procurado, numa linha de execução, e
	// a conexão corre na outra. O tempo da animação é tempo que já ia passar
	// de qualquer jeito — nenhuma espera é inventada para exibi-la.
	abertura := tela.NovaAbertura(os.Stderr, animarAbertura())

	type chegada struct {
		cliente *tela.Cliente
		inicial protocolo.Estado
		levou   time.Duration
		err     error
	}
	conectou := make(chan chegada, 1)
	go func() {
		// O cronômetro mora aqui dentro: o que ele conta é o motor montando a
		// grade, não a animação rodando ao lado. Medir a animação junto seria
		// mentir num número que existe justamente para ser verdade.
		comeco := time.Now()
		cliente, err := conectarOuSubir()
		if err != nil {
			conectou <- chegada{err: err}
			return
		}
		// O primeiro retrato chega assim que a tela conecta; ele é o ponto de
		// partida do desenho.
		inicial, err := primeiroEstado(cliente)
		conectou <- chegada{cliente: cliente, inicial: inicial, levou: time.Since(comeco), err: err}
	}()

	abertura.Montar()

	chegou := <-conectou
	if chegou.err != nil {
		if chegou.cliente != nil {
			chegou.cliente.Fechar()
		}
		return chegou.err
	}
	cliente, inicial := chegou.cliente, chegou.inicial
	defer cliente.Fechar()

	abertura.Contar(contagemDoMotor(inicial, chegou.levou))
	// Grade vazia começa com um shell aqui mesmo: sempre há o que olhar.
	if len(inicial.Projetos) == 0 {
		diretorio, err := os.Getwd()
		if err != nil {
			return err
		}
		// Sem tipo: quem decide é o motor, e a sessão já vem com as abas dos
		// agentes por dentro.
		if err := cliente.Enviar(protocolo.TipoCriar, protocolo.Criar{Caminho: diretorio}); err != nil {
			return err
		}
	}

	modelo := tela.NovoModelo(cliente, inicial)
	programa := tea.NewProgram(modelo)
	modelo.Ouvir(programa)
	_, err := programa.Run()
	return err
}

// animarAbertura diz se a abertura roda como animação. Sem terminal de
// verdade ela viraria lixo de escape num arquivo de log, e quem não quer
// esperar por ela desliga no ambiente.
func animarAbertura() bool {
	if _, desligado := os.LookupEnv("TESSERACT_SEM_ABERTURA"); desligado {
		return false
	}
	info, err := os.Stderr.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// contagemDoMotor é o que o motor devolveu: a prova de que a grade voltou
// inteira, dita em número. Some junto com a abertura quando a tela cheia abre.
func contagemDoMotor(estado protocolo.Estado, levou time.Duration) []string {
	celulas := 0
	for _, projeto := range estado.Projetos {
		celulas += len(projeto.Celulas)
	}
	return []string{
		tela.LinhaDoMotor("motor de sessão: vivo"),
		tela.LinhaDoMotor(fmt.Sprintf("%s · %s · mesma posição",
			contar(celulas, "célula recuperada", "células recuperadas"),
			contar(len(estado.Projetos), "projeto", "projetos"))),
		tela.LinhaDoMotor(fmt.Sprintf("grade montada em %dms", levou.Milliseconds())),
	}
}

// contar escreve o número junto do substantivo na forma certa. Uma célula não
// é "1 células".
func contar(quantos int, singular, plural string) string {
	if quantos == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(quantos) + " " + plural
}

// primeiroEstado espera o retrato que o motor manda assim que a tela conecta.
func primeiroEstado(cliente *tela.Cliente) (protocolo.Estado, error) {
	for {
		envelope, err := cliente.Receber()
		if err != nil {
			return protocolo.Estado{}, err
		}
		if envelope.Tipo != protocolo.TipoEstado {
			continue
		}
		return protocolo.Desempacotar[protocolo.Estado](envelope)
	}
}

// rodarMotor é o serviço: dono dos processos, do histórico e do estado.
func rodarMotor() error {
	dirEstado := motor.DiretorioEstado()
	config, err := motor.CarregarConfiguracao(motor.CaminhoConfiguracao())
	if err != nil {
		fmt.Fprintln(os.Stderr, "aviso: configuração ilegível, seguindo com o padrão:", err)
	}

	// O serviço não herda o terminal do usuário: sem isto, os agentes e as
	// ferramentas que eles chamam não existiriam para o motor.
	motor.HerdarCaminhoDoLogin()

	m := motor.Novo(dirEstado, config)
	if err := m.Iniciar(); err != nil {
		fmt.Fprintln(os.Stderr, "aviso:", err)
	}
	go m.Vigiar()

	servidor, erroSocket := motor.Servir(m, motor.CaminhoSocket())
	if errors.Is(erroSocket, motor.ErrMotorJaRodando) {
		// Quem pediu já tem o que queria: sair bem evita o serviço entrar em
		// laço de reinício por causa de um motor que já está de pé.
		fmt.Fprintln(os.Stderr, "já existe um motor rodando")
		m.Encerrar()
		return nil
	}
	if erroSocket != nil {
		return erroSocket
	}
	defer servidor.Fechar()

	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sinais:
	case <-servidor.Parado():
	}
	m.Encerrar()
	return nil
}

// adicionarProjeto cria um projeto sem abrir a tela. Projeto nasce junto com a
// primeira célula, então a célula vem junto.
func adicionarProjeto(argumentos []string) error {
	if len(argumentos) == 0 {
		return errors.New("uso: ts novo <dir> [tipo]")
	}
	caminho, err := filepath.Abs(argumentos[0])
	if err != nil {
		return err
	}
	tipo := ""
	if len(argumentos) > 1 {
		tipo = argumentos[1]
	}

	cliente, err := conectarOuSubir()
	if err != nil {
		return err
	}
	defer cliente.Fechar()

	if err := cliente.Enviar(protocolo.TipoCriar, protocolo.Criar{Caminho: caminho, Tipo: tipo}); err != nil {
		return err
	}
	// A resposta é o retrato com a célula nova, ou o erro do motor.
	prazo := time.Now().Add(10 * time.Second)
	for time.Now().Before(prazo) {
		envelope, err := cliente.Receber()
		if err != nil {
			return err
		}
		switch envelope.Tipo {
		case protocolo.TipoErro:
			recado, _ := protocolo.Desempacotar[protocolo.Erro](envelope)
			return errors.New(recado.Mensagem)
		case protocolo.TipoEstado:
			estado, _ := protocolo.Desempacotar[protocolo.Estado](envelope)
			for _, projeto := range estado.Projetos {
				if projeto.Caminho == caminho {
					fmt.Printf("%s entrou na grade\n", projeto.Nome)
					return nil
				}
			}
		}
	}
	return errors.New("o motor não confirmou a criação")
}

// mostrarStatus imprime o estado do motor e o resumo da grade.
func mostrarStatus() error {
	cliente, err := tela.Conectar(motor.CaminhoSocket())
	if err != nil {
		fmt.Println("motor não está rodando")
		return nil
	}
	defer cliente.Fechar()

	if err := cliente.Enviar(protocolo.TipoStatus, struct{}{}); err != nil {
		return err
	}
	resumo, err := esperarResumo(cliente)
	if err != nil {
		return err
	}
	fmt.Println(tema.Glifo, resumo)
	return nil
}

// pararMotor desliga o motor e todas as células.
func pararMotor() error {
	cliente, err := tela.Conectar(motor.CaminhoSocket())
	if err != nil {
		fmt.Println("motor não está rodando")
		return nil
	}
	defer cliente.Fechar()

	if err := cliente.Enviar(protocolo.TipoParar, struct{}{}); err != nil {
		return err
	}
	resumo, err := esperarResumo(cliente)
	if err != nil {
		return err
	}
	fmt.Println(resumo)
	return nil
}

// apagarEstado derruba tudo e esquece a grade, preservando a configuração.
func apagarEstado() error {
	if err := pararMotor(); err != nil {
		return err
	}
	// O motor precisa soltar os arquivos antes de eles sumirem.
	esperarMotorSair()

	dirEstado := motor.DiretorioEstado()
	if err := os.Remove(motor.CaminhoEstado(dirEstado)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(filepath.Join(dirEstado, "historico")); err != nil {
		return err
	}
	fmt.Println("estado apagado; a configuração continua onde estava")
	return nil
}

func esperarMotorSair() {
	prazo := time.Now().Add(3 * time.Second)
	for time.Now().Before(prazo) {
		cliente, err := tela.Conectar(motor.CaminhoSocket())
		if err != nil {
			return
		}
		cliente.Fechar()
		time.Sleep(100 * time.Millisecond)
	}
}

func esperarResumo(cliente *tela.Cliente) (string, error) {
	for {
		envelope, err := cliente.Receber()
		if err != nil {
			return "", err
		}
		if envelope.Tipo != protocolo.TipoResumo {
			continue
		}
		resumo, err := protocolo.Desempacotar[protocolo.Resumo](envelope)
		if err != nil {
			return "", err
		}
		return resumo.Texto, nil
	}
}

// conectarOuSubir devolve uma conversa com o motor, subindo o serviço quando
// ele ainda não está de pé.
func conectarOuSubir() (*tela.Cliente, error) {
	if cliente, err := tela.Conectar(motor.CaminhoSocket()); err == nil {
		return cliente, nil
	}
	if err := subirMotor(); err != nil {
		return nil, err
	}
	limite := time.Now().Add(esperaDoMotor)
	for time.Now().Before(limite) {
		if cliente, err := tela.Conectar(motor.CaminhoSocket()); err == nil {
			return cliente, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, errors.New("o motor não respondeu")
}

// subirMotor lança o serviço em segundo plano, solto do terminal, para ele
// sobreviver a fechar a tela.
func subirMotor() error {
	// Com o serviço do systemd instalado, quem sobe o motor é ele — e aí o
	// motor volta sozinho depois de um `wsl --shutdown`.
	if _, err := exec.LookPath("systemctl"); err == nil {
		if err := exec.Command("systemctl", "--user", "start", "tesseract.service").Run(); err == nil {
			return nil
		}
	}

	eu, err := os.Executable()
	if err != nil {
		return err
	}
	dirEstado := motor.DiretorioEstado()
	if err := os.MkdirAll(dirEstado, 0o755); err != nil {
		return err
	}
	registro, err := os.OpenFile(filepath.Join(dirEstado, "motor.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer registro.Close()

	servico := exec.Command(eu, "motor")
	servico.Stdout = registro
	servico.Stderr = registro
	servico.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return servico.Start()
}
