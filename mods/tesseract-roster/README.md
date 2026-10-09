# tesseract-roster

Mod do Claude Code: quem está chamando, busca na conversa e as dicas dos
comandos Tesseract. É para o `claude` solto: dentro de uma célula do
Tesseract o mosaico já mostra quem chama, e o engine não carrega este mod.
Lá, os comandos que sobraram aparecem no menu de `/` com a própria descrição
e na ajuda `?` do mosaico.

## O que faz

- **Rol de sessões**: cada sessão grava seu estado (`working`, `responded`,
  `approve`, `idle`) no armazenamento do plugin, que é um arquivo só por
  nome de plugin (`~/.claude/plugins/store/tesseract-roster_*.json`),
  compartilhado por todas as sessões da máquina, de qualquer projeto e de
  qualquer cópia do mod. Uma chave por sessão, regravada a cada 30 s; entrada
  sem batida há 90 s vence e é apagada. Ao sair, a sessão apaga a própria
  chave.
- **Faixa acima do prompt**: `⏵ 2 APPROVE · ⬤ 1 RESPONDED · other sessions`.
  Conta só as OUTRAS sessões. Sem ninguém chamando, a faixa não existe.
  Abaixo de 48 colunas encurta para `⏵ 2 · ⬤ 1`. Não repete o `⧉` (é da faixa
  da célula) e não inverte nada: aprovação é o glifo `⏵` em âmbar e negrito
  (sem cor, glifo e negrito). O badge invertido da tela é do mosaico.
- **`/find <texto>`**: busca nas mensagens desta sessão (suas e do Claude),
  sem diferenciar maiúsculas. Resultado em painel; Enter numa linha abre a
  mensagem inteira no lugar da lista, `b` ou Enter volta.
- **Dicas**: a linha de dica do prompt ganha os comandos Tesseract que estão
  carregados (`/dlog /find`), acrescentados ao que já havia, e
  **`/ts-help`** descreve cada um, dobrando na largura do terminal. `/dlog`
  vem do `tesseract-docker`; os painéis `/docker`, `/md` e `/open` deixaram de
  existir (são do mosaico: `d`, a aba md e `ctrl+e`).

## De onde vem o estado

Dos eventos da própria sessão, sem raspar tela: começo de turno, fim de
turno, pedido de permissão e pergunta ao usuário. "Esperando aprovação" volta
a "trabalhando" quando a ferramenta que esperava termina. "Respondeu" volta a
"ocioso" quando a pessoa retoma a sessão (digita ou envia no prompt) ou
quando começa um turno novo: sessão já atendida não fica chamando.

## Limites

- Enxerga toda sessão da máquina que carrega um plugin com este nome, venha
  de que pasta vier: o arquivo do armazenamento é por nome de plugin.
- Duas sessões gravando no mesmo instante podem perder uma escrita; a batida
  de 30 s regrava.
- Ler a resposta sem tocar no teclado não conta como atender: o "respondeu"
  só sai quando algo é digitado ou enviado no prompt.
- A busca cobre as 4.096 mensagens mais novas, guarda 60 linhas e não rola a
  conversa até o ponto.
- Com ferramentas em paralelo, uma terminar tira o `approve` da que ainda
  espera, até o próximo pedido.
- A dica do prompt vai no `tail` da linha. Um mod que reescreve a linha
  inteira precisa carregar o `tail` junto (o `tesseract-skin` faz isso).
- No `/find`, tecla sem ação: o host não deixa um painel consumir teclas, e
  uma tecla que nenhum botão tem devolve o teclado ao prompt e é escrita lá.
  Com o foco numa linha ou na mensagem aberta, toda letra e dígito sem uso
  tem um botão invisível e inerte; pontuação e espaço ainda vazam.
- Desenhado pelo host, fora da identidade Tesseract: o anel de foco invertido
  do botão, a letra da tecla (`b:`) em azul ANSI 12 e a moldura do painel na
  cor padrão do terminal.

## Arquivos

- `hooks/register.tsx`: eventos de estado, faixa, dica, `/find`, `/ts-help`.
- `hooks/roster.ts`: estado da sessão, expiração, contagem das outras, faixa, busca.
- `hooks/list.ts`: janela da lista que rola com o foco e contas de largura.
- `hooks/tokens.ts`: cores e glifos da identidade Tesseract; respeita `NO_COLOR`.
- `tests/`: `claude plugin test .`

## Rodar

```sh
claude --plugin-dir mods/tesseract-roster
claude plugin validate mods/tesseract-roster --strict
claude plugin test mods/tesseract-roster
```
