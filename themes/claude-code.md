# Tema do Claude Code dentro do Tesseract

## O problema

O Claude Code pinta com uma paleta própria em 24 bits — laranja no selo, rosa
no logo, azul e magenta na barra de contexto. Nada disso conhece o Tesseract
Neon, então dentro da célula ele briga com a grade: cores que o painel reservou
para significado (laranja é "aprovar", verde é "seu teclado está aqui")
aparecem no meio da conversa sem querer dizer nada.

O Tesseract não tem como censurar a cor do agente — a célula é um terminal de
verdade, e o que o agente escreve nela é dele. Quem resolve é o próprio Claude
Code, e ele já sabe fazer isso.

## A correção

O Claude Code tem um tema que **não usa cor própria nenhuma**: ele desenha só
com as 16 cores ANSI do terminal. Como o Tesseract Neon define exatamente essas
16 cores, o Claude Code passa a falar a mesma língua da grade.

```
tema do Claude Code:  dark-ansi
```

Para trocar, dentro de qualquer sessão do Claude Code:

```
/config
```

e escolha **dark-ansi** no campo de tema. Vale para todas as sessões, inclusive
as que já estão de pé nas células — o Claude Code relê o tema na hora.

> Não edite `~/.claude.json` na mão para isso. Toda sessão viva reescreve esse
> arquivo ao sair, e a sua edição some junto. O `/config` grava pelo caminho
> certo.

## O que cada cor vira

Com `dark-ansi`, o Claude Code passa a pedir cor pelo índice, e o índice é
resolvido pela paleta do seu emulador de terminal. Instale um dos arquivos de
`themes/` (o `windows-terminal.json`, o `kitty.conf`, o que for o seu) e o
resultado é este:

| O que o Claude Code pinta | Índice ANSI | Cor no Tesseract Neon |
|---|---|---|
| texto normal | 7 | `#BFD1C6` fg.default |
| texto apagado, dicas | 8 | `#3E534E` fg.faint |
| erro, diff removido | 1 / 9 | `#C22F38` / `#FF3B47` state.dead |
| aviso, permissão pendente | 3 / 11 | `#C9A227` / `#FFB454` state.block |
| sucesso, diff adicionado | 2 / 10 | `#1F7A4C` / `#55FFA6` brand |
| link, referência de arquivo | 4 / 12 | `#3E7FA8` / `#7DB7E8` state.read |
| destaque, comando | 6 / 14 | `#128C86` / `#22E0D0` flux |
| título, ênfase | 15 | `#E8F4EC` fg.bright |

## Por que não um tema com hex próprio

O Claude Code não aceita paleta arbitrária: o tema é uma escolha entre um
punhado de opções prontas. `dark-ansi` é a única que devolve o controle da cor
para o terminal — e devolver o controle ao terminal é exatamente o que a gente
quer, porque o terminal aqui é o Tesseract.

Fica melhor do que um tema com hex fixo, aliás: quando você trocar a paleta do
Tesseract, o Claude Code troca junto, sem ninguém reconfigurar nada.

## Os dois passos, juntos

```sh
# 1. o emulador de terminal fala Tesseract Neon
#    (escolha o arquivo do seu terminal em themes/)

# 2. o Claude Code para de ter cor própria
/config   →   tema   →   dark-ansi
```

O `⏵ APROVAR` da grade continua sendo o único laranja com significado, e o
verde phosphor continua aparecendo uma vez por tela.
