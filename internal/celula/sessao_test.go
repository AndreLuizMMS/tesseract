package celula

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/motor/historico"
)

// sessaoDeTeste sobe uma sessão com agentes de mentira e um histórico por aba.
func sessaoDeTeste(t *testing.T) (*Sessao, string) {
	t.Helper()
	dir := t.TempDir()
	fingido := agenteDeMentira(t, dir)

	sessao := &Sessao{}
	cfg := Config{
		ID: "c1", Diretorio: dir, Nome: "trabalho", Colunas: 60, Linhas: 12,
		Perfis: map[string]Perfil{
			"claude": {Programa: fingido},
			"cursor": {Programa: fingido},
		},
		AbrirHistorico: func(sufixo string) (*historico.Historico, error) {
			return historico.Abrir(filepath.Join(dir, "hist-"+sufixo+".log"), historico.TetoPadrao)
		},
	}
	if err := sessao.Nascer(cfg); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	t.Cleanup(func() { sessao.Matar() })
	return sessao, dir
}

// TestSessaoNasceComAbasSemPerguntarNada — criar uma sessão não obriga a
// escolher entre claude, cursor e shell: ela já vem com os três.
func TestSessaoNasceComAbasSemPerguntarNada(t *testing.T) {
	sessao, _ := sessaoDeTeste(t)

	abas := sessao.Abas()
	if len(abas) < 3 {
		t.Fatalf("a sessão devia nascer com as abas dos agentes, veio %v", abas)
	}
	if sessao.AbaAtiva() != abas[0] {
		t.Fatalf("a primeira aba devia estar ativa, está %q", sessao.AbaAtiva())
	}
	if sessao.Estado() != Trabalhando {
		t.Fatalf("a aba ativa devia estar de pé, está %q", sessao.Estado())
	}
}

// TestSessaoSoSobeAAbaQueEUsada — três agentes por sessão custariam caro; as
// abas nascem quando alguém troca para elas.
func TestSessaoSoSobeAAbaQueEUsada(t *testing.T) {
	sessao, dir := sessaoDeTeste(t)

	if len(sessao.dentro) != 1 {
		t.Fatalf("só a aba ativa devia ter subido, subiram %d", len(sessao.dentro))
	}
	if _, err := os.Stat(filepath.Join(dir, "hist-cursor.log")); err == nil {
		t.Fatal("a aba que ninguém abriu não devia ter histórico ainda")
	}

	if err := sessao.TrocarAba(1); err != nil {
		t.Fatalf("trocar de aba: %v", err)
	}
	if sessao.AbaAtiva() != "cursor" {
		t.Fatalf("a aba ativa devia ser cursor, está %q", sessao.AbaAtiva())
	}
	if len(sessao.dentro) != 2 {
		t.Fatalf("a aba nova devia ter subido, temos %d", len(sessao.dentro))
	}
	if _, err := os.Stat(filepath.Join(dir, "hist-cursor.log")); err != nil {
		t.Fatalf("cada aba tem o seu histórico: %v", err)
	}
}

// TestSessaoDaAVoltaNasAbas — a tecla anda em círculo, nos dois sentidos.
func TestSessaoDaAVoltaNasAbas(t *testing.T) {
	sessao, _ := sessaoDeTeste(t)
	abas := sessao.Abas()

	for i := 1; i <= len(abas); i++ {
		if err := sessao.TrocarAba(1); err != nil {
			t.Fatalf("trocar de aba: %v", err)
		}
		esperada := abas[i%len(abas)]
		if sessao.AbaAtiva() != esperada {
			t.Fatalf("passo %d: esperava %q, está %q", i, esperada, sessao.AbaAtiva())
		}
	}
	if err := sessao.TrocarAba(-1); err != nil {
		t.Fatalf("voltar de aba: %v", err)
	}
	if sessao.AbaAtiva() != abas[len(abas)-1] {
		t.Fatalf("voltar devia levar à última aba, está %q", sessao.AbaAtiva())
	}
}

// TestSessaoMostraAAbaAtiva — o que a tela desenha é o conteúdo da aba de agora.
func TestSessaoMostraAAbaAtiva(t *testing.T) {
	sessao, _ := sessaoDeTeste(t)

	// Vai até a aba do shell, que é a que dá para escrever num teste.
	for sessao.AbaAtiva() != "bash" {
		if err := sessao.TrocarAba(1); err != nil {
			t.Fatalf("trocar de aba: %v", err)
		}
	}
	if err := sessao.Tecla(Toque{Colar: "echo dentro-da-aba\n"}); err != nil {
		t.Fatalf("tecla: %v", err)
	}
	if !esperarPor(t, 3*time.Second, func() bool {
		return strings.Contains(strings.Join(sessao.Desenhar().Linhas, "\n"), "dentro-da-aba")
	}) {
		t.Fatalf("a saída da aba ativa não apareceu:\n%s", strings.Join(sessao.Desenhar().Linhas, "\n"))
	}

	// A busca olha o histórico da aba que está aparecendo.
	registro := sessao.HistoricoAtivo()
	if registro == nil {
		t.Fatal("a aba ativa devia ter histórico")
	}
	if !esperarPor(t, 2*time.Second, func() bool {
		achados, _ := registro.Buscar("dentro-da-aba")
		return len(achados) > 0
	}) {
		t.Fatal("o histórico da aba ativa não guardou o que ela escreveu")
	}
}

// TestSessaoGuardaAConversaDeCadaAba — depois de uma queda, cada aba reata a
// sua.
func TestSessaoGuardaAConversaDeCadaAba(t *testing.T) {
	sessao, _ := sessaoDeTeste(t)
	if !esperarPor(t, 3*time.Second, func() bool { return sessao.Conversas()["claude"] != "" }) {
		t.Fatalf("a conversa da aba do claude devia ter identidade: %#v", sessao.Conversas())
	}
	if sessao.Conversas()["bash"] != "" {
		t.Fatal("shell não tem conversa")
	}
}

// TestSessaoMorreInteira — matar a sessão leva todas as abas junto.
func TestSessaoMorreInteira(t *testing.T) {
	sessao, _ := sessaoDeTeste(t)
	if err := sessao.TrocarAba(1); err != nil {
		t.Fatalf("trocar de aba: %v", err)
	}
	abertas := len(sessao.dentro)
	if abertas < 2 {
		t.Fatalf("deviam existir duas abas abertas, existem %d", abertas)
	}
	if err := sessao.Matar(); err != nil {
		t.Fatalf("matar: %v", err)
	}
	for aba, celula := range sessao.dentro {
		if !esperarPor(t, 3*time.Second, func() bool { return celula.Estado() == Parada }) {
			t.Fatalf("a aba %s continuou de pé: %q", aba, celula.Estado())
		}
	}
}

// TestSessaoNasceNaAbaSalva — reconstituir volta para a aba que estava
// aparecendo.
func TestSessaoNasceNaAbaSalva(t *testing.T) {
	dir := t.TempDir()
	fingido := agenteDeMentira(t, dir)
	sessao := &Sessao{}
	if err := sessao.Nascer(Config{
		ID: "c1", Diretorio: dir, Colunas: 60, Linhas: 12, Aba: "bash",
		Perfis: map[string]Perfil{"claude": {Programa: fingido}, "cursor": {Programa: fingido}},
	}); err != nil {
		t.Fatalf("nascer: %v", err)
	}
	defer sessao.Matar()

	if sessao.AbaAtiva() != "bash" {
		t.Fatalf("devia ter nascido na aba salva, está em %q", sessao.AbaAtiva())
	}
}
