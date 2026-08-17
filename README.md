```
     ┌────┐
     │┌───┼┐    T E S S E R A C T
     ││ ▓ ││    o mosaico não desmonta
     └┼───┘│
      └────┘    ts 0.1.0 // MIT
```

[![licença](https://img.shields.io/badge/licen%C3%A7a-MIT-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6)](LICENSE)
[![versão](https://img.shields.io/badge/vers%C3%A3o-0.1.0-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6)](https://github.com/AndreLuizMMS/tesseract/releases)
[![plataforma](https://img.shields.io/badge/plataforma-Linux%20%7C%20WSL%20%7C%20macOS-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6)](#instalar)

O símbolo é um tesserato achatado: dois quadrados, um atrás do outro, deslocados, e uma
tessera acesa no meio — a célula que está com o seu teclado. Em um caractere só, ele é `⧉`.
O banner é feito só de traço e sombreado: **nada nele depende de cor**, então ele lê igual
no tema claro e no escuro do GitHub, no `cat`, no `less` e com `NO_COLOR=1`.

**Um mosaico de agentes em terminal.** Vários Claude Code, Cursor CLI, shells, logs de
Docker e arquivos markdown vivendo lado a lado, num painel só, com um motor que continua
de pé quando você fecha a tela — e reconstrói tudo depois de um `wsl --shutdown`.

```
 TESSERACT   ⬤ 1   ⏵ 1                                  NAVEGAR
━━ DOXAR-API  /home/dev/doxar-api ─────────────────────────────────────────────────────────────────────────── ⬤1  ● 4/5
┌  claude  cursor  bash  refatora auth ─────── ⬤ RESPONDEU ┐┌  claude  cursor  bash  testes ──────────── ▸ TRABALHANDO ┐
│Movi a validação de token                                 ││$ go test ./...                                           │
│pro guard.                                                ││ok                                                        │
│Qual você prefere?                                        ││                                                          │
│                                                          ││                                                          │
└──────────────────────────────────────────────────────────┘└──────────────────────────────────────────────────────────┘
── CORTZ-WEB  /home/dev/cortz-web ────────────────────────────────────────────────────────────────────────────────── ⏵1
┌  claude  cursor  bash  fix nav ─────────────────────────────────────────────────────────────────────────── ⏵ APROVAR ┐
│posso mexer no Header?                                                                                                │
│                                                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
── API-LEGADO  /home/dev/api-legado ────────────────────────────────────────────────────────────────────────────────────
┌ md · spec-m7.md ─────────────────────────────────────────────────────────────────────────────────────────── ○ PARADA ┐
│# Módulo 7                                                                                                            │
│                                                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
 ↑↓ célula   ←→ projeto   tab aba   ↵ digitar   v lista   n criar   d docker   ? ajuda
```

## O problema

Quem trabalha com vários agentes ao mesmo tempo passa o dia trocando de aba para descobrir
**quem terminou** e **quem está travado esperando um sim**. E quando a WSL cai — porque o
Windows reiniciou, porque a máquina suspendeu, porque alguém rodou `wsl --shutdown` — some
tudo junto: os agentes, as conversas, o histórico.

O Tesseract resolve os dois:

- **Uma tela só.** Todas as células abertas ao mesmo tempo, de todos os projetos, cada uma
  com o conteúdo vivo. O projeto é a divisão: uma faixa com o nome, o caminho, quantas
  células pedem atenção e o estado da stack. As células de um projeto dividem a largura
  entre si, e a tela se rearranja sozinha conforme quantas existem.
- **Um motor que não morre com a tela.** Ele é um serviço da sua conta. Fecha a tela, o
  trabalho continua. A WSL cai, ele volta com o serviço e remonta a grade nas mesmas
  posições — com as conversas reatadas e **nenhum agente trabalhando sozinho**.

## Instalar

```bash
curl -fsSL https://raw.githubusercontent.com/AndreLuizMMS/tesseract/main/install.sh | bash
```

Uma linha, e acabou. O instalador baixa o código, baixa o Go se a máquina não tiver um que
sirva, compila o comando `ts` em `~/.local/bin`, põe o diretório no PATH, instala o serviço
de usuário e sobe o motor. **Atualizar é rodar a mesma linha** — ela reinicia o motor no
código novo, que é o passo que ninguém lembra de fazer na mão.

Depois é só:

```bash
cd ~/meu-projeto
ts
```

Só o que o Tesseract não instala por você: WSL com systemd ligado e os agentes que você
usa (`claude`, `cursor-agent`).

## Como funciona

Existem exatamente **três coisas**.

### Projeto

Um diretório. Nasce junto com sua primeira célula — criar projeto e criar célula são o
mesmo gesto — e **sai da tela quando a última célula dele morre**. O disco nunca é tocado:
o projeto sair da tela não apaga, não move e não altera nada.

### Célula

A unidade de trabalho. **Uma regra só para todas**: a mesma tecla cria, mata, nomeia, foca
e navega em qualquer uma.

Criar não pergunta o que a célula vai ser. Uma **sessão** nasce com quatro abas por dentro,
todas no diretório do projeto, e `tab` troca entre elas. Só a aba que você está usando tem
processo: as outras sobem quando você chega nelas.

| Aba | O que é |
|---|---|
| `claude` | Claude Code |
| `cursor` | Cursor CLI |
| `bash` | um shell |
| `md` | os markdowns do projeto: lista com busca por nome, e o escolhido renderizado |

A aba `md` abre numa lista de todo markdown do projeto, com uma **barra de busca no topo**.
Entre em DIGITAR (`↵`), digite parte do nome para filtrar, `↑↓` escolhe e `↵` abre o
arquivo. `esc` volta para a lista. O arquivo aberto recarrega sozinho quando o disco muda —
dá para ver o agente escrevendo a spec ao lado.

O documento é desenhado **como página, não como saída de terminal**: medida de leitura
fixa e centralizada, margem dos dois lados, título em faixa, seções com filete, citação
com barra, tabela com régua fina e código em caixa própria. Linha de código mais larga que
a página é **cortada com `›`**, nunca quebrada — diagrama partido no meio não se lê.

Existem ainda duas células que não são sessão:

| Tipo | O que é |
|---|---|
| `logs` | log ao vivo de um serviço do compose, criado pelo painel Docker |
| `md` | um markdown específico, quando você preenche o campo MD na criação |

Cada célula tem um estado, e é ele que aparece no marcador. Todo estado tem **três** sinais
ao mesmo tempo — um glifo, uma cor e uma forma — para que nenhum deles seja indispensável:

| Sinal | Estado | O que fazer |
|---|---|---|
| `▸ TRABALHANDO` | processo vivo produzindo | nada — deixe trabalhar |
| `⬤ RESPONDEU` | devolveu a vez, tem resposta esperando leitura | leia quando puder; não trava nada |
| `⏵ APROVAR` | travou numa pergunta e **não anda** sem resposta | responda: o trabalho parou nisso |
| `✖ CAIU` | o processo morreu sozinho | `r` sobe de novo |
| `○ PARADA` | sem processo, célula preservada | `r` retoma de onde parou |
| `⚠ ÓRFÃ` | o diretório do projeto sumiu do disco | recrie o caminho ou mate a célula |

`⏵ APROVAR` é o único que vira **barra sólida invertida ocupando a linha inteira** do
cabeçalho da célula, e o único que **pisca** — 1,8s aceso, 200ms apagado, para sempre,
enquanto alguém estiver esperando você. Os outros cinco são um glifo e um rótulo, parados.
É de propósito: urgência aqui é área preenchida, não matiz — funciona de longe, funciona no
canto do olho e funciona sem cor nenhuma.

No quadro apagado a barra **continua barra**: só o fundo muda. A área preenchida é o que
diz "isto trava o trabalho", e ela nunca some. E o relógio só existe enquanto há célula
travada — sem nenhuma, a tela fica completamente parada. `TESSERACT_SEM_PISCA=1` desliga a
piscada para quem prefere a tela imóvel.

**Respondeu ≠ aprovar.** É a distinção que faz o alarme valer alguma coisa: agente parado
numa pergunta bloqueia o trabalho; agente que terminou o turno apenas tem algo para ler.

E **nenhum alarme falso**: spinner piscando e cursor se mexendo não contam como atividade.
O motor exige leituras seguidas de trabalho antes de armar o aviso, e leituras seguidas de
silêncio antes de declarar o turno encerrado.

### Painel Docker

Pertence ao **projeto**, não à célula. O arquivo de compose é procurado na raiz e nas
pastas de primeiro nível — porque projeto de verdade guarda a stack em `docker/`, `infra/`
e afins — e **arquivo de produção nunca é escolhido**. O painel lista os serviços com
estado, porta, saúde e tempo de pé; sobe, para, reinicia e rebuilda serviço ou stack
inteira; e transforma o log de um serviço numa célula do mosaico.

Enquanto o Docker trabalha, o painel diz o que está fazendo e vai atualizando a lista —
os serviços ficam verdes um a um em vez de tudo mudar de uma vez no fim.

**Nenhuma ação destrutiva existe aqui.** Não há `down -v`, não há apagar volume.

## Os dois modos

Por padrão você está em **NAVEGAR**: toda tecla é do aplicativo.

`↵` entra em **DIGITAR**: toda tecla é da célula, **sem nenhuma exceção**. Nem `q`, nem `D`,
nem `tab`, nem as setas. `ctrl-l` devolve o teclado.

Colar (`ctrl-v`, ou o que o seu terminal usar) funciona em DIGITAR e nos campos de texto.
O texto vai **marcado como colagem**: um prompt de várias linhas entra inteiro na caixa do
agente, em vez de cada quebra de linha virar um envio. Em campo de uma linha só — caminho
do projeto, prompt do `p` — a colagem é achatada numa linha.

Nunca há dois donos do teclado ao mesmo tempo — então colisão de atalho é estruturalmente
impossível. E o modo é impossível de errar, porque ele muda **quatro** coisas de uma vez:

1. o fundo da tela escurece e o resto apaga;
2. a borda da célula focada engrossa e vira dupla;
3. o selo `▓ DIGITAR ▓` aparece invertido;
4. a célula que tem o teclado fica **verde phosphor** — e é o único verde phosphor da tela.

Com `NO_COLOR=1` os sinais 1 e 4 somem, e os sinais 2 e 3 continuam: borda dupla e selo
invertido não dependem de cor nenhuma.

## Copiar o que o agente escreveu

Com o mosaico, a seleção do terminal não serve: ela pega os vizinhos e as bordas junto.
Então a marca é do próprio Tesseract. **Arraste o mouse por cima da célula** — o trecho
acende — e **solte**: o texto vai para a área de transferência, sem cor e sem os espaços do
fim das linhas. Vale nos dois modos e nos dois sentidos do arrasto. `esc` apaga a marca.

Clicar sem arrastar só escolhe a célula; não encosta no que você tinha copiado antes. Para
pegar o que já saiu da tela, **role primeiro** (roda do mouse) e depois arraste — a marca
vale sobre o que está à vista.

## Teclado

**Andar — só setas, nenhuma letra**

| Tecla | Ação |
|---|---|
| `↑` `↓` | célula anterior / próxima, atravessando projeto |
| `←` `→` | projeto anterior / próximo |
| `espaço` | pula para a próxima célula que pede atenção, atravessando projeto |
| `1`…`9` | vai direto para o projeto N |
| `tab` | troca a aba da célula: claude, cursor, shell, md (`shift-tab` volta) |

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
| `n` | criar — um formulário só, que começa na sua casa e não pergunta o tipo |
| `r` | retoma célula parada, ou sobe célula caída |
| `D` | mata a célula focada — sempre confirma |
| `R` | renomeia a célula **e propaga o nome para dentro do agente** |
| `ctrl-r` | adota na célula o nome que o agente deu à conversa |

**Agir e ler**

| Tecla | Ação |
|---|---|
| `p` | manda prompt para a célula focada sem entrar nela |
| `d` | abre o painel Docker do projeto focado |
| `ctrl-e` | abre o diretório do projeto na IDE configurada (`cursor /caminho`) |
| roda do mouse | rola o histórico da célula |
| arrastar com o mouse | marca um trecho da célula e **copia ao soltar** |
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
| `sessao` | volta na mesma aba, com a conversa de cada agente reatada e **parada**. Nenhum prompt é disparado |
| aba `bash` | shell novo e limpo; o histórico anterior fica rolável acima da linha de queda |
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

## Tema

A cor aqui não é enfeite, é gramática. Ela tem três leis, e nenhuma delas é estética:

- **Verde é posse do teclado.** Nunca é estado. O verde phosphor aparece no máximo uma vez
  por tela: na célula que está com o seu teclado, e em mais nada.
- **Ciano é estrutura.** Grade, cantos, numeração, rótulos. Nunca é estado.
- **Estado não usa verde nem ciano**, e urgência é área preenchida, não matiz.

Fora isso: sem brilho, sem scanline, sem ligadura, sem emoji, sem canto arredondado dentro
do terminal. Esses efeitos existem só na superfície de marca — este README, o site, o
banner.

A paleta inteira mora em **um arquivo só**, `internal/tema/tema.go`. Nenhum outro arquivo do
projeto escreve hex: quem desenha pede o token pelo nome (`tema.BrandPhosphor`,
`tema.FluxCore`, `tema.StateBlock`). O guarda dessa regra é executável:

```bash
./scripts/check-theme.sh
```

Ele imprime a paleta inteira em blocos ANSI para conferência a olho, e **falha** se alguém
usar verde ou ciano como cor de estado, ou escrever hex fora do arquivo de tema.

O tema tem três perfis e escolhe sozinho: cor cheia, 16 cores, ou nenhuma (`NO_COLOR=1` ou
`TERM=dumb`). Nos três o alfabeto de estados continua legível, porque o glifo e a forma
carregam o significado e a cor só reforça.

| Variável | O que faz |
|---|---|
| `NO_COLOR` | tira a cor inteira; sobra negrito, vídeo invertido e borda |
| `TESSERACT_SEM_PISCA` | para a piscada da barra de `aprovar`, mantendo a barra |
| `TESSERACT_SEM_ABERTURA` | pula a animação de partida e vai direto para a grade |

### Onde a marca aparece

O símbolo não vive só no README. Dentro do produto ele tem quatro casas:

| Lugar | Forma |
|---|---|
| Abertura | o símbolo se desenhando, traço a traço, na partida |
| Barra de título | glifo `⧉`, sempre à esquerda do nome |
| Grade vazia | símbolo 7×5 no centro, com a tecla que cria a primeira célula |
| Título da janela | `⧉ ts — projeto/célula`, acompanhando o foco |

### A abertura

A marca não aparece pronta na partida: **ela se monta**. O quadrado de trás nasce primeiro,
traço a traço, com a ponta da caneta acesa em fósforo correndo pelo caminho. Depois o
quadrado da frente por cima. A tessera acende, o símbolo inteiro estoura num quadro só e
assenta. Só então o nome abre, letra a letra, e as linhas do motor entram uma a uma.

```
   ┌────┐
   │┌───┼┐    T E S S E R A C T
   ││ ▓ ││    o mosaico não desmonta
   └┼───┘│
    └────┘    ts 0.1.0 // MIT

   > motor de sessão: vivo
   > 8 células recuperadas · 3 projetos · mesma posição
   > grade montada em 41ms
```

Dura cerca de um segundo e meio, e **roda em paralelo com a conexão** — enquanto a marca se
desenha, o motor é procurado e a grade é remontada na outra linha de execução. O cronômetro
de `grade montada em` conta só o motor, nunca a animação: é um número que existe para ser
verdade.

Nada disso é brilho, scanline ou glitch — o terminal não emite luz e a regra continua
valendo. O que se move é a **ordem em que as coisas passam a existir**, não a textura delas.
Sem terminal de verdade (saída num arquivo, num pipe), a abertura vira o bloco estático de
uma vez só.

### O mesmo tema no resto da mesa

A pasta `themes/` traz o **Tesseract Neon** pronto para o terminal e as ferramentas do dia:

| Arquivo | Para |
|---|---|
| `windows-terminal.json`, `wezterm.toml`, `alacritty.toml`, `kitty.conf`, `ghostty` | emuladores de terminal |
| `tesseract-neon.yaml` | esquema base16/base24 (tinted-theming) |
| `tmux.conf`, `starship.toml`, `fzf.env` | barra de status, prompt, busca |
| `bat.tmTheme`, `delta.gitconfig` | leitura de arquivo e diff |
| `nvim/tesseract.lua`, `eza-ls-colors.sh` | editor e listagem |
| `claude-code.md` | o Claude Code dentro da célula, sem cor própria |

**O agente dentro da célula.** O Claude Code pinta com paleta própria e briga com a grade —
laranja no selo, rosa no logo. A correção é uma linha: `/config` → tema → **`dark-ansi`**.
Nesse tema ele desenha só com as 16 cores ANSI, que são as do Tesseract Neon. Detalhes em
[`themes/claude-code.md`](themes/claude-code.md).

A marca em vetor está em `themes/logo.svg` (colorida) e `themes/logo-mono.svg` (traço único
em `currentColor`, para favicon e 16px). Os detalhes de cada arquivo estão em
[`themes/README.md`](themes/README.md).

## Configuração

Opcional, em `~/.config/tesseract/config.json`. Sem o arquivo, tudo funciona com o padrão.

```json
{
  "editor": "cursor",
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
   são tocados. A sessão com abas é só mais um tipo, feito dos outros.
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
