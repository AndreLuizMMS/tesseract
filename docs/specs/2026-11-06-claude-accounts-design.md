# Contas Claude visíveis, trocáveis e com failover sistêmico — design

**Data:** 2026-11-06
**Status:** aprovado pelo usuário; perguntas da revisão decididas pelo autor (marcadas "chamada minha")
**Base:** o failover por limite que já existe (`internal/engine/accounts.go`, `ts account add`,
evento `limit` do mod `tesseract-link`).

---

## 1. Por que

O usuário roda várias células Claude ao mesmo tempo, espalhadas em duas contas
(`~/.claude` e `~/.claude-norven`). Algumas células estão no limite e outras não, e ele não
consegue saber em qual conta cada uma está nem trocar de conta. Hoje há três falhas de
sistema por trás disso:

1. **A conta não aparece em lugar nenhum.** Para saber, é preciso abrir `/status` dentro da
   célula ou ler o ambiente do processo.
2. **O uso é global e misturado.** A barra de status mostra um único `quota %`, o do último
   relato que chegou, seja de qual conta for (`internal/engine/quota.go`, `reportedQuota`).
   A célula na norven e a célula na primária disputam o mesmo número.
3. **O failover só acontece quando o turno falha.** As outras células paradas na conta
   esgotada só descobrem o limite quando você manda um prompt e ele falha. Além disso, o
   engine guarda o limite só na memória e o perde quando reinicia.

Sucesso = olhar o mosaico e saber a conta de cada célula, trocar qualquer célula de conta
pelo teclado e nunca perder um prompt num limite que o Tesseract já conhece.

## 2. Objetivos

- **G1. Conta visível em toda célula:** a linha de cima de cada célula Claude mostra o nome
  curto da conta.
- **G2. Uso por conta:** a barra de status mostra o uso de cada conta separadamente.
- **G3. Troca manual:** uma tecla abre um seletor de contas e move a conversa da célula para
  a conta escolhida.
- **G4. Failover sistêmico:** quando uma conta é reconhecida como esgotada, as células
  ociosas nela migram na hora, e as que estão trabalhando migram ao terminar ou falhar o
  turno (detalhes e exceções na 4.4).
- **G5. Limite sobrevive ao reboot:** o engine reiniciado já sabe quais contas estão de fora
  e até quando.

## 3. Não-objetivos

- **Sem conta fixa por célula.** O reinício (`r`, reboot do engine, aba que acordou) escolhe
  a primeira conta livre da lista, ou seja, a primária sempre que ela puder. A troca manual
  vale até o próximo reinício. *(Decisão do usuário.)*
- **Sem volta automática para a primária** quando o limite dela reseta. A célula fica onde
  está até reiniciar.
- **Sem escolher a conta na criação (`n`).** A célula nova nasce na primeira conta livre e
  pode ser trocada depois com a tecla.
- **Sem mudar o `ts account`** (listar e adicionar contas continua como está).

**Anti-objetivos**, coisas que fariam a feature falhar mesmo funcionando:
- Uma troca, manual ou automática, que comece um turno que ninguém pediu.
- Cortar o turno de uma célula que está trabalhando por causa do limite de *outra* célula.
- Rótulo de conta que some ou mente: célula numa conta e borda dizendo outra.

## 4. Comportamento decidido

### 4.1 Conta na borda da célula (G1) — *decisão do usuário: borda de toda célula*

```
╭ doxar · claude shell ──────── norven ─ ● ╮
╭ regula-mais · claude ─────── primary ─ ● ╮
```

- O rótulo é o nome curto da pasta da conta: `~/.claude-norven` → `norven`,
  `~/.claude-2` → `2`. A pasta nativa `~/.claude` aparece como `primary`. *(Chamada minha.)*
- Só aparece com 2 ou mais contas configuradas. Com uma conta só, nada muda. *(Chamada minha.)*
- Só células com Claude rodando mostram o rótulo. Numa sessão com abas, ele vale para a aba
  claude e continua visível nas outras abas, porque a conta é da conversa. Aba dormindo ou
  célula parada não mostra conta, já que não há processo. *(Chamada minha.)*
- Prioridade de espaço na linha de cima, do que some por último ao que some primeiro: nome
  → glifo de estado → `ctx %` → rótulo da conta → nota. Célula estreita perde a nota antes
  de perder a conta. *(Chamada minha.)*
- A conta numa conta esgotada é pintada na cor de aviso. *(Chamada minha.)*
- A vista de lista (`v`) mostra a mesma informação onde couber. *(Chamada minha.)*

### 4.2 Uso por conta na barra de status (G2)

```
status:  primary 100% · 15h  │  norven 34%
```

- Cada relato de `quota` do mod é guardado sob a conta da célula que o mandou, não mais num
  número global.
- O formato e as regras de cor seguem o selo atual: aviso a partir de 80%, horário de reset
  quando está alto e a janela de 7 dias quando ela é a que está acabando, só que uma vez por
  conta.
- Conta esgotada mostra `100%` com o horário em que volta.
- O arquivo da statusline (`tesseract-quota.json`) continua como fallback só da conta onde
  ele está. *(Chamada minha.)*

### 4.3 Troca manual (G3) — *decisão do usuário: tecla abre lista de contas*

Tecla **`a`** na célula com foco (livre hoje):

```
╭ trocar conta · doxar ───────────────╮
│   primary   100%  no limite até 15h  │
│ ▸ norven     34%  ← atual            │
│                                      │
│ ↵ move a conversa   esc cancela      │
╰──────────────────────────────────────╯
```

- Lista as contas na ordem do config com o uso e o estado de cada uma.
- `↵` numa conta livre: só a aba claude reinicia na conta escolhida, na mesma conversa
  (mesmo caminho do failover, com cópia do transcript e confiança da pasta). O shell e as
  outras abas não percebem.
- `↵` numa conta no limite: avisa ("norven está no limite até 15h") e não move.
- `↵` na conta atual: fecha sem fazer nada.
- A troca manual **não manda prompt nenhum**. A conversa volta parada no prompt, esperando
  você. *(Chamada minha, por causa do anti-objetivo.)*
- Célula trabalhando ou esperando aprovação: o seletor mostra a linha "corta o turno em
  andamento" e deixa trocar mesmo assim, porque foi você que pediu. *(Chamada minha.)*
- Célula sem Claude rodando (parada, aba dormindo) ou com uma conta só: a tecla avisa por
  quê e não abre o seletor.
- Aviso ao concluir: `doxar: moved to norven`.

### 4.4 Failover sistêmico (G4) — *decisão do usuário: move as ociosas na hora*

**Invariante:** nenhuma célula Claude ociosa fica numa conta esgotada enquanto existir uma
conta livre.

**Quando uma conta vira esgotada:** só quando o mod relata `limit`, ou seja, um turno de
verdade parou no limite. Um relato de `quota` em 100% **não** tira a conta, porque uma conta
com uso extra pago continua funcionando em 100%. O uso serve só para exibir.
*(Chamada minha, depois da revisão.)*

A conta fica de fora até o reset mais distante entre as janelas cheias, ou 5h quando não se
sabe.

**O que acontece com cada célula na conta esgotada:**

| Estado da célula | Ação |
|---|---|
| A que relatou `limit` (turno cortado) | migra **com** o prompt de retomada atual ("pick up where you left off") |
| Ociosa e lida: `idle`, `interrupted` | migra **sem prompt**, se ninguém mexeu nela no último minuto |
| Com resposta não lida: `answered` | não migra sozinha, para não apagar o glifo e o resumo que dizem "leia aqui"; migra quando ficar `idle` ou quando o próximo turno falhar (com prompt de retomada) |
| Trabalhando ou esperando aprovação | segue; migra quando ficar ociosa (sem prompt) ou quando o turno falhar (com prompt) |
| Em background (`◌`), parada ou aba dormindo | não mexe; escolhe conta quando voltar |

**Proteção para quem está digitando:** uma célula ociosa que recebeu tecla ou mostrou saída
há menos de 60s (a medida de inatividade que a célula já tem) fica para a próxima checagem.
Assim, um prompt pela metade na caixa de texto não se perde. *(Chamada minha.)*

**Quando a checagem roda:** a "rebalanceada", que aplica a invariante a todas as células,
roda nestes momentos:
- chegou um relato `limit`;
- uma célula numa conta esgotada ficou `idle` ou `interrupted`;
- chegou a hora de reset mais próxima, o que pode liberar uma conta enquanto outras
  continuam de fora;
- a cada 60s, enquanto existir uma célula ociosa pendente numa conta esgotada e uma conta
  livre;
- na subida do engine, com os limites persistidos.

Migração por rebalanceada nunca leva prompt. Só a célula cujo turno foi cortado, na hora do
corte, recebe o prompt de retomada. *(Chamada minha.)*

- A migração vai para a primeira conta livre da lista.
- Cada migração gera um aviso: `regula-mais: primary at its limit, moved to norven`.
- Com todas as contas de fora, ninguém migra e um aviso único diz que todas estão no limite
  e quando a primeira volta.
- Duas células batendo o limite ao mesmo tempo não podem gerar migração dupla da mesma
  célula. Uma célula já migrando é ignorada até subir. *(Chamada minha.)*

### 4.5 Limite sobrevive ao reboot (G5)

- O "esgotada até X" de cada conta é salvo em disco, junto do estado do engine, e relido na
  subida. Limite já vencido é descartado. *(Chamada minha.)*
- Com isso, "volta pra primária ao reiniciar" significa a primeira conta **livre**: uma
  primária ainda esgotada não recebe a célula só para falhar um turno.

## 5. Restrições

- Linguagem Go do engine e da tela (Bubble Tea/lipgloss), sem dependência nova.
- O mod `tesseract-link` (TypeScript) só muda se for preciso para o relato de `quota`
  carregar o que a seção 4.4 usa. A conta já é conhecida do lado do engine pela célula que
  relatou.
- O protocolo engine↔tela ganha a conta da célula e o uso por conta. Tela antiga com engine
  novo, ou o contrário, não pode quebrar: campos novos são opcionais.
- Atualizar `docs/manual.md` (seção de contas), `docs/atalhos.md` (tecla `a`) e a ajuda (`?`).
- Mensagens da UI em inglês, como o resto do Tesseract.

## 6. Deixado para quem constrói

- Como a troca manual força a conta de um único spawn (hoje o spawn sempre pergunta ao
  engine a conta atual).
- Formato do arquivo de limites persistidos e onde exatamente ele fica no estado do engine.
- Como a tela recebe a conta: campo no `protocol.Cell` ou outro meio.
- Estrutura de teste. Cobertura mínima esperada: escolha de conta com limites persistidos,
  invariante da 4.4 (ociosa migra, trabalhando não), troca manual para conta esgotada
  recusada, barra de status com duas contas, rótulo na borda com e sem espaço.
