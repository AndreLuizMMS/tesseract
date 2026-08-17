package motor

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"

	_ "github.com/andreluiz/tesseract/internal/celula"
	"github.com/andreluiz/tesseract/internal/protocolo"
)

func esperarPor(t *testing.T, prazo time.Duration, condicao func() bool) bool {
	t.Helper()
	limite := time.Now().Add(prazo)
	for time.Now().Before(limite) {
		if condicao() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return condicao()
}

// colarEEntrar faz o que o usuário faz dentro da célula: cola o comando e
// depois manda o enter. A colagem vai marcada como colagem, então a quebra de
// linha não pode vir junto do texto colado.
func colarEEntrar(t *testing.T, m *Motor, id, comando string) {
	t.Helper()
	if err := m.Tecla(protocolo.Tecla{Celula: id, Colar: comando}); err != nil {
		t.Fatalf("colar: %v", err)
	}
	if err := m.Tecla(protocolo.Tecla{Celula: id, Codigo: vt.KeyEnter}); err != nil {
		t.Fatalf("enter: %v", err)
	}
}

// configuracaoDeTeste desliga os avisos: teste não apita nem sobe notificação.
func configuracaoDeTeste() Configuracao {
	config := ConfiguracaoPadrao()
	config.Som, config.Notificar = false, false
	config.TetoHistorico = 64 << 10
	return config
}

func motorDeTeste(t *testing.T) *Motor {
	t.Helper()
	m := Novo(t.TempDir(), configuracaoDeTeste())
	if err := m.Iniciar(); err != nil {
		t.Fatalf("iniciar motor: %v", err)
	}
	t.Cleanup(m.Encerrar)
	return m
}

// TestCriarNasceProjetoJunto — criar projeto e criar célula são o mesmo gesto.
func TestCriarNasceProjetoJunto(t *testing.T) {
	m := motorDeTeste(t)
	dir := t.TempDir()

	id, err := m.Criar(protocolo.Criar{Caminho: dir, Tipo: "bash"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	retrato := m.Retrato()
	if len(retrato.Projetos) != 1 || len(retrato.Projetos[0].Celulas) != 1 {
		t.Fatalf("esperava um projeto com uma célula: %#v", retrato)
	}
	if retrato.Projetos[0].Celulas[0].ID != id {
		t.Fatalf("a célula criada não é a que está na tela")
	}
	if nome := retrato.Projetos[0].Celulas[0].Nome; nome != "bash 1" {
		t.Fatalf("nome automático veio %q, esperado \"bash 1\"", nome)
	}

	// Segunda célula no mesmo caminho entra no mesmo projeto.
	if _, err := m.Criar(protocolo.Criar{Caminho: dir, Tipo: "bash"}); err != nil {
		t.Fatalf("criar segunda: %v", err)
	}
	retrato = m.Retrato()
	if len(retrato.Projetos) != 1 || len(retrato.Projetos[0].Celulas) != 2 {
		t.Fatalf("a segunda célula devia entrar no mesmo projeto: %#v", retrato)
	}
}

// TestMatarUltimaCelulaTiraProjetoDaTela e o disco continua intacto.
func TestMatarUltimaCelulaTiraProjetoDaTela(t *testing.T) {
	m := motorDeTeste(t)
	dir := t.TempDir()
	marca := filepath.Join(dir, "arquivo-do-usuario.txt")
	if err := os.WriteFile(marca, []byte("intacto"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	primeira, err := m.Criar(protocolo.Criar{Caminho: dir, Tipo: "bash"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	segunda, err := m.Criar(protocolo.Criar{Caminho: dir, Tipo: "bash"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}

	if m.EhUltimaCelula(primeira) {
		t.Fatal("com duas células, nenhuma é a última")
	}
	if err := m.Matar(primeira); err != nil {
		t.Fatalf("matar: %v", err)
	}
	if len(m.Retrato().Projetos) != 1 {
		t.Fatal("matar célula não-última não pode remover o projeto")
	}

	if !m.EhUltimaCelula(segunda) {
		t.Fatal("a que sobrou é a última do projeto")
	}
	if err := m.Matar(segunda); err != nil {
		t.Fatalf("matar: %v", err)
	}
	if len(m.Retrato().Projetos) != 0 {
		t.Fatal("o projeto devia ter saído da tela com a última célula")
	}

	if conteudo, err := os.ReadFile(marca); err != nil || string(conteudo) != "intacto" {
		t.Fatalf("o diretório do projeto foi mexido: %v %q", err, conteudo)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("o diretório do projeto sumiu do disco: %v", err)
	}
}

// TestCriarEmCaminhoInvalidoNaoMudaEstado protege a grade de pedido ruim.
func TestCriarEmCaminhoInvalidoNaoMudaEstado(t *testing.T) {
	m := motorDeTeste(t)

	inexistente := filepath.Join(t.TempDir(), "não-existe")
	if _, err := m.Criar(protocolo.Criar{Caminho: inexistente, Tipo: "bash"}); err == nil {
		t.Fatal("caminho inexistente devia falhar")
	} else if !strings.Contains(err.Error(), "não existe") {
		t.Fatalf("mensagem pouco clara: %v", err)
	}

	somenteLeitura := filepath.Join(t.TempDir(), "trancado")
	if err := os.Mkdir(somenteLeitura, 0o500); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	if os.Geteuid() != 0 { // root escreve em qualquer lugar
		if _, err := m.Criar(protocolo.Criar{Caminho: somenteLeitura, Tipo: "bash"}); err == nil {
			t.Fatal("diretório sem permissão de escrita devia falhar")
		} else if !strings.Contains(err.Error(), "permissão") {
			t.Fatalf("mensagem pouco clara: %v", err)
		}
	}

	if _, err := m.Criar(protocolo.Criar{Caminho: t.TempDir(), Tipo: "planilha"}); err == nil {
		t.Fatal("tipo inexistente devia falhar")
	}

	if len(m.Retrato().Projetos) != 0 {
		t.Fatalf("nenhum pedido inválido pode ter mexido na grade: %#v", m.Retrato())
	}
}

// TestReconstituirMantemAGrade é o que salva o trabalho depois de uma queda da
// WSL: o motor morre, sobe de novo e a grade volta igual.
func TestReconstituirMantemAGrade(t *testing.T) {
	dirEstado := t.TempDir()
	dirProjeto := t.TempDir()

	primeiro := Novo(dirEstado, configuracaoDeTeste())
	if err := primeiro.Iniciar(); err != nil {
		t.Fatalf("iniciar: %v", err)
	}
	if _, err := primeiro.Criar(protocolo.Criar{Caminho: dirProjeto, Tipo: "bash", Nome: "testes"}); err != nil {
		t.Fatalf("criar: %v", err)
	}
	if _, err := primeiro.Criar(protocolo.Criar{Caminho: dirProjeto, Tipo: "bash", Nome: "build"}); err != nil {
		t.Fatalf("criar: %v", err)
	}
	antes := primeiro.Retrato()
	primeiro.Encerrar()

	segundo := Novo(dirEstado, configuracaoDeTeste())
	if err := segundo.Iniciar(); err != nil {
		t.Fatalf("reiniciar: %v", err)
	}
	defer segundo.Encerrar()

	depois := segundo.Retrato()
	if len(depois.Projetos) != 1 || len(depois.Projetos[0].Celulas) != 2 {
		t.Fatalf("a grade não voltou: %#v", depois)
	}
	for i, celulaAntes := range antes.Projetos[0].Celulas {
		celulaDepois := depois.Projetos[0].Celulas[i]
		if celulaAntes.ID != celulaDepois.ID || celulaAntes.Tipo != celulaDepois.Tipo || celulaAntes.Nome != celulaDepois.Nome {
			t.Fatalf("célula %d voltou diferente:\nantes %#v\ndepois %#v", i, celulaAntes, celulaDepois)
		}
	}
}

// TestTeclaChegaNaCelula fecha a fatia vertical: tecla entra pelo motor e a
// saída aparece no retrato.
func TestTeclaChegaNaCelula(t *testing.T) {
	m := motorDeTeste(t)
	id, err := m.Criar(protocolo.Criar{Caminho: t.TempDir(), Tipo: "bash"})
	if err != nil {
		t.Fatalf("criar: %v", err)
	}
	colarEEntrar(t, m, id, "echo tesseract")
	if !esperarPor(t, 2*time.Second, func() bool {
		return strings.Contains(strings.Join(m.Retrato().Projetos[0].Celulas[0].Linhas, "\n"), "tesseract")
	}) {
		t.Fatalf("a saída não chegou ao retrato:\n%#v", m.Retrato().Projetos[0].Celulas[0].Linhas)
	}
}

// TestSocketAtendeUmaTela cobre o caminho inteiro pelo protocolo.
func TestSocketAtendeUmaTela(t *testing.T) {
	m := motorDeTeste(t)
	caminhoSocket := filepath.Join(t.TempDir(), "motor.sock")
	servidor, err := Servir(m, caminhoSocket)
	if err != nil {
		t.Fatalf("servir: %v", err)
	}
	defer servidor.Fechar()

	if _, err := Servir(m, caminhoSocket); err == nil {
		t.Fatal("dois motores no mesmo socket não podem coexistir")
	}

	conexao, err := net.Dial("unix", caminhoSocket)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	defer conexao.Close()

	escritor := json.NewEncoder(conexao)
	leitor := json.NewDecoder(conexao)
	pedir := func(tipo string, dados any) {
		t.Helper()
		envelope, err := protocolo.Empacotar(tipo, dados)
		if err != nil {
			t.Fatalf("empacotar: %v", err)
		}
		if err := escritor.Encode(envelope); err != nil {
			t.Fatalf("escrever: %v", err)
		}
	}

	pedir(protocolo.TipoCriar, protocolo.Criar{Caminho: t.TempDir(), Tipo: "bash"})

	// O primeiro retrato com uma célula chega sozinho, sem a tela pedir.
	var idCelula string
	conexao.SetReadDeadline(time.Now().Add(3 * time.Second))
	for idCelula == "" {
		var envelope protocolo.Mensagem
		if err := leitor.Decode(&envelope); err != nil {
			t.Fatalf("ler retrato: %v", err)
		}
		switch envelope.Tipo {
		case protocolo.TipoEstado:
			estado, err := protocolo.Desempacotar[protocolo.Estado](envelope)
			if err != nil {
				t.Fatalf("desempacotar estado: %v", err)
			}
			if len(estado.Projetos) == 1 && len(estado.Projetos[0].Celulas) == 1 {
				idCelula = estado.Projetos[0].Celulas[0].ID
			}
		case protocolo.TipoErro:
			erro, _ := protocolo.Desempacotar[protocolo.Erro](envelope)
			t.Fatalf("o motor recusou o pedido: %s", erro.Mensagem)
		}
	}

	pedir(protocolo.TipoTamanho, protocolo.Tamanho{Celula: idCelula, Colunas: 60, Linhas: 10})
	pedir(protocolo.TipoTecla, protocolo.Tecla{Celula: idCelula, Colar: "echo pelo-socket"})
	pedir(protocolo.TipoTecla, protocolo.Tecla{Celula: idCelula, Codigo: vt.KeyEnter})

	achou := false
	conexao.SetReadDeadline(time.Now().Add(3 * time.Second))
	for !achou {
		var envelope protocolo.Mensagem
		if err := leitor.Decode(&envelope); err != nil {
			t.Fatalf("ler retrato: %v", err)
		}
		if envelope.Tipo != protocolo.TipoEstado {
			continue
		}
		estado, err := protocolo.Desempacotar[protocolo.Estado](envelope)
		if err != nil {
			t.Fatalf("desempacotar estado: %v", err)
		}
		if len(estado.Projetos) == 0 {
			continue
		}
		achou = strings.Contains(strings.Join(estado.Projetos[0].Celulas[0].Linhas, "\n"), "pelo-socket")
	}

	pedir(protocolo.TipoStatus, struct{}{})
	conexao.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var envelope protocolo.Mensagem
		if err := leitor.Decode(&envelope); err != nil {
			t.Fatalf("ler resumo: %v", err)
		}
		if envelope.Tipo != protocolo.TipoResumo {
			continue
		}
		resumo, err := protocolo.Desempacotar[protocolo.Resumo](envelope)
		if err != nil {
			t.Fatalf("desempacotar resumo: %v", err)
		}
		if !strings.Contains(resumo.Texto, "1 célula") {
			t.Fatalf("resumo não conta a célula: %q", resumo.Texto)
		}
		break
	}
}
