# Tema e marca

A cor aqui não é enfeite, é gramática. Ela tem três leis, e nenhuma delas é estética:

- **Verde é posse do teclado.** Nunca é estado. O verde phosphor aparece no máximo uma vez
  por tela: na célula que está com o seu teclado, e em mais nada.
- **Ciano é estrutura.** Grade, cantos, numeração, rótulos. Nunca é estado.
- **Estado não usa verde nem ciano**, e urgência é área preenchida, não matiz.

Fora isso: sem brilho, sem scanline, sem ligadura, sem emoji, sem canto arredondado dentro
do terminal. Esses efeitos existem só na superfície de marca — este README, o site, o
banner.

A paleta inteira mora em **um arquivo só**, `internal/tema/tema.go`. Nenhum outro arquivo do
projeto escreve hex: quem desenha pede o token pelo nome (`tema.BrandPhosphor`,
`tema.FluxCore`, `tema.StateBlock`). O guarda dessa regra é executável:

```bash
./scripts/check-theme.sh
```

Ele imprime a paleta inteira em blocos ANSI para conferência a olho, e **falha** se alguém
usar verde ou ciano como cor de estado, ou escrever hex fora do arquivo de tema.

O tema tem três perfis e escolhe sozinho: cor cheia, 16 cores, ou nenhuma (`NO_COLOR=1` ou
`TERM=dumb`). Nos três o alfabeto de estados continua legível, porque o glifo e a forma
carregam o significado e a cor só reforça.

| Variável | O que faz |
|---|---|
| `NO_COLOR` | tira a cor inteira; sobra negrito, vídeo invertido e borda |
| `TESSERACT_SEM_PISCA` | para a piscada da barra de `aprovar`, mantendo a barra |
| `TESSERACT_SEM_ABERTURA` | pula a animação de partida e vai direto para a grade |

### Onde a marca aparece

O símbolo não vive só no README. Dentro do produto ele tem quatro casas:

| Lugar | Forma |
|---|---|
| Abertura | o símbolo se desenhando, traço a traço, na partida |
| Barra de título | glifo `⧉`, sempre à esquerda do nome |
| Grade vazia | símbolo 7×5 no centro, com a tecla que cria a primeira célula |
| Título da janela | `⧉ ts — projeto/célula`, acompanhando o foco |

### A abertura

A marca não aparece pronta na partida: **ela se monta**. O quadrado de trás nasce primeiro,
traço a traço, com a ponta da caneta acesa em fósforo correndo pelo caminho. Depois o
quadrado da frente por cima. A tessera acende, o símbolo inteiro estoura num quadro só e
assenta. Só então o nome abre, letra a letra, e as linhas do motor entram uma a uma.

```
   ┌────┐
   │┌───┼┐    T E S S E R A C T
   ││ ▓ ││    o mosaico não desmonta
   └┼───┘│
    └────┘    ts 0.1.0 // MIT

   > motor de sessão: vivo
   > 8 células recuperadas · 3 projetos · mesma posição
   > grade montada em 41ms
```

Dura cerca de um segundo e meio, e **roda em paralelo com a conexão** — enquanto a marca se
desenha, o motor é procurado e a grade é remontada na outra linha de execução. O cronômetro
de `grade montada em` conta só o motor, nunca a animação: é um número que existe para ser
verdade.

Nada disso é brilho, scanline ou glitch — o terminal não emite luz e a regra continua
valendo. O que se move é a **ordem em que as coisas passam a existir**, não a textura delas.
Sem terminal de verdade (saída num arquivo, num pipe), a abertura vira o bloco estático de
uma vez só.

### O mesmo tema no resto da mesa

A pasta `themes/` traz o **Tesseract Neon** pronto para o terminal e as ferramentas do dia:

| Arquivo | Para |
|---|---|
| `windows-terminal.json`, `wezterm.toml`, `alacritty.toml`, `kitty.conf`, `ghostty` | emuladores de terminal |
| `tesseract-neon.yaml` | esquema base16/base24 (tinted-theming) |
| `tmux.conf`, `starship.toml`, `fzf.env` | barra de status, prompt, busca |
| `bat.tmTheme`, `delta.gitconfig` | leitura de arquivo e diff |
| `nvim/tesseract.lua`, `eza-ls-colors.sh` | editor e listagem |
| `claude-code.md` | o Claude Code dentro da célula, sem cor própria |

**O agente dentro da célula.** O Claude Code pinta com paleta própria e briga com a grade —
laranja no selo, rosa no logo. A correção é uma linha: `/config` → tema → **`dark-ansi`**.
Nesse tema ele desenha só com as 16 cores ANSI, que são as do Tesseract Neon. Detalhes em
[`themes/claude-code.md`](../themes/claude-code.md).

A marca em vetor está em `themes/logo.svg` (colorida) e `themes/logo-mono.svg` (traço único
em `currentColor`, para favicon e 16px). Os detalhes de cada arquivo estão em
[`themes/README.md`](../themes/README.md).
