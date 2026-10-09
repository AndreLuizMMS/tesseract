# Levantamento: Tesseract → mods do Claude Code

> **Como ficou (divisão final).** TUI Go = painéis; mods = sinais, guarda e tools.
> Docker, leitor de markdown e seletor de IDE existem uma vez só, no mosaico (`d`, aba
> `md`, `ctrl+e`). Os mods `tesseract-docker`, `tesseract-md` e `tesseract-ide` perderam
> `/docker`, `/md` e `/open` e ficaram com o que só roda dentro da sessão: a guarda de
> comando destrutivo, as tools do Claude, `/dlog` (log para o prompt) e os relatos
> "markdown escrito" e "último arquivo editado" ao engine. O texto abaixo é o levantamento
> original, anterior a essa decisão.

> Data: 2026-10-08. Somente análise; nenhum código foi alterado.
> Fonte: leitura do código do repositório e da doc oficial de mods
> (<https://code.claude.com/docs/en/plugins/mods/>). Tipos da API conferidos
> contra a versão 2.1.294 do Claude Code.

## Conclusão

O Tesseract não vira mod por inteiro. A casca (mosaico, daemon, PTY, teclado)
fica em Go. O que migra é o que hoje ele adivinha de fora, mais as ferramentas
de projeto.

- **Por quê:** o Tesseract roda **por fora** do Claude Code (binário Go, daemon
  e TUI com várias células em PTY). Um mod roda **dentro de uma** sessão. A
  arquitetura resultante é híbrida.
- **Placar pela tabela:** 9 viram mod direto, 10 com adaptação, 15 não cabem e
  2 ficam redundantes. O resumo do subagente dizia 7/9/15/4; a contagem linha a
  linha das tabelas abaixo dá 9/10/15/2.
- **Código que sai:** cerca de 450 das cerca de 10.800 linhas de produção, com
  certeza. Até cerca de 1.900 a mais se Docker, markdown e IDE migrarem de vez.
- **Esforço:** 8 a 12 dias úteis, começando por um spike de 0,5 dia.

### Exemplo do ganho central

Hoje "pediu aprovação" é inferido porque o texto "Do you want to" apareceu na
tela, lida a cada 500 ms, com 3 leituras para armar e 6 de silêncio para
encerrar (`internal/cell/turn.go:13-22`). Com mod, o evento chega pronto e diz
qual ferramenta e qual comando estão esperando.

## Como o Tesseract integra com o Claude Code hoje

Zero hooks, zero MCP, zero SDK. São cinco pontos, todos por fora:

1. Lança `claude --session-id` ou `--resume` no PTY
   (`internal/cell/claude.go:52-85`).
2. Descobre o estado do turno raspando a tela (`internal/cell/turn.go:76`,
   `internal/cell/agent.go:76`).
3. Manda prompt colando texto no PTY e esperando 120 ms antes do Enter
   (`internal/cell/agent.go:138-160`).
4. Lê o título da conversa varrendo o `.jsonl` em `~/.claude/projects/`
   (`internal/cell/claude.go:117-158`).
5. Lê a quota de um arquivo que o statusline do usuário precisa escrever
   (`internal/engine/quota.go:21,61`).

O próprio projeto assume o limite: não consegue pintar a interface do agente
(`internal/cell/bash.go:39-47`). A saída oferecida hoje é exportar
`TESSERACT=1` e recomendar o tema `dark-ansi` à mão (`themes/claude-code.md`).

## Tabela de features

Vereditos: **D** direto · **A** com adaptação · **N** não cabe · **R**
redundante (o Claude Code já faz).

### Núcleo (contêiner)

| Feature | Onde | V | Motivo / limite |
|---|---|---|---|
| Mosaico, layout, navegação por setas | `internal/screen/grid.go:97` | N | Mod só desenha em sites da própria sessão |
| Daemon que sobrevive a fechar a tela e a reboot | `internal/engine/engine.go:88`, `internal/engine/state.go:48` | N | Mod nasce e morre com a sessão |
| PTY, emulador, scrollback, replay | `internal/cell/process.go:52` | N | Processo iniciado por mod não tem TTY nem teclado |
| Modos NAVEGAR/DIGITAR, keymap único | `internal/keyboard/keyboard.go:128-197` | N | Mod não lê teclado |
| Sessão com 4 abas, sono de aba ociosa | `internal/cell/session.go:23` | N | Cursor e bash são outros processos |
| Células cursor, bash, shell em contêiner | `internal/cell/cursor.go`, `bash.go`, `shell.go` | N | Idem |
| Retomar conversa sem disparar trabalho | `internal/cell/claude.go:52-85` | N | O engine lança o processo; o mod só informa o id real da sessão |
| Tela cheia, lista, pular para quem chama | `internal/screen/model.go:389`, `internal/screen/list.go:25` | N | Mod não troca de terminal |
| Criar célula (navegador de pastas com recentes) | `internal/screen/form.go:62-333` | N | Sessão não cria sessão irmã |
| CLI `ts`, instalador | `cmd/tess/main.go:242-346` | N | Fora do processo do Claude |
| Diretório órfão, PATH de login | `internal/engine/watcher.go:116` | N | Existem por causa do daemon |
| Seleção com mouse e cópia | `internal/screen/selection.go` | R | Dentro da sessão o Claude Code já faz; no mosaico continua sendo do Tesseract |
| Colagem marcada como paste | `internal/screen/model.go:296` | R | Só existe por estar entre o terminal e o agente |

### Estado, alertas, quota

| Feature | Onde | V | Mecanismo | Ganho | Perda |
|---|---|---|---|---|---|
| Estado do turno | `internal/cell/turn.go:76-124` | D | Eventos de turno, tool e permissão | Zero heurística; não quebra quando um texto da tela muda | Só Claude; cursor segue raspando |
| Notificação com som e toast do Windows | `internal/engine/notice.go:15-88` | D | Fim de turno ou permissão dispara o notificador | Corpo com conteúdo real ("quer rodar `pnpm migrate`") | "Crashed" fica no engine |
| Badge de quota das 5 h | `internal/engine/quota.go:37-87` | D | Medição de sessão em push | Some o arquivo do statusline; ganha janela de 7 dias, % de contexto e custo | — |
| Barra APPROVE piscando | `internal/theme/theme.go:358-427` | A | Faixa acima do prompt | Pouco | O valor do sinal está no mosaico |
| Contadores "quem está chamando" | `internal/screen/drawing.go:177-195` | A | Cada sessão grava seu estado no store compartilhado | Funciona em `claude` solto, fora do Tesseract | Sem o pulo; corrida entre sessões |
| Busca no histórico | `internal/engine/history/history.go:141` | A | Comando sobre as mensagens, resultado em painel | Busca estruturada, não em bytes ANSI | Só 4.096 entradas; sem rolar até o ponto |

### Ações

| Feature | Onde | V | Mecanismo | Ganho | Perda |
|---|---|---|---|---|---|
| Painel Docker | `internal/docker/docker.go:50`, `internal/screen/docker.go:87` | D | Comando abre painel; botões com hotkey | Claude vê e reinicia serviço sozinho; guarda nega `down -v` no Bash | Hotkey só dígito ou minúscula |
| Faixa de resumo Docker | `internal/engine/watcher.go:134` | D | Linha de status | — | Prefixo fixo com o nome do mod |
| Abrir no IDE | `internal/engine/actions.go:144-440` | D | Comando abre seletor | Abrir o arquivo recém-editado, na linha | IDE que fica vivo pode prender o processo (spike) |
| Configuração em JSON | `internal/engine/config.go:25-46` | D | Config declarada no manifesto | Editável pelo próprio Claude Code | A config do engine continua |
| Log de serviço | `internal/cell/logs.go:32-71` | A | Processo em follow, cauda no painel | Erro de log vai ao prompt em 1 tecla | Cauda curta, sem scrollback de 2.000 linhas |
| Leitor de markdown | `internal/cell/md.go:68-522` | A | Comando abre painel com lista e filtro | Abre sozinho o `.md` recém-escrito | Perde o estilo de página (coluna de 82, banner); sem watch, só polling |
| Prompt sem entrar na célula | `internal/cell/agent.go:138-150` | A | Engine deixa o pedido; mod envia como fala do usuário | Acaba a corrida colar + 120 ms + Enter | Precisa de canal engine → mod |
| Renomear / adotar título | `internal/engine/actions.go:94-133` | A | Roda o comando de rename | Dispensa a cola no PTY | Sem API de título; a leitura do `.jsonl` provavelmente fica |
| Ajuda e dicas de rodapé | `internal/screen/boxes.go:73` | A | Dica do prompt | — | Só cobre o que o mod adiciona |

### Estilização

| Feature | Onde | V | Mecanismo | Ganho | Perda |
|---|---|---|---|---|---|
| Claude Code sem cor própria na célula | `themes/claude-code.md` (passo manual) | D | Troca o tema no início se `TESSERACT=1` | Some o passo manual | Parece config global: afetaria sessões fora do Tesseract (spike) |
| Identidade dentro do transcript | Hoje impossível (`internal/cell/bash.go:39-47`) | D | Redesenho de spinner, fim de turno e linhas de tool | Maior ganho visual: "⬤ ANSWERED · 42s" | Prompt de permissão e caixa do prompt não são redesenháveis |
| Paleta única, 3 perfis, `NO_COLOR` | `internal/theme/theme.go:34-80` | A | Cor aceita hex ou chave do tema | Tokens reaproveitados | Mod não repinta a paleta do Claude Code |
| Statusline com `⧉` | `internal/cell/bash.go:47` | A | Faixa `⧉ projeto/célula · quota · docker` | Independe de script do usuário | — |
| Abertura animada da marca | `internal/screen/opening.go:107` | N | — | — | O logo do Claude Code não é redesenhável |
| Título da janela | `internal/screen/model.go:916` | N | — | — | Sem API de título |
| Leis de cor do mosaico | `internal/theme/theme.go:6-18` | N | — | — | São regras do mosaico |
| Temas de terminal, tmux, nvim etc. | `themes/*` | N | — | — | Config de outras ferramentas |

## O que apaga e o que fica

### Apaga com o mod sensor (cerca de 450 linhas)

- Raspagem de turno inteira (`internal/cell/turn.go`, marcadores e laço).
- Opções de marcadores configuráveis (`internal/engine/config.go:17-21`).
- Leitura de quota (`internal/engine/quota.go`) e o script de statusline.
- Atraso de 120 ms no envio de prompt.
- Passo manual do tema.

### Não apagar agora (cerca de 1.900 linhas)

Docker, markdown, IDE e logs. No mosaico essas features são alcançáveis de
qualquer célula; como mod, só de dentro de uma célula Claude. Tratar o mod como
ganho adicional e apagar o Go só depois de uso real.

### Fica fora de mod, sem alternativa

Engine, protocolo, PTY, grid, keymap, persistência, células não-Claude, CLI e
instalador.

## Ordem de migração

Distribuição: o repositório vira marketplace e o engine lança cada célula com
`claude --plugin-dir <caminho>`. Os mods carregam só em célula, sem instalação.

| # | Plugin | Conteúdo | Valor / risco | Esforço |
|---|---|---|---|---|
| 0 | Spike | Ordem dos eventos de permissão, escopo do tema, canal mod → engine, IDE destacado | Destrava 1, 2 e 5 | 0,5 dia |
| 1 | `tesseract-link` | Estado do turno, id real da sessão, quota, notificação, caixa de prompt, tema | Alto / médio; único que apaga código | 2-3 dias |
| 2 | `tesseract-skin` | Spinner, fim de turno, linhas de tool, faixa `⧉` | Alto percebido / baixo | 1-2 dias |
| 3 | `tesseract-docker` | Painel, tool para o Claude, guarda, cauda de log | Médio-alto / médio | 2-3 dias |
| 4 | `tesseract-md` | Lista, filtro, leitor, abertura automática | Médio / baixo | 1,5-2 dias |
| 5 | `tesseract-ide` | Seletor e "abrir arquivo editado" | Baixo-médio / baixo | 0,5-1 dia |
| 6 | `tesseract-roster` (opcional) | Contadores de outras sessões | Baixo | 1-2 dias |

## Riscos e incógnitas

### Verificado no código ou na doc

- API em acesso antecipado: muda entre releases sem aviso.
- Mouse não chega à célula: a TUI consome os cliques. Dentro do Tesseract,
  botão de mod é só por teclado.
- Largura: painel lateral pede 110 colunas, ou 144 se aberto sozinho. A célula
  nasce com 80 e pode chegar a 34. Na prática, o painel abre como bloco acima
  do prompt e só por comando.
- Imagem não funciona na célula; desenho por células de caractere funciona.
- Sem sandbox; três quedas descarregam todos os mods.

### Inferência, não testado

- Bug latente: o id da conversa Claude é fixado no spawn e nunca atualizado.
  Após `/clear` ou `/resume` dentro da célula, um reboot retomaria a conversa
  antiga. O mod resolveria.
- Tema customizado pode existir e ser melhor que o `dark-ansi`.

### Pede spike

1. Sequência exata de eventos até a aprovação; pergunta ao usuário e saída do
   modo plano contam como "esperando você"?
2. Troca de tema: global ou por sessão.
3. A faixa acima do prompt continua visível com o diálogo de permissão aberto?
4. Título da conversa por API.
5. Lançar IDE sem prender o processo.

### Não confirmado

A afirmação "sessão WSL no app Desktop não roda plugins" veio da primeira
leitura da doc e não foi encontrada em 6 páginas na segunda leitura. Tratar
como não confirmada.

As páginas `/admin`, `/events` (releitura) e `/test` da doc não foram lidas no
levantamento.

## Achado lateral: docs do repositório defasadas

- `docs/tema.md` cita `TESSERACT_SEM_PISCA` e `TESSERACT_SEM_ABERTURA`; o
  código lê `TESSERACT_NO_BLINK` e `TESSERACT_NO_OPENING`.
- `docs/atalhos.md` lista `R` para renomear; o keymap só tem `ctrl+r`.
- `docs/arquitetura.md` cita as pastas `motor/`, `celula/` e `tela/`; o código
  usa `engine/`, `cell/` e `screen/`.

## Próximo passo

Rodar o spike de 0,5 dia para validar os 5 pontos antes de escrever o
`tesseract-link`.
