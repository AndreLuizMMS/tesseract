# tesseract-ide

Mod do Claude Code: conta ao engine do Tesseract onde o Claude mexeu por
último. Não tem seletor, não detecta nem lança IDE: isso é do mosaico
(`ctrl+e`).

## O que faz

- **Dentro de uma célula do Tesseract**: depois de cada `Edit` ou `Write`
  que deu certo, relata o arquivo e a linha onde a mudança começa. No seletor
  de IDE do mosaico (`ctrl+e`) aparece uma linha própria, só quando há
  arquivo: `f  opens internal/cell/md.go:42, the agent's last edit`. `f` abre
  esse arquivo, na linha, no IDE escolhido; `↵` continua abrindo o projeto.
- **Fora de célula** e em **sessão de fundo**: nada.

A linha de um `Edit` é onde o trecho novo começa no arquivo já gravado (as
três primeiras linhas do trecho bastam para achar); de um `Write`, a 1.

## Como fala com o engine

Como o `tesseract-link`: roda
`ts report '{"kind":"edit","file":"<caminho>","line":42}'`, um processo curto
por evento. O binário vem de `TESSERACT_BIN` e a célula de
`TESSERACT_CELL_ID`, postos pelo engine no ambiente. O engine confere o
caminho antes de usar (absoluto, sem caractere de controle, arquivo que
existe): ele termina na linha de comando do IDE.

## O que foi para o mosaico

A detecção (VS Code, Insiders, VSCodium e Cursor, no WSL e do lado Windows),
o seletor e o lançamento ficam em `internal/engine/actions.go` e
`internal/screen/ide.go`. Do que este mod fazia melhor, o Go herdou:

- **Lançamento que não prende**: o IDE nasce em sessão própria, com a saída
  num arquivo em vez de um pipe do engine; meio segundo separa "abriu" de
  "morreu na hora", e só a segunda vira erro no rodapé.
- **Uma instalação, uma linha**: o `code` que a instalação Windows põe no
  PATH do WSL (`/mnt/c/.../bin/code`) é a mesma instalação e aparece uma vez,
  como `windows`.

O IDE "lembrado" entre sessões deixou de existir: o seletor do mosaico
sempre mostra a lista.

## Arquivos

- `hooks/register.ts`: o mod inteiro.
- `tests/`: `claude plugin test .`

## Rodar

```sh
claude --plugin-dir mods/tesseract-ide
claude plugin validate mods/tesseract-ide --strict
claude plugin test mods/tesseract-ide
```
