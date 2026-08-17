```
     ┌────┐
     │┌───┼┐    T E S S E R A C T
     ││ ▓ ││    o mosaico não desmonta
     └┼───┘│
      └────┘    ts 0.1.0 // MIT
```

[![licença](https://img.shields.io/badge/licen%C3%A7a-MIT-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6)](../LICENSE)
[![versão](https://img.shields.io/badge/vers%C3%A3o-0.1.0-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6)](https://github.com/andreluiz/tesseract/releases)
[![plataforma](https://img.shields.io/badge/plataforma-Linux%20%7C%20WSL%20%7C%20macOS-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6)](#instalação)

> O banner acima é feito só de traço e sombreado — nada nele depende de cor.
> Ele lê igual no tema escuro e no tema claro do GitHub, no `cat`, no `less` e
> num terminal com `NO_COLOR=1`. Essa é a regra: **se some quando a cor some,
> não entra**.

**Tesseract** é um painel de terminal onde vários agentes de IA rodam lado a
lado numa grade. Um comando, `ts`, e a tela inteira é o seu mosaico: cada
célula é um agente vivo, cada projeto é uma faixa, e o motor continua de pé
quando você fecha a tela.

---

## O alfabeto de estados

Toda célula está em exatamente um estado, e o estado tem sempre **três**
sinais: um glifo, uma cor e uma forma. Tire a cor e o glifo continua. Tire a
cor e o glifo e a forma continua — só o estado que **bloqueia** ocupa a linha
inteira.

| Sinal | Estado | O que fazer |
|---|---|---|
| `▸ TRABALHANDO` | processo vivo produzindo | nada — deixe trabalhar |
| `⬤ RESPONDEU` | devolveu a vez, tem texto esperando | leia quando puder; não trava nada |
| `⏵ APROVAR` | travou numa pergunta e **não anda** | responda: o trabalho está parado nisso |
| `✖ CAIU` | o processo morreu sozinho | `r` sobe de novo |
| `○ PARADA` | sem processo, célula preservada | `r` retoma de onde parou |
| `⚠ ÓRFÃ` | o diretório do projeto sumiu do disco | recrie o caminho ou mate a célula |

**Respondeu ≠ aprovar.** É a distinção que faz o alarme valer alguma coisa:
agente parado numa pergunta bloqueia o trabalho; agente que terminou o turno
apenas tem algo para ler.

Por isso `⏵ APROVAR` é a **única** linha que aparece como barra sólida
invertida, ocupando a largura toda: urgência aqui é área preenchida, não
matiz. Os outros cinco estados são um glifo e um rótulo.

E **nenhuma cor de estado é verde ou ciano**, nunca — os dois têm dono:

- **verde é posse do teclado.** O verde phosphor `#55FFA6` aparece no máximo
  uma vez por tela, na célula que está com o seu teclado. Em mais nada.
- **ciano é estrutura.** Grade, cantos, numeração, rótulos. Nunca um estado.

---

## Os dois modos

Nunca há dois donos do teclado ao mesmo tempo. É essa regra que torna colisão
de atalho estruturalmente impossível.

### NAVEGAR — o teclado é do aplicativo

O padrão. Toda tecla é comando: as setas andam pela grade, as letras agem
sobre a célula focada. A borda das células é **simples**, o fundo é o fundo
padrão, e a barra de baixo mostra os atalhos.

### DIGITAR — o teclado é da célula

`↵` entra. A partir daí **toda tecla vai para o agente, sem nenhuma exceção**
— nem `q`, nem `D`, nem `tab`, nem as setas. Só `ctrl-l` devolve o teclado ao
aplicativo.

O modo é impossível de errar, porque ele muda quatro coisas ao mesmo tempo:

1. o fundo da tela escurece;
2. a borda da célula focada vira **dupla**;
3. o selo `▓ DIGITAR ▓` aparece invertido;
4. a célula que tem o teclado fica **verde phosphor** — e é o único verde
   phosphor da tela.

Com `NO_COLOR=1` os itens 1 e 4 somem, e os itens 2 e 3 continuam: borda
dupla e selo invertido não dependem de cor nenhuma.

---

## Instalação

```sh
git clone https://github.com/andreluiz/tesseract
cd tesseract
./install.sh
```

Ou compilando direto:

```sh
go install github.com/andreluiz/tesseract/cmd/ts@latest
```

Depois, é um comando só:

```sh
ts
```

Requisitos: Go 1.25+ para compilar, um terminal com suporte a 256 cores para
a experiência completa, e nada além disso — o Tesseract lê e escreve na
própria pasta de configuração e não pede daemon de terceiros.

---

## Atalhos

**Andar — só setas, nenhuma letra**

| Tecla | Ação |
|---|---|
| `←` `→` | célula anterior / próxima, atravessando projeto |
| `↑` `↓` | projeto anterior / próximo |
| `espaço` | pula para a próxima célula que pede atenção |
| `1`…`9` | vai direto para o projeto N |
| `tab` | troca a aba da célula (`shift-tab` volta) |

**Teclado e tela**

| Tecla | Ação |
|---|---|
| `↵` | entra em DIGITAR na célula focada |
| `ctrl-l` | devolve o teclado ao aplicativo |
| `o` | célula focada em tela cheia |
| `v` | alterna mosaico ↔ lista |

**Criar, matar, nomear**

| Tecla | Ação |
|---|---|
| `n` | criar — pede o projeto, depois a célula |
| `r` | retoma célula parada, ou sobe célula caída |
| `D` | mata a célula focada — sempre confirma |
| `ctrl-r` | adota na célula o nome que o agente deu à conversa |

**Agir e ler**

| Tecla | Ação |
|---|---|
| `p` | manda prompt para a célula focada sem entrar nela |
| `d` | abre o painel Docker do projeto focado |
| `ctrl-e` | abre o diretório do projeto na IDE configurada |
| `/` | busca no histórico da célula focada |
| `esc` | sai da rolagem e fecha o que estiver aberto |
| `?` | ajuda |
| `q` | fecha a tela — o motor continua rodando |

---

## Configuração de tema

A paleta inteira mora em um arquivo só, `internal/tema/tema.go`. Nenhum outro
arquivo do projeto escreve hex — quem desenha pede o token pelo nome
(`tema.BrandPhosphor`, `tema.FluxCore`, `tema.StateBlock`). O script
`scripts/check-theme.sh` falha se alguém quebrar essa regra, ou se tentar usar
verde ou ciano como cor de estado.

### O terminal e as ferramentas

A pasta `themes/` traz o **Tesseract Neon** pronto para o resto da mesa:

| Arquivo | Para |
|---|---|
| `windows-terminal.json` | fragmento do array `schemes` |
| `wezterm.toml` | WezTerm |
| `alacritty.toml` | Alacritty |
| `kitty.conf` | kitty |
| `ghostty` | Ghostty |
| `tesseract-neon.yaml` | esquema base16/base24 (tinted-theming) |
| `tmux.conf` | barra de status |
| `starship.toml` | prompt |
| `fzf.env` | `FZF_DEFAULT_OPTS` |
| `bat.tmTheme` | bat e delta |
| `delta.gitconfig` | seção `[delta]` do git |
| `nvim/tesseract.lua` | colorscheme do Neovim |
| `eza-ls-colors.sh` | `LS_COLORS` e `EZA_COLORS` |

Cada um deles segue as mesmas regras do painel: verde só onde há posse, ciano
só onde há estrutura, estado em nenhum dos dois.

O tema de sintaxe (`bat.tmTheme`) serve o `bat` e o `delta` ao mesmo tempo:

```sh
mkdir -p "$(bat --config-dir)/themes"
cp themes/bat.tmTheme "$(bat --config-dir)/themes/Tesseract Neon.tmTheme"
bat cache --build
export BAT_THEME="Tesseract Neon"
```

### Terminal pobre e sem cor

O tema tem três perfis e escolhe sozinho:

| Perfil | Quando | O que muda |
|---|---|---|
| 24 bits | `COLORTERM=truecolor` ou `TERM=*256color*` | a paleta sai como está |
| 16 cores | qualquer outro `TERM` com cor | cada token cai no seu índice da ANSI 16 |
| sem cor | `NO_COLOR` definido, ou `TERM=dumb` | só negrito, reverso e borda |

Nos três, o alfabeto de estados continua legível — porque o glifo e a forma
carregam o significado sozinhos, e a cor só reforça.

### Verificando

```sh
./scripts/check-theme.sh
```

O script imprime a paleta inteira em blocos ANSI para inspeção a olho, e falha
se encontrar verde ou ciano no mapa de estados, ou hex escrito fora do arquivo
de tema.

---

## A marca

O símbolo é um tesserato achatado: dois quadrados, um atrás do outro,
deslocados — e uma tessera acesa no meio.

```
┌────┐
│┌───┼┐
││ ▓ ││
└┼───┘│
 └────┘
```

Na versão em caractere: o quadrado de trás é ciano (`#22E0D0`, a segunda
dimensão), o da frente é verde escuro (`#1F7A4C`), e a tessera é o phosphor
(`#55FFA6`). No vetor a ordem se inverte — o quadrado da frente carrega o
phosphor e a tessera é branca (`#E8F4EC`), porque no traço fino o verde escuro
some contra fundo escuro. Em um caractere só, o símbolo é `⧉` (U+29C9).

Versões vetoriais: `themes/logo.svg` (colorida) e `themes/logo-mono.svg`
(traço único em `currentColor`, para favicon e 16px).

Brilho, scanline e aberração cromática existem **só** na superfície de marca —
README, site, banner. Dentro do terminal, nunca.

---

## Licença

MIT. Veja [LICENSE](../LICENSE).
