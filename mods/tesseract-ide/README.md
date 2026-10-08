# tesseract-ide

Mod do Claude Code: abre o projeto no IDE de dentro da sessão. Feito para
rodar em célula do Tesseract (80 colunas, sem mouse), mas funciona em
qualquer `claude`.

O comando é **`/open`**: `/ide` já é um comando do próprio Claude Code e o
registro com esse nome é recusado.

## O que faz

- **`/open`** detecta os IDEs e abre o seletor; Enter abre o projeto.
- **`/open file`** abre o último arquivo que o Claude editou, na linha da
  edição, no IDE usado da última vez. Sem IDE lembrado, cai no seletor, onde
  `f` troca o alvo para o arquivo.
- **IDE lembrado**: fica no armazenamento do plugin, que é um arquivo por
  nome de plugin (`~/.claude/plugins/store/tesseract-ide_*.json`). Vale para
  todos os projetos e todas as cópias do mod: escolher o Cursor num projeto
  muda o `/open file` dos outros.
- **Detecção** (porte de `internal/engine/actions.go`): VS Code, VS Code
  Insiders, VSCodium e Cursor; no PATH do Linux e, dentro do WSL, do lado
  Windows mesmo fora do PATH (procura o `.exe` em
  `Users/*/AppData/Local/Programs` e nos dois `Program Files`; o PATH do
  Windows, via `cmd.exe`, é só reserva). O `code` que a instalação Windows
  põe no PATH do WSL (`/mnt/c/.../bin/code`) é a mesma instalação: aparece
  uma vez, como `windows`.

## Como o IDE é lançado sem prender a sessão

Um processo iniciado pelo mod é esperado até fechar a saída: um IDE que fica
vivo seguraria a chamada até o tempo limite. O lançamento passa por um `sh`
mínimo que põe o IDE em sessão própria (`setsid`), com a saída fora dos pipes
da chamada, espera meio segundo e sai:

- IDE vivo depois de 0,5 s: abriu. A chamada volta e o IDE segue, inclusive
  depois de a sessão do Claude fechar.
- IDE morreu antes: não abriu. A primeira linha do que ele disse vai ao painel.

Medido: a chamada volta em cerca de 510 ms.

Caminhos, na ordem: janela do IDE Windows já aberta (CLI remoto mais o socket
que ainda responde); o `.exe` achado no disco, rodado direto pelo WSL;
`cmd.exe /c start` quando só o PATH do Windows conhece o nome.

## Teclas do seletor

| Tecla | Efeito |
|---|---|
| `↑` `↓` / `Tab` | escolhe o IDE; a lista rola com o foco |
| `Enter` | abre o alvo no IDE escolhido |
| `p` / `f` | alvo: projeto / último arquivo editado |
| `Esc` | fecha |

O alvo ativo aparece entre colchetes na linha de teclas
(`Enter opens p: [project]  f: main.go:42`); o foco não passa por `p` e `f`.
O painel desenha só o que cabe: em célula de 16 linhas são 2 IDEs e a linha
de teclas, e o cabeçalho entra quando sobra linha.

Tecla sem ação: o host não deixa um painel consumir teclas, só dar tecla a
um botão, e uma tecla que nenhum botão tem devolve o teclado ao prompt e é
escrita lá. Por isso cada tela do painel tem um botão invisível e inerte para
toda letra e dígito sem uso. Pontuação e espaço não podem ser tecla de
botão: esses ainda vazam para o prompt.

Desenhado pelo host, fora da identidade Tesseract: o anel de foco invertido
do botão, a letra da tecla (`p:`) em azul ANSI 12 e a moldura do painel na
cor padrão do terminal.

## Arquivos

- `hooks/register.tsx`: comando, seletor, sondagem da máquina, lançamento.
- `hooks/ides.ts`: candidatos, detecção, comando por caminho, script destacado.
- `hooks/list.ts`: janela da lista que rola com o foco e contas de largura.
- `hooks/tokens.ts`: cores e glifos da identidade Tesseract; respeita `NO_COLOR`.
- `tests/`: `claude plugin test .`

## Rodar

```sh
claude --plugin-dir mods/tesseract-ide
claude plugin validate mods/tesseract-ide --strict
claude plugin test mods/tesseract-ide
```
