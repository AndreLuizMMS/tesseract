# Prompt — Tesseract Neon: colorscheme + superfícies

Cole o bloco abaixo inteiro num agente (Claude Code, Cursor, etc.) na raiz do repositório do Tesseract. Ele é autossuficiente: carrega a paleta completa, não depende de nenhum outro arquivo.

Antes de colar, ajuste as duas linhas marcadas com `<<<` na seção CONTEXTO.

---

````
Você é engenheiro de design systems especializado em ferramentas de terminal.
Gere os artefatos de tema do projeto Tesseract a partir da especificação abaixo.
Não invente cores. Não altere hex. Não adicione cores novas.

## CONTEXTO

Tesseract é um painel de terminal onde vários agentes de IA rodam lado a lado
numa grade. Comando: `ts`. Open source, MIT. Documentação em português.
Linguagem/stack do projeto: <<< PREENCHA (ex: Go + Bubble Tea / Rust + Ratatui)
Diretório de saída: <<< PREENCHA (ex: ./themes)

## PALETA CANÔNICA — fonte única de verdade

### Base
bg.void          #030507   fundo do modo DIGITAR
bg.base          #070B0C   fundo padrão
bg.surface       #0C1315   corpo da célula
bg.raised        #121C1F   cabeçalho, seleção
line.dim         #16282A   grade não focada
line.active      #205047   grade do projeto focado
fg.faint         #3E534E   desligado, atalhos
fg.muted         #6C8076   texto secundário
fg.default       #BFD1C6   texto principal
fg.bright        #E8F4EC   títulos

### Neon — verde é POSSE DO TECLADO
brand.deep       #0B3322
brand.core       #1F7A4C   grade do projeto ativo, logo
brand.live       #35C27A   célula focada
brand.phosphor   #55FFA6   dono do teclado — 1 por tela, sempre

### Neon — ciano é ESTRUTURA
flux.deep        #082F31
flux.core        #128C86   grade, cantos, labels
flux             #22E0D0   glifo, numeração, segunda dimensão

### Estados — nenhum verde, nenhum ciano
state.working    #6C8076   ▸ trabalhando
state.read       #7DB7E8   ⬤ respondeu
state.block      #FFB454   ⏵ aprovar
state.dead       #FF3B47   ✖ caiu
state.off        #3E534E   ○ parada
state.orphan     #C77DFF   ⚠ órfã

### ANSI 16
bg #070B0C · fg #BFD1C6 · cursor #55FFA6 · cursor_text #030507
selection_bg #121C1F · selection_fg #E8F4EC
0  #070B0C   1  #C22F38   2  #1F7A4C   3  #C9A227
4  #3E7FA8   5  #8B4FC4   6  #128C86   7  #BFD1C6
8  #3E534E   9  #FF3B47   10 #55FFA6   11 #FFB454
12 #7DB7E8   13 #C77DFF   14 #22E0D0   15 #E8F4EC

## REGRAS INVIOLÁVEIS

1. Verde nunca é cor de estado. Verde significa apenas "seu teclado está aqui".
2. Ciano nunca é cor de estado. Ciano significa apenas estrutura/grade.
3. #55FFA6 aparece no máximo uma vez por tela.
4. Sem glow, sem scanline, sem aberração cromática dentro do terminal —
   esses efeitos existem só na superfície de marca (README, site, banner).
5. Urgência é área preenchida, não matiz: `aprovar` sempre renderiza como
   barra sólida invertida ocupando a linha inteira; `respondeu` é só um glifo.
6. Tudo deve continuar distinguível com NO_COLOR=1 e em terminal de 16 cores.
7. Nenhuma ligadura de fonte. Nenhum emoji. Nenhum canto arredondado.

## SÍMBOLO — versão em caractere (7×5), canônica

```
┌────┐
│┌───┼┐
││ ▓ ││
└┼───┘│
 └────┘
```

Colorização: quadrado de trás (interno, deslocado) em `flux` #22E0D0,
quadrado da frente em `brand.core` #1F7A4C, `▓` em `brand.phosphor` #55FFA6.
Glifo de 1 caractere: ⧉ (U+29C9).

## ENTREGÁVEIS

Gere cada arquivo abaixo, completo e válido. Um arquivo por bloco, com o
caminho no topo. Não resuma, não escreva "e assim por diante".

### A — Colorschemes de terminal
1.  `windows-terminal.json`   — fragmento do array `schemes`, nome "Tesseract Neon"
2.  `wezterm.toml`
3.  `alacritty.toml`
4.  `kitty.conf`
5.  `ghostty`                 — arquivo de config de tema
6.  `tesseract-neon.yaml`     — esquema base16/base24 (tinted-theming),
                                mapeando base00–base0F a partir da paleta acima.
                                Explique em comentário a escolha de cada base.

### B — Ferramentas do dia a dia
7.  `tmux.conf`               — status bar: verde só na janela ativa, ciano na estrutura
8.  `starship.toml`           — prompt com `⧉`, verde só no símbolo de posse
9.  `fzf.env`                 — variável FZF_DEFAULT_OPTS com as cores
10. `bat.tmTheme`             — tema para bat/delta
11. `delta.gitconfig`         — seção [delta] do git
12. `nvim/tesseract.lua`      — colorscheme Neovim mínimo mas completo:
                                Normal, Comment, String, Function, Keyword,
                                Type, Constant, DiagnosticError/Warn/Info/Hint,
                                CursorLine, Visual, StatusLine, WinSeparator,
                                DiffAdd/Change/Delete, Search, Pmenu
13. `eza-ls-colors.sh`        — LS_COLORS/EZA_COLORS

### C — Tema do próprio Tesseract
14. Arquivo de tema na stack do projeto, com os 20 tokens como constantes
    nomeadas (nunca hex solto no código de UI), mais:
    - mapeamento estado → cor + glifo + estilo (normal / invertido)
    - dois modos: NAVEGAR (borda simples) e DIGITAR (borda dupla + selo
      `▓ DIGITAR ▓` invertido)
    - fallback para 16 cores e para NO_COLOR=1

### D — README
15. `README.md` completo, em português, com:
    - Banner ASCII no topo, dentro de bloco de código:
      ```
         ┌────┐
         │┌───┼┐    T E S S E R A C T
         ││ ▓ ││    o mosaico não desmonta
         └┼───┘│
          └────┘    ts 0.1.0 // MIT
      ```
    - Badges gerados em shields.io com `style=flat-square`,
      `color=55FFA6` e `labelColor=070B0C`: licença, versão, plataforma
    - Tabela do alfabeto de estados (sinal, estado, o que fazer)
    - Seção "Os dois modos" explicando NAVEGAR × DIGITAR
    - Instalação, atalhos, configuração de tema
    - Regra: o banner precisa ler no tema CLARO do GitHub também —
      não use nada que dependa de cor.
16. `logo.svg` — dois quadrados 100×100 traço 8, o de trás deslocado +24/+24
    em `#22E0D0`, o da frente em `#55FFA6`, tessera 18×18 em `#E8F4EC`.
    Sem gradiente, sem sombra, sem raio de borda. viewBox "0 0 136 136".
17. `logo-mono.svg` — mesma geometria, traço 11, cor única `currentColor`,
    SEM a tessera (versão favicon/16px).

### E — Verificação
18. `scripts/check-theme.sh` — script que:
    - falha se algum hex verde ou ciano da paleta aparecer no mapa de estados
    - falha se houver hex hardcoded fora do arquivo de tema
    - imprime todas as cores em blocos ANSI para inspeção visual

## FORMATO DA RESPOSTA

Um arquivo por vez, na ordem 1→18, cada um em bloco de código com o caminho
completo na primeira linha como comentário. Sem preâmbulo, sem resumo final.
Se algum formato exigir uma decisão não coberta pela paleta, escolha a opção
mais conservadora e registre a decisão em um comentário no próprio arquivo.
````

---

## Como usar

| Situação | O que fazer |
|---|---|
| Quer tudo de uma vez | Cole o bloco inteiro |
| Só o colorscheme de terminal | Apague as seções B, C, D, E dos ENTREGÁVEIS |
| Quer distribuir publicamente | Peça o item 6 (base16) primeiro — de um YAML saem ~50 aplicações via `tinted-builder` |
| Vai rodar em contexto pequeno | Divida por seção: A, depois B, depois C, depois D |
