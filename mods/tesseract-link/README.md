# tesseract-link

Mod do Claude Code que liga a sessão de uma célula ao engine do Tesseract. Sem
ele o engine adivinha o estado do agente lendo a tela; com ele, a própria sessão
conta.

Só age dentro de uma célula (`TESSERACT=1` e `TESSERACT_CELL_ID`). Nada sai da
máquina: os dois caminhos terminam no socket local do engine.

## Os dois caminhos

| Sentido | Como | O que passa |
|---|---|---|
| mod → engine | `ts report <json>`, um processo por evento | estado do turno, id da conversa, quota, rename concluído, recibo de entrega |
| engine → mod | `ts link`, um processo que vive com a sessão | um prompt por linha JSON, mandado do mosaico |

## O que é reportado

| Momento | Relato | Efeito no mosaico |
|---|---|---|
| Turno começa | `working` | `▸ working` |
| Diálogo de permissão, pergunta do agente ou plano pronto | `approve` + o que espera (`Bash: pnpm migrate`) | barra `⏵ APPROVE` e aviso |
| Turno termina com resposta | `answered` + primeira linha da resposta | `⬤ responded` e aviso |
| Pessoa interrompe o turno ou recusa o diálogo com Esc | `working`, sem detalhe | nenhum aviso: quem parou está olhando |
| Turno termina sem texto | `working`, sem detalhe | nenhum aviso: não há o que ler |
| `/clear`, `/resume`, `/branch` | id novo da conversa | o reboot retoma a conversa certa |
| Medição de uso | janela de 5 horas | selo de quota |
| Sessão encerra | `ended` | a célula mostra o processo saindo |

O protocolo não tem estado próprio para "ocioso" nem para "interrompido": os
dois vão como `working` sem detalhe.

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

`refused` é a sessão sem onde pôr a entrega: com a conversa mandada para o
fundo (`←` no prompt vazio) o processo da célula só mostra a lista de agentes
do Claude Code, e `$.prompt.submit` rejeita. Uma entrega que chega depois de
`expires` é descartada, porque o engine já colou o texto.

## Sessões de fundo ficam mudas

As sessões de fundo do Claude Code (a conversa mandada para trás e a reserva
que o serviço de fundo mantém aquecida) nascem do processo da célula e herdam
`TESSERACT_CELL_ID` e os mods. Não são o que a célula mostra: com
`CLAUDE_CODE_SESSION_KIND=bg` o mod não reporta nem abre a linha, e o `ts`
recusa `report` e `link` vindos de uma delas. Sem isso a última a falar
trocava o id da conversa da célula pelo dela e ficava com os prompts.

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
| `notifyDetail` | `true` | Manda o que espera e o começo da resposta. Desligado, nenhum texto da conversa sai e o aviso fica genérico |
| `quota` | `true` | Reporta a janela de 5 horas a cada medição |

Ambiente lido: `TESSERACT`, `TESSERACT_CELL_ID`, `CLAUDE_CODE_SESSION_KIND`, `TESSERACT_BIN` (o binário que
fala com o engine; sem ela, `ts`).

## Desenvolver

```bash
claude plugin validate mods/tesseract-link --strict
claude plugin test mods/tesseract-link
```

Os testes ficam em `tests/`. Se `claude` for um alias com opções, use
`command claude plugin test`.
