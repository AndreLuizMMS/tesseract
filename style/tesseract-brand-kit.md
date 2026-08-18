# Tesseract — Brand Kit

**Versão 2.0 · NEON · identidade para software de terminal**

Marca: `Tesseract` · Comando: `ts` · Glifo: `⧉` · Tagline: *o mosaico não desmonta*

---

## 1. As duas regras fundadoras

O símbolo é **dois quadrados**. Por isso a marca tem **duas cores**, e cada uma carrega um significado fixo.

> **Verde é posse.** Verde nunca é cor de estado. Verde significa "seu teclado está aqui". Existe um único fósforo aceso na tela inteira, em uma célula por vez.

> **Ciano é estrutura.** Ciano é a segunda dimensão — o quadrado de trás, a grade, o glifo, a numeração. Também nunca é estado.

Consequência: o alfabeto de estados (▸ ⬤ ⏵ ✖ ○ ⚠) não usa verde nem ciano. Nenhum tom, em lugar nenhum.

**O neon só funciona porque quase tudo está apagado.** Escuridão é o material principal; o brilho é a exceção. Se mais de 5% da tela estiver acesa, a identidade quebrou.

---

## 2. Onde o cyberpunk pode aparecer — e onde não

| Efeito | Superfície de marca<br><small>site, README, banner, docs, social</small> | Produto<br><small>dentro do terminal</small> |
|---|---|---|
| Duotone verde + ciano | sim | sim |
| Glow / bloom | sim | **não** — terminal não emite luz |
| Scanline, vinheta CRT | sim | **não** |
| Aberração cromática | só no wordmark | **não** |
| Glitch / jitter | só em hover do símbolo | **não** |
| Chuva de caracteres, katakana | **não** | **não** |
| Rosa neon como terceira cor | **não** | **não** — vira arcade |

Motivo único para todos os "não" do produto: o Tesseract promete **denso, legível e confiável**. Glow em texto de UI destrói densidade legível. O neon é a imagem da marca, não a textura do trabalho.

---

## 3. Essência

| Campo | Definição |
|---|---|
| Essência | Um mosaico de agentes que não desmonta |
| Promessa | Nada esconde atrás de aba, nada se perde quando a janela fecha |
| Arquétipo | Governante (ordem, controle) + Criador (instrumento de ofício) |
| Personalidade | Denso · Disciplinado · Permanente · Alerta sem gritar |
| Metáfora | Torre de comando feita de pedras de mosaico |

**Tagline:** `O mosaico não desmonta.`
**One-liner técnico:** `Vários agentes. Uma grade. Um teclado por vez.`
**Descrição:** `Painel de terminal para rodar agentes de IA lado a lado, com sessões que sobrevivem à máquina.`

---

## 4. Símbolo

Dois quadrados iguais deslocados na diagonal (projeção do hipercubo) + uma tessera acesa no centro da sobreposição.

| Regra | Valor |
|---|---|
| Quadrado da frente | 100 × 100, traço 8, cor `brand.phosphor` |
| Quadrado de trás | idêntico, deslocado +24 em X e +24 em Y, cor `flux` |
| Tessera | 18 × 18 sólido, `fg.bright`, no centro da sobreposição |
| Proibido | raio de borda, gradiente, sombra, preenchimento nos quadrados |
| Respiro | metade da largura do quadrado externo, todos os lados |

### Versão em caractere — canônica, 7×5

Esta é a versão que vive dentro do produto.

```
┌────┐
│┌───┼┐
││ ▓ ││
└┼───┘│
 └────┘
```

Colorização em terminal: quadrado de trás em `color14` (ciano), da frente em `color2` (verde), `▓` em `color10` (fósforo).

### Glifo único

`⧉` (U+29C9) — prompt, título de janela, badge, qualquer lugar com um caractere.

### Degradação

| Tamanho | O que mostrar |
|---|---|
| ≥ 48px | Símbolo completo, duotone, tessera |
| 24–47px | Completo, traço proporcionalmente mais grosso |
| 16px (favicon) | Só os dois quadrados, monocromático, **sem** tessera |
| 1 caractere | `⧉` |
| Dentro do app | Versão 7×5 em box-drawing |

---

## 5. Paleta v2.0

### Escuridão

| Token | Hex | Uso |
|---|---|---|
| `bg.void` | `#030507` | Fundo do modo DIGITAR |
| `bg.base` | `#070B0C` | Fundo padrão |
| `bg.surface` | `#0C1315` | Corpo da célula |
| `bg.raised` | `#121C1F` | Cabeçalho, seleção, painel Docker |
| `line.dim` | `#16282A` | Grade não focada |
| `line.active` | `#205047` | Grade do projeto focado |

### Texto

| Token | Hex | Uso |
|---|---|---|
| `fg.faint` | `#3E534E` | Desligado, ajuda, atalhos |
| `fg.muted` | `#6C8076` | Texto secundário |
| `fg.default` | `#BFD1C6` | Texto principal |
| `fg.bright` | `#E8F4EC` | Títulos, tessera |

### Neon — verde é posse

| Token | Hex | Uso | Frequência |
|---|---|---|---|
| `brand.deep` | `#0B3322` | Fundo de selos | livre |
| `brand.core` | `#1F7A4C` | Grade do projeto ativo, logo | livre |
| `brand.live` | `#35C27A` | Célula focada em NAVEGAR | 1 por tela |
| `brand.phosphor` | `#55FFA6` | **Dono do teclado.** Cursor, borda DIGITAR, selo, tessera | **1 por tela** |

### Neon — ciano é estrutura

| Token | Hex | Uso |
|---|---|---|
| `flux.deep` | `#082F31` | Fundos de rótulo |
| `flux.core` | `#128C86` | Grade, cantos, labels |
| `flux` | `#22E0D0` | Glifo `⧉`, numeração, quadrado de trás |

### Estados — nenhum verde, nenhum ciano

| Sinal | Estado | Token | Hex |
|---|---|---|---|
| `▸` | trabalhando | `state.working` | `#6C8076` |
| `⬤` | respondeu | `state.read` | `#7DB7E8` |
| `⏵` | aprovar | `state.block` | `#FFB454` |
| `✖` | caiu | `state.dead` | `#FF3B47` |
| `○` | parada | `state.off` | `#3E534E` |
| `⚠` | órfã | `state.orphan` | `#C77DFF` |

---

## 6. Respondeu ≠ Aprovar

O coração do produto. A diferença **não pode depender de cor nem de brilho**.

**Regra: urgência é área preenchida, não matiz.**

| | Respondeu | Aprovar |
|---|---|---|
| Forma | ponto `⬤` | triângulo `⏵` |
| Área | glifo pequeno | **barra sólida na linha inteira** |
| Vídeo | normal | **invertido** |
| Movimento | estático | pisca a cada 2s |
| Cor | azul-gelo | âmbar |

Teste: com `NO_COLOR=1`, a linha de `aprovar` continua sendo a única barra sólida da tela.

---

## 7. Os dois modos

### NAVEGAR

- Fundo `bg.base`, células em `bg.surface`
- Grade em traço simples `┌ ─ ┐ │ └ ┘`, em `flux.core`
- Projeto focado em `line.active`, os outros em `line.dim`
- Célula selecionada com borda `brand.live`
- Todos os estados visíveis

### DIGITAR

- Fundo cai para `bg.void`; tudo que não é a célula ativa vai para `fg.faint`
- Célula ativa mantém `bg.surface` e ganha **borda dupla** `╔ ═ ╗ ║ ╚ ╝` em `brand.phosphor`
- Selo invertido no canto superior direito: `▓ DIGITAR ▓` (fundo fósforo, texto `bg.void`)
- Cursor em bloco, `brand.phosphor`

**Regra:** traço simples = você comanda o app. Traço duplo = a célula comanda o teclado. Funciona sem cor.

---

## 8. Tipografia

| Papel | Fonte | Nota |
|---|---|---|
| Display / wordmark | **Martian Mono** 800 | Só caixa alta, tracking +.24em, com aberração cromática de 2px |
| Produto | **Iosevka Term** | Estreita = mais colunas por célula |
| Alternativa livre | **JetBrains Mono** | Se Iosevka for densa demais |
| Alternativa paga | **Berkeley Mono** | Caráter, se quiser pagar |

**Não use:** Fira Code, Cascadia, Inter, nada com ligadura de seta — ligadura embaralha alinhamento de grade.

Hierarquia dentro do app:

| Nível | Estilo |
|---|---|
| Nome do projeto | Caixa alta, `fg.bright`, tracking +1 |
| Nome da célula | `fg.default` |
| Estado / metadado | `fg.muted` |
| Ajuda / atalho | `fg.faint` |

---

## 9. Aplicações

### Banner de boot

```
   ┌────┐
   │┌───┼┐    T E S S E R A C T
   ││ ▓ ││    o mosaico não desmonta
   └┼───┘│
    └────┘    ts 0.1.0 // MIT

   8 células recuperadas // 3 projetos // 41ms
```

### Outras superfícies

| Superfície | Forma |
|---|---|
| Título da janela | `⧉ ts — api/claude-refactor` |
| Prompt | `⧉ ~/projetos/api ›` |
| Badge de README | `⧉ tesseract // MIT` |
| Favicon | dois quadrados, sólido, sem tessera |
| Fundo claro (README GitHub) | símbolo sólido `#070B0C`, sem neon — verde claro em branco não lê |

---

## 10. Tom de voz

Direto, técnico, no imperativo, com o atalho junto. Sem exclamação, sem emoji, sem primeira pessoa do plural.

| Somos | Não somos |
|---|---|
| Precisos | Frios |
| Curtos | Secos |
| Técnicos | Herméticos |
| Calmos no erro | Dramáticos |

### Microcopy real

```
Nenhuma célula neste projeto. `n` cria a primeira.
Travado esperando você. `Enter` aprova, `Esc` recusa.
Caiu com exit 1. `r` reinicia no mesmo lugar.
8 células recuperadas. Mesma posição.
Célula órfã: processo vivo, projeto sumiu. `m` move, `k` mata.
```

### Anti-exemplos

```
Ops! Algo deu errado 😅
Carregando sua mágica...
ACESSO CONCEDIDO, RUNNER.
Nenhum item encontrado.
```

O terceiro é o risco novo da v2: **neon na imagem não autoriza cosplay no texto**. A voz continua seca.
O quarto falha por outro motivo: descreve o vazio sem dar a saída. Todo estado vazio termina numa tecla.

---

## 11. Do & Don't

**Faça**

1. Escuridão primeiro — o neon só existe porque 95% da tela está apagada.
2. Verde = posse, ciano = estrutura. Nunca invertido.
3. Glow e scanline só na superfície de marca, nunca no terminal.
4. Diferencie estados por forma e área antes de cor.
5. Um único fósforo aceso por tela.
6. Teste tudo com `NO_COLOR=1` antes de aprovar.

**Não faça**

1. Verde ou ciano em sinal de estado.
2. Chuva de caracteres, katakana decorativo, "ACESSO CONCEDIDO".
3. Glow em texto de UI.
4. Mais de um selo invertido visível ao mesmo tempo.
5. Rosa neon como terceira cor.
6. A palavra "tesseract" sozinha perto de OCR ou imagem.

---

## 12. ANSI 16 — a base do colorscheme

```
background      #070B0C
foreground      #BFD1C6
cursor          #55FFA6
cursor_text     #030507
selection_bg    #121C1F
selection_fg    #E8F4EC

color0   black    #070B0C
color1   red      #C22F38
color2   green    #1F7A4C
color3   yellow   #C9A227
color4   blue     #3E7FA8
color5   magenta  #8B4FC4
color6   cyan     #128C86
color7   white    #BFD1C6

color8   black    #3E534E
color9   red      #FF3B47
color10  green    #55FFA6
color11  yellow   #FFB454
color12  blue     #7DB7E8
color13  magenta  #C77DFF
color14  cyan     #22E0D0
color15  white    #E8F4EC
```

### Tokens → ANSI

| Token | ANSI |
|---|---|
| `brand.core` | `color2` |
| `brand.phosphor` | `color10` |
| `flux.core` | `color6` |
| `flux` | `color14` |
| `state.read` | `color12` |
| `state.block` | `color11` |
| `state.dead` | `color9` |
| `state.orphan` | `color13` |
| `state.working` | `color7` |
| `state.off` | `color8` |

---

## 13. Checklist de implementação

- [ ] 20 tokens como constantes no código, nenhum hex solto
- [ ] Grade em traço simples (NAVEGAR) e duplo (DIGITAR)
- [ ] `brand.phosphor` só acessível pelo componente da célula ativa
- [ ] `aprovar` renderiza barra invertida, não só glifo colorido
- [ ] Banner de boot com o mark 7×5 duotone
- [ ] Prompt e título de janela com `⧉`
- [ ] Rodar com `NO_COLOR=1` — estados continuam distinguíveis
- [ ] Testar em terminal de 16 cores (WSL padrão)
- [ ] Testar o README em tema claro do GitHub
