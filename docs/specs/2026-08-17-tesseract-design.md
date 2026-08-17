# Tesseract — especificação de design

**Data:** 2026-08-17
**Status:** aprovado conceitualmente, pronto para virar plano de implementação
**Substitui:** fork pessoal de `claude-squad` (`~/claude-squad-andre`) — do fork sobrevive
apenas o conceito; nenhum código é reaproveitado.

---

## 1. Por que existe

O fork atual resolveu o problema certo — acompanhar vários agentes ao mesmo tempo sem
entrar em cada um — mas cresceu por acumulação. Cada feature nova disputou o mesmo
teclado, criou seu próprio caminho de tela e sua própria regra. Hoje a lista e o mosaico
discordam entre si, a mesma tecla faz coisas diferentes dependendo do painel, e o Docker
é tratado como se fosse propriedade de uma sessão quando na verdade é do projeto.

O Tesseract nasce da mesma necessidade, com a decisão explícita de **padronizar antes de
crescer**: uma unidade só, uma regra só, um lugar só onde as decisões moram.

Objetivo de longo prazo declarado pelo usuário: **substituir a IDE**. Ele já trabalha
majoritariamente dentro de agentes e só abre a IDE para ler markdown, revisar diff de PR
e olhar a árvore git. O V1 ataca o primeiro desses três.

### 1.1 Restrições de contexto

- Roda no **WSL** sobre Windows. Processos morrem sem aviso (`wsl --shutdown`, suspensão
  da máquina, reinício do Windows). Perder trabalho por causa disso é inaceitável.
- Todo projeto do usuário tem um **Docker Compose** rodando.
- O usuário **não lê código**. A entrega é uma aplicação pronta e funcionando; arquitetura,
  estrutura de pastas e stack são decisão de quem implementa.
- A aplicação precisa aceitar features novas sem degenerar. Não haverá um segundo
  Frankenstein.

---

## 2. Nome

**Tesseract.** Comando `tess`.

O tesseract é o hipercubo: uma grade estendida em uma dimensão a mais — que é exatamente
a tela do produto, células dentro de projetos dentro de uma grade. A raiz é a mesma de
*tessera*, a peça individual de um mosaico romano.

O comando é `tess` e não `tesseract` porque `tesseract` é o nome do motor de OCR do Google.
Nenhum dos dois está ocupado na máquina hoje, mas a colisão é evitável.

---

## 3. Modelo conceitual

Existem exatamente **três coisas**. Nada mais.

### 3.1 Projeto

Um diretório versionado — uma aplicação sobre a qual o usuário trabalha.

Nasce junto com sua primeira célula — criar projeto e criar célula são o mesmo gesto, porque
projeto sem célula não faz sentido. **Some da tela quando a última célula dele morre.**

**O disco nunca é tocado**: um projeto sair da tela não apaga, não move e não altera nada.

Guarda: caminho, cor da coluna, ordem na tela, arquivo de compose detectado (se houver), e
as células que moram nele.

### 3.2 Célula

A unidade de trabalho. Toda célula tem tipo, nome, estado, posição na coluna e histórico.
Nasce, vive, pede atenção, morre.

**A mesma regra vale para todas.** A mesma tecla cria, mata, nomeia, foca e navega em
qualquer célula, seja ela um agente conversando ou um arquivo markdown parado.

Tipos do V1:

| Tipo | O que é | Família |
|---|---|---|
| `claude` | Claude Code no diretório do projeto | com processo, interativa |
| `cursor` | Cursor CLI no diretório do projeto | com processo, interativa |
| `bash` | shell no diretório do projeto | com processo, interativa |
| `logs` | log ao vivo de um serviço do compose | com processo, só leitura |
| `md` | arquivo markdown renderizado, recarrega quando o disco muda | sem processo, só leitura |

Duas famílias por dentro, **uma só por fora**. A distinção entre "tem processo" e "não tem
processo" é detalhe de implementação e nunca aparece para o usuário como regra diferente.

Não há limite de células por projeto. O fork tinha teto de dez sessões; a tira lateral e a
rolagem da coluna resolvem excesso sem inventar um número arbitrário.

### 3.3 Painel Docker

Pertence ao projeto, não é célula. Existe quando o projeto tem arquivo de compose na raiz.
Não se cria e não se mata.

Foi a colisão conceitual mais clara do fork: lá o Docker era propriedade da sessão, então
dois agentes no mesmo projeto produziam dois painéis do mesmo compose, sem resposta para
"qual deles eu uso para derrubar a stack".

---

## 4. Estado da célula

Estado de turno só faz sentido para quem conversa. Um shell não "responde".

| Estado | Quem tem | Significado |
|---|---|---|
| `▸ trabalhando` | todas | processo vivo produzindo |
| `⬤ respondeu` | `claude` `cursor` | devolveu a vez, tem resposta esperando leitura |
| `⏵ aprovar` | `claude` `cursor` | travou numa pergunta e **não anda** sem resposta |
| `✖ caiu` | com processo | o processo morreu sozinho |
| `○ parada` | todas | sem processo, célula preservada |
| `⚠ órfã` | todas | o diretório do projeto sumiu do disco |

**Respondeu ≠ aprovar.** É a distinção que faz o alarme valer alguma coisa, e é herdada do
fork porque funciona. Agente parado numa pergunta bloqueia o trabalho; agente que terminou
o turno apenas tem algo para ler.

**Nenhum alarme falso.** Spinner piscando e cursor se mexendo não contam como atividade.
O motor exige leituras consecutivas de trabalho antes de considerar a célula "armada", e
leituras consecutivas de silêncio antes de declarar o turno encerrado — o mesmo critério
que já provou funcionar no fork.

**Estado lido pelo processo, não por texto na tela.** Detectar que o agente caiu olhando
palavras na saída é frágil; o motor olha o processo.

### 4.1 Nome da célula

Toda célula nasce com nome automático (`claude 1`, `bash testes`). O usuário renomeia
quando quiser.

Célula `claude` pode **adotar o nome que o próprio Claude deu à conversa**, sem digitar
nada. O nome escolhido à mão dentro do agente tem prioridade sobre o título automático que
o agente gera. Se a conversa ainda não tem nome, a ação avisa em vez de renomear para vazio.

Nome é rótulo. Trocar o nome não mexe em processo, diretório nem histórico.

### 4.2 Identidade

Cada célula tem identidade estável que **não é o processo**. O motor guarda a intenção —
este projeto, esta célula, deste tipo, com este nome, nesta posição, ligada a esta conversa.

O processo morre; a identidade fica. É isso que permite reconstituir tudo depois de uma
queda do WSL sem perder trabalho.

---

## 5. Telas

Três telas e um formulário.

### 5.1 Mosaico — tela principal

Todos os projetos ao mesmo tempo, cada projeto uma coluna própria.

```
 TESSERACT            ⬤ 3   ⏵ 1          NAVEGAR       ⏳ 59% 2:47
┌───┬──────────────── CORTZ-WEB ─────────────────────┬───┐
│ D │ ┌ claude · fix nav ─────────────── ⬤ RESPONDEU ┐│ A │
│ O │ │ Ajustei o Header pra colapsar em <768px.     ││ P │
│ X │ │ Build passou. Cubro o menu mobile também?    ││ I │
│ A │ └──────────────────────────────────────────────┘│ - │
│ R │ ┏ bash · testes ───────────────────── ▸ RODANDO┓│ L │
│   │ ┃ $ pnpm test                                  ┃│ E │
│ 5 │ ┃   ✓ 12 passed   ✗ 0 failed                   ┃│ G │
│⬤2 │ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛│ A │
│⏵1 │ ┌ md · spec-m7.md ─────────────────────── ○ ───┐│ D │
│ ● │ │ # Módulo 7 — Fichas clínicas                 ││ O │
│4/5│ │ Registrar atendimento com PHI…               ││ 1 │
└───┴──────────────────────────────────────────────────┴───┘
 ↑↓ célula   ←→ projeto   ↵ digitar   n criar  d docker   ? ajuda
```

**Regra de largura.** A coluna do projeto focado ocupa a largura de leitura de verdade,
com o conteúdo vivo das células. As outras colunas encolhem para uma tira estreita.

**A tira nunca some.** Ela mostra, de cima para baixo: o nome do projeto na vertical, o
número de células, quantas pedem atenção, e o estado do Docker. **Some o texto, nunca o
sinal.** Isso preserva a visão de conjunto que motivou escolher o mosaico global, sem o
custo de células ilegíveis com três projetos abertos.

**Navegar entre projetos** é mover para a coluna do lado: ela engorda, a atual encolhe.
É esse gesto que corresponde a "entrar e sair de um projeto" — não existe uma tela separada
de projeto.

**Uma borda, um significado.** No fork havia duas bordas grossas com sentidos diferentes
(onde estão as setas e para onde vai o que se digita). Com os dois modos explícitos, foco e
teclado são sempre a mesma célula, e uma borda basta.

Quando as células de um projeto não cabem na altura, a coluna rola e a tira sinaliza que há
mais.

### 5.2 O modo é impossível de errar

Em `DIGITAR` o aplicativo fica mudo e **mostra que está mudo**.

```
 tesseract            ⬤ 3   ⏵ 1        ▓ DIGITAR ▓     ⏳ 59% 2:47
┌───┬───────────────── cortz-web ────────────────────┬───┐
│ d │ ┌ claude · fix nav ──────────────── ⬤ respondeu┐│ a │
│ o │ │ Ajustei o Header pra colapsar em <768px.     ││ p │
│ x │ └──────────────────────────────────────────────┘│ i │
│ a │ ┏━ bash · testes ━━━━━━━━━━━━━━━━━━━━ ▸ RODANDO┓│ - │
│ r │ ┃ $ pnpm test                                  ┃│ l │
│   │ ┃   ✓ 12 passed   ✗ 0 failed                   ┃│ e │
│ 5 │ ┃ $ █                                          ┃│ g │
│   │ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛│ a │
└───┴──────────────────────────────────────────────────┴───┘
 ctrl-l devolve o teclado
```

Três sinais redundantes ao mesmo tempo: barra e tiras apagadas, selo `▓ DIGITAR ▓` na
barra de título, e rodapé reduzido a uma linha. A célula focada é a única coisa acesa.

A redundância é deliberada: errar o modo significa digitar um comando na cara do agente.

### 5.3 Lista — o índice

Mesmo conteúdo, sem vídeo. Serve para varrer muitos projetos, achar rápido, e trabalhar em
terminal de pouca altura.

```
 TESSERACT            ⬤ 3   ⏵ 1          NAVEGAR       ⏳ 59% 2:47
┌────────────────────────────┬─────────────────────────────┐
│ DOXAR-API      ~/dev/doxar │ claude · refatora auth      │
│  ⬤ claude  refatora auth   │                             │
│  ⏵ claude  migra prisma    │ Movi a validação de token   │
│  ▸ cursor  revisa PR       │ pro guard. Falta decidir se │
│  ○ bash    testes          │ o refresh entra no mesmo    │
│  ▸ logs    worker          │ fluxo ou vira endpoint.     │
│  ● docker  4/5 up          │                             │
│                            │ Qual você prefere?          │
│ CORTZ-WEB      ~/dev/cortz │                             │
│▸ ⬤ claude  fix nav         │                             │
│  ▸ bash    testes          │                             │
│  ○ md      spec-m7.md      │                             │
│  ○ docker  parado          │                             │
└────────────────────────────┴─────────────────────────────┘
 ↑↓ navegar   ↵ digitar   n criar  v mosaico   d docker   ? ajuda
```

Índice à esquerda, prévia ao vivo da célula selecionada à direita. `↵` entra em `DIGITAR`
na prévia, sem sair da lista.

A tela escolhida (lista ou mosaico) é lembrada entre execuções.

### 5.4 Painel Docker

Uma tecla abre o Docker do projeto focado, sobre a tela. Fecha e volta exatamente para onde
estava.

```
┌──────────── DOCKER · doxar-api ──────── docker-compose.yml ─┐
│                                                             │
│    SERVIÇO      ESTADO         PORTA    SAÚDE      UPTIME   │
│  ▸ api          ● up           :3000    saudável   2h14m    │
│    postgres     ● up           :5432    saudável   2h14m    │
│    redis        ● up           :6379    —          2h14m    │
│    worker       ○ exited (1)   —        —          —        │
│    minio        ● up           :9000    —          2h14m    │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ↑↓ escolhe serviço                                         │
│                                                             │
│  NO SERVIÇO   u sobe   s para   r reinicia   b rebuilda     │
│               l abre o log como célula                      │
│  NA STACK     U sobe tudo   S para tudo   R reinicia tudo   │
│                                                             │
│  esc fecha                                                  │
└─────────────────────────────────────────────────────────────┘
```

`↑↓` continua sendo navegação, como em toda a aplicação. As ações são letras, e a
**maiúscula é sempre a versão da stack inteira** da minúscula correspondente.

**Enquanto o painel está aberto, o teclado é dele** — mesma lógica dos dois modos: nunca há
dois donos do teclado ao mesmo tempo. `esc` devolve.

**Log de um serviço vira célula.** Pedir o log de `worker` cria uma célula do tipo `logs`
no mosaico daquele projeto — com a mesma navegação, o mesmo foco, a mesma tecla de matar e
o mesmo histórico de qualquer outra célula. Assim o log do banco pode ficar fixo ao lado do
agente que está depurando ele.

**Nenhuma ação destrutiva existe aqui.** Não há `down -v`, não há apagar volume. Se um dia
for necessário, entra com confirmação escrita.

**Detecção do compose:** apenas na raiz do diretório do projeto, sem busca recursiva.
Projeto sem compose simplesmente não tem painel Docker, e a tira não mostra indicador.

### 5.5 Formulário de criação

**Um formulário só, para tudo.** Não existe "criar projeto" separado de "criar célula":
o formulário começa perguntando o projeto e termina criando a célula.

```
┌──────────────────────── NOVA ───────────────────────┐
│                                                     │
│  PROJETO ~/dev/cortz-web▏                           │
│          tab completa · caminho novo cria projeto   │
│                                                     │
│  TIPO    ▸claude    cursor    bash    logs    md    │
│                                                     │
│  NOME    fix do menu mobile_                        │
│          (vazio → nome automático)                  │
│                                                     │
│  MD      docs/spec-m7.md▏                           │
│          tab completa · 3 arquivos casam            │
│                                                     │
│  PROMPT  (opcional — sobe já trabalhando)           │
│          ▏                                          │
│                                                     │
│  ↵ criar        esc cancelar                        │
└─────────────────────────────────────────────────────┘
```

- **PROJETO** vem preenchido com o projeto focado. Digitar um caminho que ainda não está na
  tela cria o projeto ali mesmo. Autocomplete por `tab`, mostrando quantas pastas casam.
  O caminho é validado na hora — precisa existir e ser gravável; se não for, o campo
  continua aberto com o que foi digitado.
- O quarto campo depende do tipo: `md` → arquivo com autocomplete; `logs` → seletor de
  serviço do compose; `claude` `cursor` `bash` → some.
- **PROMPT** aparece só para `claude` e `cursor`, e cobre o "criar já trabalhando" que no
  fork era uma tecla separada.
- O perfil de agente usado por `claude` e `cursor` vem da configuração, com o padrão em
  primeiro.

---

## 6. Modos e teclado

### 6.1 Princípio

Uma tecla, um significado, em qualquer tela e em qualquer projeto. Se `D` mata célula no
mosaico, mata célula na lista. Sem exceção por painel — foi exatamente isso que quebrou no
fork.

### 6.2 Os dois modos

Por padrão o usuário está em **NAVEGAR**: toda tecla é do aplicativo.

Uma tecla entra em **DIGITAR**: toda tecla é da célula, **sem nenhuma exceção**. Nem `q`,
nem `D`, nem `tab`, nem as setas. Uma tecla devolve o teclado ao aplicativo.

Nunca há dois donos do teclado ao mesmo tempo, então colisão é estruturalmente impossível.

### 6.3 MODO NAVEGAR

**Andar — só setas, nenhuma letra**

| Tecla | Ação |
|---|---|
| `↑` `↓` | célula anterior / próxima na coluna |
| `←` `→` | projeto anterior / próximo (a coluna engorda) |
| `espaço` | pula para a próxima célula que pede atenção — atravessa projeto |
| `1`…`9` | vai direto para o projeto N |

Navegação é exclusivamente direcional. Letra nenhuma anda pela grade, para que o
vocabulário de movimento seja um só e não concorra com ação.

**Teclado e tela**

| Tecla | Ação |
|---|---|
| `↵` | entra em DIGITAR na célula focada |
| `ctrl-l` | sai de DIGITAR, devolve o teclado ao aplicativo |
| `o` | célula focada em tela cheia (liga/desliga); `esc` também sai |
| `v` | alterna mosaico ↔ lista |

**Criar e matar**

| Tecla | Ação |
|---|---|
| `n` | criar — pede o projeto, depois a célula (formulário único, §5.5) |
| `r` | retoma célula parada, ou sobe célula caída |
| `D` | mata a célula focada — confirma. Se for a última do projeto, a confirmação avisa que o projeto sai da tela |

Não existe tecla para fechar projeto: o projeto sai da tela quando sua última célula morre.

**Nomear**

| Tecla | Ação |
|---|---|
| `R` | renomeia a célula **e propaga o nome para dentro do agente** — `/rename` no Claude Code, o comando equivalente no Cursor CLI. Em `bash`, `logs` e `md` só troca o rótulo |
| `ctrl-r` | adota na célula o nome que o agente deu à conversa |

Os dois sentidos coexistem sem se atrapalhar: `R` empurra o nome escolhido pelo usuário para
dentro do agente; `ctrl-r` puxa para a célula o nome que o agente escolheu sozinho.

**Agir**

| Tecla | Ação |
|---|---|
| `p` | manda prompt para a célula focada sem entrar nela — vale para `claude` e para `cursor`, conforme o tipo da célula |
| `d` | abre o painel Docker do projeto focado |
| `ctrl-e` | abre o diretório do projeto no editor configurado |

**Ler**

| Gesto | Ação |
|---|---|
| roda do mouse | rola o histórico da célula sob o cursor |
| `/` | busca no histórico da célula focada |
| `esc` | sai da rolagem / fecha o que estiver aberto |

**Sistema**

| Tecla | Ação |
|---|---|
| `?` | ajuda |
| `q` | fecha a tela. O motor continua rodando — nada morre |

### 6.4 MODO DIGITAR

| Tecla | Ação |
|---|---|
| `ctrl-l` | devolve o teclado ao aplicativo |
| todo o resto | vai para a célula, sem exceção |

A roda do mouse continua rolando o histórico, porque não é tecla.

### 6.5 Selecionar e copiar texto

O aplicativo segura o mouse para receber a roda, e isso desliga a seleção nativa do
terminal. O escape padrão é **`shift` + arrastar**, que o Windows Terminal respeita.

No mosaico, porém, `shift` + arrastar pega os vizinhos: a mesma linha da tela atravessa
várias colunas. Para copiar um bloco de texto, `o` põe a célula em tela cheia e a seleção
passa a pegar só ela.

Por isso **não existe uma tecla para "devolver o mouse ao terminal"** — o fork precisava de
uma; aqui a tela cheia resolve sem gastar tecla.

### 6.6 Teclas livres, de propósito

Minúsculas: `a` `b` `c` `e` `f` `g` `h` `i` `j` `k` `l` `m` `s` `t` `u` `w` `x` `y` `z`
Maiúsculas: todas menos `D` e `R`
Mais `tab` e `ctrl-` de quase todas as letras.

Dezenove teclas minúsculas livres. Feature nova não precisa roubar tecla de ninguém.

### 6.7 Revisão do que existe hoje no fork

**Fica igual**

Estados de turno e a distinção respondeu ≠ aprovar · heurística contra alarme falso · som e
notificação desligáveis com comando customizável · `espaço` pula para quem chamou · `p`
manda prompt sem entrar · `ctrl-r` adota o nome do agente · `ctrl-e` abre no editor · badge
de consumo da janela de 5h · perfis de agente configuráveis · autocomplete de diretório por
`tab` · agrupamento por projeto · a tela escolhida é lembrada.

**Muda**

| Hoje | No Tesseract | Por quê |
|---|---|---|
| `tab` troca painel dentro da sessão | morre | painel virou célula; não há o que trocar |
| `n` cria sessão, `N` cria com prompt | `n` cria tudo: projeto e célula, num formulário só | projeto sem célula não faz sentido; prompt virou campo |
| `o` = sinônimo de `↵` | `o` = tela cheia | `↵` já entra; e a tela cheia passa a ser o jeito de copiar texto sem pegar os vizinhos |
| `↑↓` e `jk` navegam | só `↑↓` | letra nenhuma anda pela grade; movimento é direcional e ponto |
| `R` renomeia só o rótulo | `R` renomeia **e propaga para dentro do agente** | o nome da célula e o nome da conversa deixam de divergir |
| `shift-↑↓` rola o histórico | roda do mouse | rolar é gesto de mouse; libera as teclas |
| `D` no mosaico mata sem confirmar | sempre confirma | mesma tecla, mesma regra, em toda tela |
| diff `+/-` por sessão | por projeto, no cabeçalho da coluna | é um repositório só |
| Docker é aba da sessão | painel do projeto + célula de log | o compose pertence ao projeto |
| Notificação sai da tela | sai do motor | avisa mesmo com a tela fechada |

**Morre**

| O que | Por quê |
|---|---|
| `max_sessions` (teto de 10) | limite artificial; a tira e a rolagem resolvem excesso |
| Auto-yes e o daemon que aprova sozinho | aprovar sem ler é o que o estado `⏵ aprovar` existe para impedir. Reponível a pedido |
| `ctrl-space` (devolver o mouse ao terminal) | `o` + `shift` + arrastar cobre o caso sem gastar tecla (§6.5) |
| `J` `K` `H` `L` (reordenar) | a ordem é a de criação. Reordenar fica fora do V1 e volta sem gastar tecla nova, se a falta aparecer |
| `X` (fechar projeto) | o projeto sai da tela quando sua última célula morre |
| `restart.sh` / `clean.sh` / `clean_hard.sh` | viram `tess reset` |

---

## 7. O motor

### 7.1 O que ele é

Um serviço de usuário rodando no WSL. Sobe junto com o WSL, sobrevive a fechar a tela, e é
dono de tudo: os processos, a tela interna de cada célula, o histórico e o estado desejado.

A tela é **cliente**. Conecta ao motor, desenha o que ele manda, devolve tecla. Nenhuma
regra mora na tela.

O motor mantém a tela interna de cada célula em memória — ele **já sabe** o que está escrito
em cada uma, sem precisar perguntar a ninguém a cada quadro. É isso que mantém o mosaico
fluido com quinze células vivas, e é a diferença central em relação ao fork, que precisava
consultar o tmux a cada rodada.

Mais de uma tela pode estar conectada ao mesmo tempo, em terminais diferentes, vendo o
mesmo estado.

### 7.2 O que fica em disco

| O quê | Comportamento |
|---|---|
| Estado desejado | projetos, células, tipo, nome, posição, conversa ligada. Escrita atômica. Se corromper, carrega o que der, preserva o arquivo original e avisa na tela |
| Histórico por célula | arquivo próprio, com teto de tamanho e descarte do começo. É o que sustenta rolagem e busca depois de uma queda |
| Configuração | perfis de agente, editor, som, notificação, teto de histórico |

### 7.3 Recuperação

Cenário: `wsl --shutdown`. Todos os processos morrem.

O usuário reabre o WSL. O serviço sobe sozinho e reconstrói a grade nas mesmas posições:

| Tipo | O que acontece |
|---|---|
| `claude` `cursor` | reata a conversa de onde parou e acorda **parado**. Nenhum prompt é disparado |
| `bash` | shell novo e limpo; o histórico anterior fica rolável acima da linha de queda |
| `logs` | volta a acompanhar o serviço; se a stack estiver parada, fica `○ parada` e engata sozinha quando o serviço subir |
| `md` | relê o arquivo |
| Docker | **não sobe sozinho**. Subir stack é decisão do usuário |

O risco que a recuperação automática cria — agente trabalhando sem ninguém na frente da
tela — é cortado na raiz: **reatar a conversa nunca dispara trabalho**. O agente acorda
esperando.

### 7.4 Célula que cai com o aplicativo rodando

Vira `✖ caiu`, toca o aviso, e espera. `r` sobe de novo.

**Nada reinicia sozinho.** Reinício automático esconde causa raiz e produz loop de crash
silencioso.

### 7.5 Aviso

O motor é quem notifica, não a tela. Som e notificação do sistema com o nome da célula e do
projeto, detectando o comando de notificação disponível (`wsl-notify-send.exe` no WSL), com
possibilidade de apontar outro comando. Ambos desligáveis.

Como o motor é independente da tela, o usuário é avisado mesmo com a tela fechada — o que o
fork não fazia.

### 7.6 Badge de consumo

A barra de título mostra o consumo da janela de 5 horas e o tempo até virar. O Claude Code
entrega esse número apenas para o statusline, então o motor lê de um arquivo que o
statusline do usuário escreve. O badge desaparece se o dado estiver velho e muda de cor
acima de 80%. Sem o arquivo, tudo funciona igual — só não aparece o badge.

---

## 8. Como isso não vira outro Frankenstein

Três regras duras. São elas que respondem à exigência de aceitar features novas.

**1. Tipo de célula é uma peça fechada.** Um tipo declara quatro coisas: como nasce, como se
desenha, o que faz com uma tecla, e quais estados tem. Nada fora dele muda. Adicionar a
célula de git, de PR, de banco de dados ou de qualquer outra coisa é escrever essa peça — o
mosaico, a lista, os atalhos e o motor não são tocados.

**2. Feature nova não ganha tecla nova.** O conjunto de teclas é fechado e mora num lugar
só. Feature entra como tipo de célula, como ação do painel Docker, ou como comando escrito.
Se de fato precisar de tecla, alguém perde a dela — decisão consciente, não acúmulo. Foi o
acúmulo silencioso que criou o Frankenstein.

**3. A tela não decide nada.** Ela desenha o que o motor manda. Regra nova entra no motor
uma vez e aparece nas duas telas de graça. É estruturalmente impossível a lista e o mosaico
discordarem.

---

## 9. Comandos de linha

| Comando | O que faz |
|---|---|
| `tess` | abre a tela, conectando ao motor (sobe o motor se não estiver rodando) |
| `tess novo <dir>` | adiciona um projeto sem abrir a tela |
| `tess status` | estado do motor e resumo de projetos e células |
| `tess stop` | desliga o motor e todas as células |
| `tess reset` | apaga o estado salvo e derruba tudo, preservando a configuração |

---

## 10. Escopo

### 10.1 Entra no V1

Motor como serviço, com reconstituição · projetos · células `claude` `cursor` `bash` `logs`
`md` · mosaico com colunas e tira de status · lista com prévia · os dois modos · painel
Docker com ação na stack e no serviço · histórico gravado com rolagem e busca · notificação
pelo motor · badge de consumo · os cinco comandos de linha.

### 10.2 Fica de fora, conscientemente

| O que | Quando volta |
|---|---|
| Célula de git (árvore local + diff) | quando o V1 estiver em uso diário e a falta aparecer |
| Célula de PR do GitHub | idem — é a mais cara: depende de rede, autenticação e tratamento de erro de API |
| Auto-yes | se o usuário confirmar que usa de verdade |
| Comando escrito (paleta) | quando a primeira feature não couber nas dezesseis teclas livres |
| Configuração visual e temas | até ser pedido |
| Rodar fora do WSL | até ser necessário |

---

## 11. Decisões registradas

| # | Decisão | Alternativa descartada e por quê |
|---|---|---|
| 1 | Tudo é célula tipada; uma regra só de criar, matar, nomear e focar | Separar "agente" de "infra" cria duas regras de interação — a semente da colisão que existe no fork |
| 2 | Projeto é diretório versionado e dono do Docker | Docker por sessão duplica painel e deixa "quem derruba a stack" sem resposta |
| 3 | Mosaico global com projeto por coluna; a focada engorda, o resto vira tira | Paginar esconde justamente o projeto que chamou; rolar de lado tem o mesmo defeito sem fronteira clara |
| 4 | Motor como serviço que sobe com o WSL e reconstitui sozinho | Reconstituição só ao abrir a tela deixa o usuário sem aviso enquanto não abre |
| 5 | Painel Docker lista serviços e age; log vira célula | Log dentro do painel é sempre efêmero e cria navegação interna própria |
| 6 | Dois modos explícitos, nada reservado em DIGITAR | Teclas reservadas fazem toda feature nova roubar tecla do agente |
| 7 | V1 fecha com `claude` `cursor` `bash` `logs` `md` | Git e PR são caros e o modelo de célula tipada torna barato adicioná-los depois |
| 8 | Motor com tela interna própria por célula | Usar tmux como substrato obriga a consultar processo externo a cada quadro e deixa `md` e Docker fora do modelo |
| 9 | Navegação só por seta; nenhuma letra anda pela grade | `jk`/`hl` fazem movimento e ação disputarem o mesmo vocabulário, e é de onde vem a sensação de tecla imprevisível |
| 10 | Criar projeto e criar célula são um gesto só; projeto some com a última célula | Ter tecla própria para abrir e fechar projeto cria estado vazio (projeto sem célula) que não serve para nada |
| 11 | `R` propaga o nome para dentro do agente | Renomear só o rótulo deixa o nome da célula e o nome da conversa divergirem, que é o problema que o `ctrl-r` do fork tentava remendar de um lado só |
| 12 | Rolar é roda do mouse; copiar é tela cheia + `shift` | Manter `shift-↑↓` e `ctrl-space` gastava três teclas para o que dois gestos de mouse resolvem |
