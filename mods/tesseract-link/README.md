# tesseract-link

Mod do Claude Code que liga a sessão de uma célula ao engine do Tesseract. Sem
ele o engine adivinha o estado do agente lendo a tela; com ele, a própria sessão
conta.

Só age dentro de uma célula (`TESSERACT=1` e `TESSERACT_CELL_ID`). Nada sai da
máquina: os dois caminhos terminam no socket local do engine.

## Os dois caminhos

| Sentido | Como | O que passa |
|---|---|---|
| mod → engine | `ts report <json>`, um processo por evento | estado do turno, id da conversa, quota, contexto, rename concluído, recibo de entrega |
| engine → mod | `ts link`, um processo que vive com a sessão | um prompt por linha JSON, mandado do mosaico; e `{"event":"attached"}` quando a linha é atada |

## O que é reportado

| Momento | Relato | Efeito no mosaico |
|---|---|---|
| Linha com o engine atada (sessão nova, ou linha refeita) | o estado em que a sessão está; parada no prompt, `idle` | célula sem marcador de atividade |
| Turno começa | `working` | `▸` no cabeçalho da célula |
| Diálogo de permissão, pergunta do agente ou plano pronto | `approve` + o que espera (`Bash: pnpm migrate`) + `ask` (`tool`, `question`, `plan`) | barra `⏵ APPROVE`, aviso, e a linha de status do mosaico dizendo o que espera |
| Turno termina com resposta | `answered` + primeira linha da resposta (+ `summary`, com a opção ligada) | `⬤` (com o resumo antes dele, quando cabe) e aviso |
| Pessoa interrompe o turno ou recusa o diálogo com Esc | `interrupted` | `◐`, sem aviso: quem parou está olhando |
| Turno termina sem texto | `interrupted` | idem: não há o que ler |
| `/clear`, `/resume`, `/branch` | id novo da conversa, e `idle` | o reboot retoma a conversa certa |
| Medição de uso | `quota`: janelas de 5 horas e de 7 dias | `quota 12%` na linha de status (uma só; a de 7 dias só entra quando é a que está acabando) |
| Medição de uso, ou `/clear` | `context`: preenchimento da janela da conversa, quando muda | `ctx 86%` no cabeçalho da célula, só a partir de 80% |
| Sessão encerra | `ended` | a célula mostra o processo saindo |

`answered` é o nome do estado no protocolo; na tela ele é o glifo `⬤`, e a
palavra `responded` só aparece na legenda da ajuda (`?`) e nos avisos. O mosaico
escreve por extenso um único estado, `⏵ APPROVE`.

Os relatos de estado só valem com a linha `ts link` atada, e o mod carrega antes
dela. Por isso a primeira coisa que o engine manda numa linha nova é
`{"event":"attached"}`, e o mod responde dizendo em que pé a sessão está. É o
que faz a célula recém-criada aparecer ociosa, e não `▸`.

`idle` nunca derruba um `⬤` que ninguém leu: quem tira essa marca é a
pessoa entrando na célula, e o que sobra aí é ocioso.

### O que o `ask` libera

Só `ask: "tool"` — o diálogo de permissão de uma ferramenta, que é sim ou não —
pode ser resolvido do mosaico: com a célula focada, o rodapé mostra
`⏵ Bash: pnpm migrate … Enter approves  Esc declines`, e o engine digita a tecla
no terminal da célula. Pergunta com opções (`question`) e plano para revisar
(`plan`) pedem a pessoa dentro da célula.

Limite conhecido: com mais de um diálogo na fila, o mod só reporta cada um
quando ele é pedido; depois da primeira resposta a célula sai de `APPROVE` até
o próximo relato.

## O que é recebido

Uma linha que abre com `/comando` conhecido pela sessão roda como comando (o
`/rename` avisa o engine quando termina). O resto entra como fala da própria
pessoa, na fila atrás do turno em curso.

Cada entrega tem recibo. A linha traz `id` e `expires`; o mod põe a entrega na
fila da sessão e responde `ts report {"kind":"delivered","delivery":"<id>"}`.
O engine espera esse recibo por 2 s:

| O que o engine ouve | O que faz |
|---|---|
| recibo | entrega feita |
| recibo com `refused` | avisa no rodapé do mosaico; nada é colado |
| nada no prazo | cola o texto no terminal da célula, como numa célula sem mod |

`refused` é a sessão sem onde pôr a entrega (`$.prompt.submit` rejeitou). Com a
conversa mandada para o fundo (`←` no prompt vazio) e a lista de agentes na
tela, o engine nem chega a entregar: ele lê a lista na tela da célula e avisa
no rodapé. Uma entrega que chega depois de `expires` é descartada, porque o
engine já colou o texto.

## Sessões de fundo: uma fala, as outras ficam mudas

As sessões de fundo do Claude Code nascem do processo da célula e herdam
`TESSERACT_CELL_ID` e os mods (`CLAUDE_CODE_SESSION_KIND=bg`). São duas espécies:

| Sessão | `CLAUDE_BG_SOURCE` | Fala pela célula? |
|---|---|---|
| A conversa mandada para trás com `←` (ou `/bg`) | `slash` | sim |
| A reserva que o serviço de fundo mantém aquecida, e o que for despachado da lista de agentes | `spare` e outros | não |

`←` não encerra nada no processo da célula: o Claude Code faz um **fork** da
conversa numa sessão nova, hospedada pelo serviço de fundo, e `Esc` transforma
o processo da célula em visor dessa sessão — sem evento nenhum para os mods.
Dali em diante é o fork que roda os turnos. Por isso ele fala: reporta o id
novo (que o engine guarda, e é o que um reinício retoma), abre a própria linha
e, por ser a mais nova, passa a ser a dona das entregas.

As outras ficam mudas: o mod não reporta nem abre a linha, e o `ts` recusa
`report` e `link` vindos delas. Sem isso a última a falar trocava o id da
conversa da célula pelo dela e ficava com os prompts.

Enquanto a lista de agentes está na tela (`◌`), o engine recusa `p` e o rename
com o que apertar para voltar. Ao subir a célula de novo, o engine roda
`claude stop <id>` antes do `--resume` quando a conversa é uma dessas sessões
hospedadas: o Claude Code recusa retomar uma sessão que ainda roda no fundo.

Limite conhecido: se, na lista de agentes, a pessoa abrir uma sessão que não é
a conversa da célula (de outro projeto, por exemplo), o mosaico continua
falando da conversa da célula, não da que está na tela.

Várias linhas para a mesma célula: a mais nova é a dona e recebe as entregas;
quando ela cai, a anterior volta a ser a dona.

## Reconexão

A linha `ts link` é refeita enquanto a sessão viver. A espera dobra a cada queda
seguida (2 s, 4 s, 8 s… até 60 s) e volta a 2 s depois de uma conexão que durou
10 s. Enquanto ela está caída, o engine volta a ler a tela.

## Configuração

`/config`, ou `pluginConfigs["tesseract-link"].options` no settings:

| Opção | Padrão | Efeito |
|---|---|---|
| `notifyDetail` | `true` | Manda o que espera e o começo da resposta. Desligado, nenhum texto da conversa sai e o aviso fica genérico (o rodapé do mosaico ainda oferece Enter/Esc, sem dizer o comando) |
| `quota` | `true` | Reporta as janelas de 5 horas e de 7 dias e o preenchimento do contexto a cada medição |
| `summarize` | `false` | Ao fim do turno, uma chamada ao modelo mais barato (`haiku`, via `$.model.complete`) resume a resposta em até 60 caracteres. O resumo aparece no cabeçalho da célula, antes do glifo `⬤` (o cabeçalho não tem a palavra `responded`), e no aviso, no lugar do começo da resposta. Custa uma chamada curta por turno e segura o aviso por até 6 s; se a chamada falha, o aviso sai como sairia sem a opção. Exige `notifyDetail` |

Ambiente lido: `TESSERACT`, `TESSERACT_CELL_ID`, `CLAUDE_CODE_SESSION_KIND`, `CLAUDE_BG_SOURCE`, `TESSERACT_BIN` (o binário que
fala com o engine; sem ela, `ts`).

## Desenvolver

```bash
claude plugin validate mods/tesseract-link --strict
claude plugin test mods/tesseract-link
```

Os testes ficam em `tests/`. Se `claude` for um alias com opções, use
`command claude plugin test`.
