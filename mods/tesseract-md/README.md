# tesseract-md

Mod do Claude Code: avisa quando o Claude escreve um markdown. Não tem
painel nem comando: quem lê é a aba `md` da célula, no mosaico do Tesseract.

## O que faz

- **Dentro de uma célula do Tesseract**: cada `.md` ou `.markdown` que o
  Claude escreve ou edita vira um relato ao engine. Com a célula em foco, o
  rodapé do mosaico termina em `m reads plan.md`; `m` troca a célula para a
  aba `md` já naquele arquivo, e o aviso some depois de lido.
- **Fora de célula** (`claude` solto): não há engine nem leitor. Um aviso
  curto diz o caminho (`Wrote docs/plan.md.`), uma vez por arquivo.
- **Sessão de fundo** do Claude Code dentro de uma célula: muda. Ela herda o
  ambiente da célula, mas não é o que a célula mostra.

O mod só observa: a escrita roda primeiro, e o relato nunca a barra nem a
atrasa além de 3 s. Edição negada ou que falhou não avisa.

## Como fala com o engine

Como o `tesseract-link`: roda `ts report '{"kind":"doc","file":"<caminho>"}'`,
um processo curto por evento. O binário vem de `TESSERACT_BIN` e a célula de
`TESSERACT_CELL_ID`, que o engine põe no ambiente do processo; o relato nunca
diz de qual célula é. O engine confere o caminho antes de usar: absoluto,
sem caractere de controle, arquivo que existe.

O leitor que substituiu o painel deste mod é o do mosaico
(`internal/cell/md.go`): lista com busca, leitura desenhada como página,
recarga quando o arquivo muda.

## Arquivos

- `hooks/register.ts`: o mod inteiro.
- `tests/`: `claude plugin test .`

## Rodar

```sh
claude --plugin-dir mods/tesseract-md
claude plugin validate mods/tesseract-md --strict
claude plugin test mods/tesseract-md
```
