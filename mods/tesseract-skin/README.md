# tesseract-skin

Mod do Claude Code que leva o desenho do Tesseract para dentro da sessão que
roda em uma célula. O Tesseract não consegue pintar a interface do agente por
fora; este mod a redesenha por dentro.

Fonte da verdade visual: `internal/theme/theme.go`, descrita em `docs/tema.md`.
Cor e glifo ficam em um arquivo só, `hooks/tokens.ts`.

## O que muda

Dentro de uma célula (`TESSERACT=1`) o estado é **só o glifo**, o mesmo do
cabeçalho do mosaico. Fora do Tesseract (`outside_tesseract`) não há mosaico em
volta para dizer o estado, então o glifo vem com a palavra.

| Superfície | Antes | Dentro de uma célula | Fora do Tesseract |
|---|---|---|---|
| Spinner | `Sautéing… (12s, 300 tokens)` | `▸ thinking · 12s` | `▸ WORKING · thinking · 12s` |
| Spinner com diálogo aberto | `Sautéing… (52s)` | `Blocked, waiting on you.`, sem contar tempo | idem |
| Fim de turno | `Baked for 42s` | `⬤ 42s`; `◐ 4s` se a pessoa interrompeu ou recusou o diálogo; `✖ 4s` em erro | `⬤ RESPONDED · 42s`, `◐ INTERRUPTED · 4s`, `✖ FAILED · 4s` |
| Chamada de ferramenta | `● Bash(pnpm test)` | `▸ Bash pnpm test`, depois `·`, `✖ … · failed` ou `◐ … · interrupted` | idem |
| Chamada esperando aprovação | `● Bash(pnpm migrate)` | `⏵ Bash: pnpm migrate  Enter approves, Esc declines`: glifo em âmbar, fundo normal, sem piscar | barra ` ⏵ APPROVE  Bash: pnpm migrate`, sólida em âmbar, piscando a cada 2 s |
| Pergunta do agente e plano pronto | `● AskUserQuestion` | `⏵ tabs or spaces?  Enter answers, Esc declines` e `⏵ plan ready for review` | a mesma barra, com `APPROVE` |
| Grupo de ferramentas | `Read 3 files, ran 2 shell commands` | `· Read ×3 · Bash ×2` | idem |
| Prompt enviado (digitado ou mandado do mosaico) | `> texto` | `› texto` sobre fundo elevado | idem |
| Dica do prompt | `esc to interrupt` | `Esc interrupts`, seguido do que outros mods acrescentaram (`· /find`) | idem |
| Modos da sessão | `focus & memory paused` | `FOCUS · MEMORY PAUSED` | idem |
| Faixa acima do prompt | vazia | **não é desenhada** | `⧉ pasta · ⬤ RESPONDED · 5h 23% · ctx 41%`; ociosa, só `⧉ pasta · 5h 23% · ctx 41%` |

Dentro de uma célula cada dado aparece uma vez na tela: o cabeçalho do mosaico
diz célula e estado, a linha de status do mosaico diz a quota, e a barra
`⏵ APPROVE` — o único estado escrito por extenso, e a única área sólida — é a
do cabeçalho. O mod não repete nenhum deles: sem faixa, sem palavra de estado,
sem barra.

O resultado da ferramenta (diff, saída do comando) continua desenhado pelo
Claude Code: o tema `dark-ansi` já o alinha à paleta (`themes/claude-code.md`).

## Regras de desenho que o mod segue

- Verde fósforo não aparece: é onde a digitação cai, um por tela, e quem o
  acende é o mosaico (a palavra `TYPE` na célula).
- O matiz do projeto também não: é da borda da célula. Aqui dentro só existem a
  escada neutra e as cores de estado.
- Ciano é só a marca: o `⧉` da faixa, fora do Tesseract. Nome de ferramenta,
  rótulo e modos saem em tom secundário.
- Estado é glifo antes de cor, e o alfabeto é o do mosaico: `▸` trabalhando,
  `⬤` respondeu, `⏵` aprovar, `✖` falhou, `◐` interrompido, `○` parado.
- Uma área sólida por tela. Dentro de uma célula ela é a barra `⏵ APPROVE` do
  cabeçalho do mosaico, então a linha da chamada fica só com a forma `⏵` e o
  âmbar. Fora do Tesseract a linha é a única barra, e aí sim sólida e piscando.
  Nos dois casos só a primeira chamada da fila pede aprovação; as demais
  esperam como `▸`.
- Fora do Tesseract, o desfecho do turno na faixa (`⬤ RESPONDED`,
  `◐ INTERRUPTED`, `✖ FAILED`) sai na primeira edição do prompt depois do fim do
  turno, sinal de que a pessoa voltou à sessão. Ociosa, a faixa não tem estado.
- Recusar o diálogo com Esc ou interromper o turno não é `⬤`: é `◐`, na faixa
  e na linha de fim de turno.
- Com `NO_COLOR` nenhuma cor é emitida; a barra usa vídeo invertido e não pisca
  (dentro da célula, a marca fica só em negrito).

## Limites

- A faixa acima do prompt (fora do Tesseract) **some enquanto o diálogo de
  permissão está aberto** (o diálogo ocupa o lugar do prompt). Por isso o
  `⏵` é a linha da chamada no transcript, logo acima do diálogo, e não a
  faixa.
- A faixa só sabe que a pessoa voltou quando o prompt é editado.
- Um prompt mandado do mosaico ganha o desenho de fala da pessoa, mas a linha
  `› Prompt from the tesseract-link plugin` acima dele é do Claude Code e fica.
- A dica do prompt mantém o que outro mod acrescentou quando esse mod é
  carregado antes do skin (a ordem do Tesseract, alfabética, garante isso para o
  `tesseract-roster`). Carregado depois, o acréscimo dele se perde: reescrever a
  dica descarta o complemento, e o skin não enxerga o que vem abaixo dele.
- A linha de fim de turno de uma sessão retomada (ou depois de um reload do mod)
  sai como `⬤`: o desfecho só é conhecido para turnos que o mod viu
  acabar.
- O diálogo de permissão, a caixa do prompt e o logo não são redesenháveis.
- O spinner próprio mostra fase e tempo; a contagem de tokens do original não
  chega ao mod.
- Agent e tarefas mantêm a linha viva do Claude Code. Chamadas de um grupo
  aberto com `ctrl+o` também, porque só ele desenha o resultado embutido: nessa
  visão convivem linhas `● Read(...)` e `· Bash ...`.
- O rótulo da faixa é o nome da pasta da sessão.
- Hex exato depende de terminal truecolor, que a célula garante
  (`COLORTERM=truecolor`). Dentro do tmux o Claude Code rebaixa para 256 cores.

## Configuração

`/config`, ou `pluginConfigs["tesseract-skin"].options` no settings:

| Opção | Padrão | Efeito |
|---|---|---|
| `outside_tesseract` | `false` | Age mesmo sem `TESSERACT=1`; a linha de approve vira a barra invertida |
| `band` | `true` | Faixa acima do prompt. Só tem efeito fora do Tesseract: dentro de uma célula ela nunca é desenhada |
| `blink` | `true` | Pisca da barra de approve (só fora do Tesseract) |
| `spinner`, `turn_end`, `tools`, `user_message`, `prompt_hint`, `session_mode` | `true` | Liga cada superfície |

Ambiente lido: `TESSERACT`, `TESSERACT_NO_BLINK`, `NO_COLOR`.

## Desenvolver

```bash
claude plugin validate mods/tesseract-skin --strict
claude plugin test mods/tesseract-skin
TESSERACT=1 claude --plugin-dir mods/tesseract-skin
```

Se `claude` for um alias com opções, `claude plugin test` recusa: use
`command claude plugin test`.
