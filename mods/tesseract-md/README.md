# tesseract-md

Mod do Claude Code: o leitor de markdown do projeto dentro da sessão. Feito
para rodar em célula do Tesseract (80 colunas, sem mouse), mas funciona em
qualquer `claude`.

## O que faz

- **`/md`** abre a lista dos `.md` e `.markdown` do projeto, mais raso
  primeiro, com um filtro por nome ou caminho.
- **Leitura** do arquivo escolhido, desenhada como uma resposta do Claude.
  Relê sozinha quando o arquivo muda no disco (um `stat` a cada 1,5 s, só com
  o painel aberto).
- **Links entre docs**: os que levam a outro markdown do projeto viram as
  teclas `1` a `9`; `b` volta ao doc anterior e, no fim, à lista. Link para
  arquivo que não existe aparece como `nome.md (broken)` e link que sobe além
  da raiz como `nome.md (outside project)`, os dois sem tecla.
- **`/md last`** abre o último markdown que o Claude escreveu ou editou nesta
  sessão. Quando ele escreve um, aparece um aviso curto; o painel nunca abre
  sozinho.
- **`/md <texto>`** abre a lista já filtrada, ou o arquivo, se o texto for o
  caminho exato.

## Teclas

| Onde | Tecla | Efeito |
|---|---|---|
| lista | digitar | filtra (com o foco numa linha, a letra vai para o filtro) |
| lista | `↑` `↓` / `Tab` | escolhe o arquivo; a lista rola com o foco |
| lista | `PgUp` `PgDn` | anda uma janela |
| lista | `Enter` | lê (no filtro: abre o primeiro da lista) |
| leitura | `↑` `↓` | rola uma linha |
| leitura | `PgUp` `PgDn` | rola uma tela |
| leitura | `1`–`9` | segue o link |
| leitura | `b` / `Enter` | volta |
| qualquer | `Esc` | fecha |

Ao abrir um doc o foco fica em `list`/`back`, nunca num link. Voltar à lista
limpa o filtro: a lista volta inteira.

Tecla sem ação: o host não deixa um painel consumir teclas, só dar tecla a
um botão, e uma tecla que nenhum botão tem devolve o teclado ao prompt e é
escrita lá. Por isso cada tela do painel tem um botão invisível e inerte para
toda letra e dígito sem uso (na lista, com o foco numa linha, a letra vai para o filtro). Pontuação e espaço não podem ser tecla de
botão: esses ainda vazam para o prompt.

## Célula baixa ou estreita

O painel desenha só o que cabe nas linhas que a célula dá (numa célula de 16
linhas sobram 3 de corpo). Na lista: o filtro, os arquivos que couberem e uma
linha de dicas com a posição (`3/8`). Na leitura o texto rola por baixo de
uma linha de teclas fixa; o cabeçalho com o caminho entra a partir de 6
linhas de corpo.

## Limites

- Varre até 6 níveis, 2.000 arquivos e 1.500 pastas; ignora `node_modules`,
  `.git`, `vendor`, `dist`, `build` e pastas ocultas (menos `.github`).
- `.md` que é link simbólico não entra na lista (a varredura só lista
  arquivos de verdade). Abre por link de outro doc ou por `/md last`.
- Arquivo acima de 4 MiB não abre; texto acima de 90 mil caracteres é cortado.
- Só 9 links por doc ganham tecla.
- O fim da rolagem é estimado pelo tamanho do texto: em doc com muita tabela
  dá para rolar algumas linhas em branco além do fim.
- Desenhado pelo host, fora da identidade Tesseract: o anel de foco invertido
  do botão, a letra da tecla (`b:`) e os links do markdown em azul ANSI 12, e
  a moldura do painel na cor padrão do terminal.

## Arquivos

- `hooks/register.tsx`: comando, painel, polling, aviso de arquivo escrito.
- `hooks/docs.ts`: quais arquivos entram, ordem, filtro, links e limite da rolagem.
- `hooks/list.ts`: janela da lista que rola com o foco e contas de largura.
- `hooks/tokens.ts`: cores e glifos da identidade Tesseract; respeita `NO_COLOR`.
- `tests/`: `claude plugin test .`

## Rodar

```sh
claude --plugin-dir mods/tesseract-md
claude plugin validate mods/tesseract-md --strict
claude plugin test mods/tesseract-md
```
