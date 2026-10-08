# tesseract-skin

Mod do Claude Code que aplica a identidade **Tesseract Neon** dentro da sessão
que roda em uma célula. O Tesseract não consegue pintar a interface do agente
por fora; este mod a redesenha por dentro.

Fonte da verdade visual: `style/tesseract-brand-identity.html`. Cor e glifo
ficam em um arquivo só, `hooks/tokens.ts`.

## O que muda

| Superfície | Antes | Depois |
|---|---|---|
| Spinner | `Sautéing… (12s, 300 tokens)` | `▸ WORKING · thinking · 12s` |
| Spinner com diálogo aberto | `Sautéing… (52s)` | `Blocked, waiting on you.`, sem contar tempo |
| Fim de turno | `Baked for 42s` | `⬤ RESPONDED · 42s`; `○ INTERRUPTED · 4s` se a pessoa interrompeu ou recusou o diálogo; `✖ FAILED · 4s` em erro |
| Chamada de ferramenta | `● Bash(pnpm test)` | `▸ Bash pnpm test`, depois `·`, `✖ … · failed` ou `○ … · interrupted` |
| Chamada esperando aprovação, dentro de uma célula | `● Bash(pnpm migrate)` | `⏵ APPROVE  Bash: pnpm migrate  Enter approves, Esc declines`: glifo e rótulo em âmbar, fundo normal, sem piscar |
| Chamada esperando aprovação, fora do Tesseract | `● Bash(pnpm migrate)` | barra ` ⏵ APPROVE  Bash: pnpm migrate`, invertida em âmbar, piscando a cada 2 s |
| Pergunta do agente e plano pronto | `● AskUserQuestion` | `⏵ APPROVE  tabs or spaces?  Enter answers, Esc declines` e `⏵ APPROVE  plan ready for review` |
| Grupo de ferramentas | `Read 3 files, ran 2 shell commands` | `· Read ×3 · Bash ×2` |
| Prompt enviado (digitado ou mandado do mosaico) | `> texto` | `› texto` sobre fundo elevado |
| Dica do prompt | `esc to interrupt` | `Esc interrupts`, seguido do que outros mods acrescentaram (`· /find`) |
| Modos da sessão | `focus & memory paused` | `FOCUS · MEMORY PAUSED` |
| Faixa acima do prompt | vazia | `⧉ projeto/célula · ⬤ RESPONDED · 5h 23% · ctx 41%`; ociosa, só `⧉ projeto/célula · 5h 23% · ctx 41%` |

O resultado da ferramenta (diff, saída do comando) continua desenhado pelo
Claude Code: o tema `dark-ansi` já o alinha à paleta (`themes/claude-code.md`).

## Regras da marca que o mod segue

- Verde fósforo não aparece: é posse do teclado, um por tela, e quem o acende é
  o mosaico.
- Ciano é estrutura: `⧉`, rótulo da célula, nome da ferramenta, modos.
- Estado é forma antes de cor: `▸` `⬤` `⏵` `✖` `○`, cada um com sua palavra.
- Um badge invertido por tela. Dentro de uma célula (`TESSERACT=1`) esse badge
  é a barra `⏵ APPROVE` do cabeçalho do mosaico, então a linha da chamada fica
  só com a forma `⏵` e o âmbar. Fora do Tesseract (`outside_tesseract`) a linha
  é a única barra, e aí sim invertida e piscando. Nos dois casos só a primeira
  chamada da fila pede aprovação; as demais esperam como `▸`.
- A faixa não contradiz o mosaico: o desfecho do turno (`⬤ RESPONDED`,
  `○ INTERRUPTED`, `✖ FAILED`) sai na primeira edição do prompt depois do fim do
  turno, sinal de que a pessoa voltou à sessão. Ociosa, a faixa não tem estado.
- Recusar o diálogo com Esc ou interromper o turno não é `RESPONDED`: é
  `○ INTERRUPTED`, na faixa e na linha de fim de turno.
- Com `NO_COLOR` nenhuma cor é emitida; a barra usa vídeo invertido e não pisca
  (dentro da célula, a marca fica só em negrito).

## Limites

- A faixa acima do prompt **some enquanto o diálogo de permissão está aberto**
  (o diálogo ocupa o lugar do prompt). Por isso o `⏵ APPROVE` é a linha da
  chamada no transcript, logo acima do diálogo, e não a faixa.
- Entrar na célula sem tocar em nenhuma tecla não limpa a faixa: o mod só sabe
  que a pessoa voltou quando o prompt é editado.
- Um prompt mandado do mosaico ganha o desenho de fala da pessoa, mas a linha
  `› Prompt from the tesseract-link plugin` acima dele é do Claude Code e fica.
- A dica do prompt mantém o que outro mod acrescentou quando esse mod é
  carregado antes do skin (a ordem do Tesseract, alfabética, garante isso para o
  `tesseract-roster`). Carregado depois, o acréscimo dele se perde: reescrever a
  dica descarta o complemento, e o skin não enxerga o que vem abaixo dele.
- A linha de fim de turno de uma sessão retomada (ou depois de um reload do mod)
  sai como `⬤ RESPONDED`: o desfecho só é conhecido para turnos que o mod viu
  acabar.
- O diálogo de permissão, a caixa do prompt e o logo não são redesenháveis.
- O spinner próprio mostra fase e tempo; a contagem de tokens do original não
  chega ao mod.
- Agent e tarefas mantêm a linha viva do Claude Code. Chamadas de um grupo
  aberto com `ctrl+o` também, porque só ele desenha o resultado embutido: nessa
  visão convivem linhas `● Read(...)` e `· Bash ...`.
- O rótulo `projeto/célula` vem de `TESSERACT_CELL`, que o engine exporta em
  toda célula; fora do Tesseract, é o nome da pasta da sessão.
- Hex exato depende de terminal truecolor, que a célula garante
  (`COLORTERM=truecolor`). Dentro do tmux o Claude Code rebaixa para 256 cores.

## Configuração

`/config`, ou `pluginConfigs["tesseract-skin"].options` no settings:

| Opção | Padrão | Efeito |
|---|---|---|
| `outside_tesseract` | `false` | Age mesmo sem `TESSERACT=1`; a linha de approve vira a barra invertida |
| `band` | `true` | Faixa acima do prompt |
| `blink` | `true` | Pisca da barra de approve (só fora do Tesseract) |
| `spinner`, `turn_end`, `tools`, `user_message`, `prompt_hint`, `session_mode` | `true` | Liga cada superfície |

Ambiente lido: `TESSERACT`, `TESSERACT_CELL`, `TESSERACT_NO_BLINK`, `NO_COLOR`.

## Desenvolver

```bash
claude plugin validate mods/tesseract-skin --strict
claude plugin test mods/tesseract-skin
TESSERACT=1 claude --plugin-dir mods/tesseract-skin
```

Se `claude` for um alias com opções, `claude plugin test` recusa: use
`command claude plugin test`.
