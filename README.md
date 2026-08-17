# Tesseract

**Um mosaico de agentes em terminal.** Vários Claude Code, Cursor CLI, shells, logs de
Docker e arquivos markdown vivendo lado a lado, num painel só, com um motor que continua
de pé quando você fecha a tela — e reconstrói tudo depois de um `wsl --shutdown`.

```
 TESSERACT   ⬤ 1   ⏵ 1                                  NAVEGAR              ⏳ 26% 2:30
┌───────────────────────────────────────────────── DOXAR-API ──────────────────────────────────────────────────┬───┬───┐
│┌ claude · refatora auth ─────────────────────────────────────────────────────────────────────── ⬤ RESPONDEU ┐│ C │ A │
││Movi a validação de token                                                                                   ││ O │ P │
││pro guard.                                                                                                  ││ R │ I │
││Qual você prefere?                                                                                          ││ T │ - │
││                                                                                                            ││ Z │ L │
│└────────────────────────────────────────────────────────────────────────────────────────────────────────────┘│ - │ E │
│┌ bash · testes ────────────────────────────────────────────────────────────────────────────── ▸ TRABALHANDO ┐│ W │ G │
││$ go test ./...                                                                                             ││ E │ A │
││ok                                                                                                          ││ B │ D │
│└────────────────────────────────────────────────────────────────────────────────────────────────────────────┘│   │ O │
│┌ logs · worker ────────────────────────────────────────────────────────────────────────────── ▸ TRABALHANDO ┐│ 1 │   │
││worker-1  | processando fila…                                                                               ││⏵1 │ 1 │
│└────────────────────────────────────────────────────────────────────────────────────────────────────────────┘│   │   │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────────┴───┴───┘
 ↑↓ célula   ←→ projeto   ↵ digitar   v lista   n criar   d docker   ? ajuda
```

## O problema

Quem trabalha com vários agentes ao mesmo tempo passa o dia trocando de aba para descobrir
**quem terminou** e **quem está travado esperando um sim**. E quando a WSL cai — porque o
Windows reiniciou, porque a máquina suspendeu, porque alguém rodou `wsl --shutdown` — some
tudo junto: os agentes, as conversas, o histórico.

O Tesseract resolve os dois:

- **Uma tela só.** Todos os projetos ao mesmo tempo, cada um numa coluna. A coluna do
  projeto em foco fica larga e mostra o conteúdo vivo das células; as outras encolhem numa
  tira que nunca some — nome na vertical, quantas células, quantas pedem atenção, estado
  do Docker.
- **Um motor que não morre com a tela.** Ele é um serviço da sua conta. Fecha a tela, o
  trabalho continua. A WSL cai, ele volta com o serviço e remonta a grade nas mesmas
  posições — com as conversas reatadas e **nenhum agente trabalhando sozinho**.

## Instalar

Precisa de Go 1.25+, WSL com systemd, e os agentes que você usa (`claude`, `cursor-agent`).

```bash
git clone git@github.com:AndreLuizMMS/tesseract.git
cd tesseract
./instalar.sh
```

O script compila o comando `ts` em `~/.local/bin`, instala o serviço de usuário e liga ele.
Depois é só:

```bash
cd ~/meu-projeto
ts
```

## Como funciona

Existem exatamente **três coisas**.

### Projeto

Um diretório. Nasce junto com sua primeira célula — criar projeto e criar célula são o
mesmo gesto — e **sai da tela quando a última célula dele morre**. O disco nunca é tocado:
o projeto sair da tela não apaga, não move e não altera nada.

### Célula

A unidade de trabalho. Cinco tipos, **uma regra só para todos**: a mesma tecla cria, mata,
nomeia, foca e navega em qualquer um.

| Tipo | O que é |
|---|---|
| `claude` | Claude Code no diretório do projeto |
| `cursor` | Cursor CLI no diretório do projeto |
| `bash` | shell no diretório do projeto |
| `logs` | log ao vivo de um serviço do compose |
| `md` | arquivo markdown renderizado, recarrega quando o disco muda |

Cada célula tem um estado, e é ele que aparece no marcador:

| Marcador | Significado |
|---|---|
| `▸ trabalhando` | processo vivo produzindo |
| `⬤ respondeu` | devolveu a vez, tem resposta esperando leitura |
| `⏵ aprovar` | travou numa pergunta e **não anda** sem resposta |
| `✖ caiu` | o processo morreu sozinho |
| `○ parada` | sem processo, célula preservada |
| `⚠ órfã` | o diretório do projeto sumiu do disco |

**Respondeu ≠ aprovar.** É a distinção que faz o alarme valer alguma coisa: agente parado
numa pergunta bloqueia o trabalho; agente que terminou o turno apenas tem algo para ler.

E **nenhum alarme falso**: spinner piscando e cursor se mexendo não contam como atividade.
O motor exige leituras seguidas de trabalho antes de armar o aviso, e leituras seguidas de
silêncio antes de declarar o turno encerrado.

### Painel Docker

Pertence ao **projeto**, não à célula. Existe quando há arquivo de compose na raiz. Lista os
serviços com estado, porta, saúde e tempo de pé; sobe, para, reinicia e rebuilda serviço ou
stack inteira; e transforma o log de um serviço numa célula do mosaico.

**Nenhuma ação destrutiva existe aqui.** Não há `down -v`, não há apagar volume.

## Os dois modos

Por padrão você está em **NAVEGAR**: toda tecla é do aplicativo.

`↵` entra em **DIGITAR**: toda tecla é da célula, **sem nenhuma exceção**. Nem `q`, nem `D`,
nem `tab`, nem as setas. `ctrl-l` devolve o teclado.

Nunca há dois donos do teclado ao mesmo tempo — então colisão de atalho é estruturalmente
impossível. E o modo é impossível de errar: em DIGITAR a barra e as tiras apagam, aparece o
selo `▓ DIGITAR ▓` e a borda da célula engrossa.

## Teclado

**Andar — só setas, nenhuma letra**

| Tecla | Ação |
|---|---|
| `↑` `↓` | célula anterior / próxima na coluna |
| `←` `→` | projeto anterior / próximo (a coluna engorda) |
| `espaço` | pula para a próxima célula que pede atenção, atravessando projeto |
| `1`…`9` | vai direto para o projeto N |

**Teclado e tela**

| Tecla | Ação |
|---|---|
| `↵` | entra em DIGITAR na célula focada |
| `ctrl-l` | devolve o teclado ao aplicativo |
| `o` | célula focada em tela cheia (liga e desliga) |
| `v` | alterna mosaico ↔ lista |

**Criar, matar, nomear**

| Tecla | Ação |
|---|---|
| `n` | criar — pede o projeto, depois a célula, num formulário só |
| `r` | retoma célula parada, ou sobe célula caída |
| `D` | mata a célula focada — sempre confirma |
| `R` | renomeia a célula **e propaga o nome para dentro do agente** |
| `ctrl-r` | adota na célula o nome que o agente deu à conversa |

**Agir e ler**

| Tecla | Ação |
|---|---|
| `p` | manda prompt para a célula focada sem entrar nela |
| `d` | abre o painel Docker do projeto focado |
| `ctrl-e` | abre o diretório do projeto no editor configurado |
| roda do mouse | rola o histórico da célula |
| `/` | busca no histórico da célula focada |
| `esc` | sai da rolagem / fecha o que estiver aberto |
| `?` | ajuda |
| `q` | fecha a tela — o motor continua rodando |

## Comandos

```
ts                 abre a tela, subindo o motor se preciso
ts novo <dir>      adiciona um projeto sem abrir a tela
ts status          estado do motor e resumo de projetos e células
ts stop            desliga o motor e todas as células
ts reset           apaga o estado salvo e derruba tudo, preservando a configuração
```

## Recuperação

`wsl --shutdown` mata todos os processos. Quando a WSL volta, o serviço sobe sozinho e
reconstrói a grade:

| Tipo | O que acontece |
|---|---|
| `claude` `cursor` | reata a conversa de onde parou e acorda **parado**. Nenhum prompt é disparado |
| `bash` | shell novo e limpo; o histórico anterior fica rolável acima da linha de queda |
| `logs` | volta a acompanhar o serviço; se a stack estiver parada, engata sozinha quando ele subir |
| `md` | relê o arquivo |
| Docker | **não sobe sozinho**. Subir stack é decisão sua |

O risco que a recuperação automática cria — agente trabalhando sem ninguém na frente da
tela — é cortado na raiz: **reatar a conversa nunca dispara trabalho**. Existe um teste que
falha se qualquer byte for escrito no teclado do agente durante a reconstituição.

## Aviso

Quem notifica é o motor, não a tela. Som e notificação do sistema com o nome da célula e do
projeto, mesmo com a tela fechada. Na WSL, o toast sai pelo PowerShell do Windows; se você
tiver `wsl-notify-send.exe` ou `notify-send`, eles são usados no lugar. Os dois avisos são
desligáveis, separadamente.

## Configuração

Opcional, em `~/.config/tesseract/config.json`. Sem o arquivo, tudo funciona com o padrão.

```json
{
  "editor": "code",
  "som": true,
  "notificar": true,
  "comandoNotificacao": "",
  "tetoHistorico": 5242880,
  "agentes": {
    "claude": {
      "programa": "claude",
      "args": ["--model", "opus"],
      "comandoRenomear": "/rename"
    }
  }
}
```

O **badge de consumo** da janela de 5 horas aparece na barra de título quando o seu
statusline do Claude Code escreve `~/.claude/tesseract-quota.json` (ou o antigo
`squad-quota.json`) no formato `{"used_percentage": 59, "resets_at": 1786955400}`. Sem o
arquivo, tudo funciona igual — só não aparece o badge.

## Como isso não vira um Frankenstein

Três regras duras, e cada uma tem um teste que quebra se ela for violada:

1. **Tipo de célula é uma peça fechada.** Um tipo declara como nasce, como se desenha, o que
   faz com uma tecla e quais estados tem. Adicionar um tipo é escrever um arquivo em
   `internal/celula/` e uma linha no registro — o mosaico, a lista, os atalhos e o motor não
   são tocados.
2. **Feature nova não ganha tecla nova.** O mapa de teclas mora num arquivo só. Um teste
   percorre o mapa inteiro e falha se a mesma tecla tiver dois significados no mesmo modo,
   se qualquer tecla ficar sem texto de ajuda, ou se alguma letra andar pela grade.
3. **A tela não decide nada.** Ela desenha o que o motor manda e devolve tecla. Um teste
   alimenta a lista e o mosaico com o mesmo estado e exige que os dois mostrem os mesmos
   projetos, células e marcadores.

## Arquitetura

```
cmd/tess/              o comando ts: sobe o motor ou conecta nele
internal/
  motor/               estado, projetos, ciclo de vida, persistência, avisos
    historico/         gravação, rotação e busca por célula
  celula/              um arquivo por tipo, sobre um contrato só
  docker/              lê o compose e age na stack, nunca destrutivamente
  protocolo/           contrato motor ↔ tela, JSON por linha em socket unix
  teclado/             o mapa de teclas, num arquivo só
  tela/                mosaico, lista, painel, formulário
systemd/               a unidade de usuário que mantém o motor de pé
```

O motor mantém **a tela interna de cada célula em memória** — ele já sabe o que está escrito
em cada uma, sem perguntar a ninguém a cada quadro. É isso que mantém o mosaico fluido com
quinze células vivas.

## Stack

Go 1.25 e seis dependências, nenhuma a mais:

| Uso | Módulo |
|---|---|
| PTY | `github.com/creack/pty` |
| Emulação de terminal | `github.com/charmbracelet/x/vt` |
| Interface de terminal | `charm.land/bubbletea/v2` |
| Estilo | `charm.land/lipgloss/v2` |
| Markdown renderizado | `charm.land/glamour/v2` |
| Arquivo mudou | `github.com/fsnotify/fsnotify` |

Docker sai por `docker compose`, notificação por `powershell.exe`, comunicação por socket
unix com JSON por linha, estado em arquivo, testes com a biblioteca padrão. Sem SDK, sem
gRPC, sem banco, sem framework de teste.

## Desenvolver

```bash
make gate      # build + vet + testes, que é o portão de tudo
make build     # compila ./ts aqui mesmo
make instalar  # instala o comando e o serviço
```

Os testes sobem shells de verdade, stacks de Docker de verdade e a tela dentro de um
pseudo terminal de verdade — os que dependem de Docker se pulam sozinhos quando ele não
está disponível.

## Licença

MIT.
