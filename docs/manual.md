# Manual

## Como funciona

Existem exatamente **três coisas**.

### Projeto

Um diretório. Nasce junto com sua primeira célula — criar projeto e criar célula são o
mesmo gesto — e **sai da tela quando a última célula dele morre**. O disco nunca é tocado:
o projeto sair da tela não apaga, não move e não altera nada.

### Célula

A unidade de trabalho. **Uma regra só para todas**: a mesma tecla cria, mata, nomeia, foca
e navega em qualquer uma.

Criar não pergunta o que a célula vai ser. Uma **sessão** nasce com quatro abas por dentro,
todas no diretório do projeto, e `tab` troca entre elas. Só a aba que você está usando tem
processo: as outras sobem quando você chega nelas.

| Aba | O que é |
|---|---|
| `claude` | Claude Code |
| `cursor` | Cursor CLI |
| `bash` | um shell |
| `md` | os markdowns do projeto: lista com busca por nome, e o escolhido renderizado |

A aba `md` abre numa lista de todo markdown do projeto, com uma **barra de busca no topo**.
Entre em DIGITAR (`↵`), digite parte do nome para filtrar, `↑↓` escolhe e `↵` abre o
arquivo. `esc` volta para a lista. O arquivo aberto recarrega sozinho quando o disco muda —
dá para ver o agente escrevendo a spec ao lado.

O documento é desenhado **como página, não como saída de terminal**: medida de leitura
fixa e centralizada, margem dos dois lados, título em faixa, seções com filete, citação
com barra, tabela com régua fina e código em caixa própria. Linha de código mais larga que
a página é **cortada com `›`**, nunca quebrada — diagrama partido no meio não se lê.

Existem ainda duas células que não são sessão:

| Tipo | O que é |
|---|---|
| `logs` | log ao vivo de um serviço do compose, criado pelo painel Docker |
| `md` | um markdown específico, quando você preenche o campo MD na criação |

Cada célula tem um estado, e é ele que aparece no marcador. Todo estado tem **três** sinais
ao mesmo tempo — um glifo, uma cor e uma forma — para que nenhum deles seja indispensável:

| Sinal | Estado | O que fazer |
|---|---|---|
| `▸ TRABALHANDO` | processo vivo produzindo | nada — deixe trabalhar |
| `⬤ RESPONDEU` | devolveu a vez, tem resposta esperando leitura | leia quando puder; não trava nada |
| `⏵ APROVAR` | travou numa pergunta e **não anda** sem resposta | responda: o trabalho parou nisso |
| `✖ CAIU` | o processo morreu sozinho | `r` sobe de novo |
| `○ PARADA` | sem processo, célula preservada | `r` retoma de onde parou |
| `⚠ ÓRFÃ` | o diretório do projeto sumiu do disco | recrie o caminho ou mate a célula |

`⏵ APROVAR` é o único que vira **barra sólida invertida ocupando a linha inteira** do
cabeçalho da célula, e o único que **pisca** — 1,8s aceso, 200ms apagado, para sempre,
enquanto alguém estiver esperando você. Os outros cinco são um glifo e um rótulo, parados.
É de propósito: urgência aqui é área preenchida, não matiz — funciona de longe, funciona no
canto do olho e funciona sem cor nenhuma.

No quadro apagado a barra **continua barra**: só o fundo muda. A área preenchida é o que
diz "isto trava o trabalho", e ela nunca some. E o relógio só existe enquanto há célula
travada — sem nenhuma, a tela fica completamente parada. `TESSERACT_SEM_PISCA=1` desliga a
piscada para quem prefere a tela imóvel.

**Respondeu ≠ aprovar.** É a distinção que faz o alarme valer alguma coisa: agente parado
numa pergunta bloqueia o trabalho; agente que terminou o turno apenas tem algo para ler.

E **nenhum alarme falso**: spinner piscando e cursor se mexendo não contam como atividade.
O motor exige leituras seguidas de trabalho antes de armar o aviso, e leituras seguidas de
silêncio antes de declarar o turno encerrado.

### Painel Docker

Pertence ao **projeto**, não à célula. O arquivo de compose é procurado na raiz e nas
pastas de primeiro nível — porque projeto de verdade guarda a stack em `docker/`, `infra/`
e afins — e **arquivo de produção nunca é escolhido**. O painel lista os serviços com
estado, porta, saúde e tempo de pé; sobe, para, reinicia e rebuilda serviço ou stack
inteira; e transforma o log de um serviço numa célula do mosaico.

Enquanto o Docker trabalha, o painel diz o que está fazendo e vai atualizando a lista —
os serviços ficam verdes um a um em vez de tudo mudar de uma vez no fim.

**Nenhuma ação destrutiva existe aqui.** Não há `down -v`, não há apagar volume.

## Os dois modos

<p align="center">
  <img src="img/digitar.svg" alt="O modo DIGITAR: a tela apaga, o selo aparece e só a célula com o teclado fica acesa" width="1000">
</p>

Por padrão você está em **NAVEGAR**: toda tecla é do aplicativo.

`↵` entra em **DIGITAR**: toda tecla é da célula, **sem nenhuma exceção**. Nem `q`, nem `D`,
nem `tab`, nem as setas. `ctrl-l` devolve o teclado.

Colar (`ctrl-v`, ou o que o seu terminal usar) funciona em DIGITAR e nos campos de texto.
O texto vai **marcado como colagem**: um prompt de várias linhas entra inteiro na caixa do
agente, em vez de cada quebra de linha virar um envio. Em campo de uma linha só — caminho
do projeto, prompt do `p` — a colagem é achatada numa linha.

Nunca há dois donos do teclado ao mesmo tempo — então colisão de atalho é estruturalmente
impossível. E o modo é impossível de errar, porque ele muda **quatro** coisas de uma vez:

1. o fundo da tela escurece e o resto apaga;
2. a borda da célula focada engrossa e vira dupla;
3. o selo `▓ DIGITAR ▓` aparece invertido;
4. a célula que tem o teclado fica **verde phosphor** — e é o único verde phosphor da tela.

Com `NO_COLOR=1` os sinais 1 e 4 somem, e os sinais 2 e 3 continuam: borda dupla e selo
invertido não dependem de cor nenhuma.

## Copiar o que o agente escreveu

Com o mosaico, a seleção do terminal não serve: ela pega os vizinhos e as bordas junto.
Então a marca é do próprio Tesseract. **Arraste o mouse por cima da célula** — o trecho
acende — e **solte**: o texto vai para a área de transferência, sem cor e sem os espaços do
fim das linhas. Vale nos dois modos e nos dois sentidos do arrasto. `esc` apaga a marca.

Clicar sem arrastar só escolhe a célula; não encosta no que você tinha copiado antes. Para
pegar o que já saiu da tela, **role primeiro** (roda do mouse) e depois arraste — a marca
vale sobre o que está à vista.

## Configuração

Opcional, em `~/.config/tesseract/config.json`. Sem o arquivo, tudo funciona com o padrão.

```json
{
  "editor": "cursor",
  "som": true,
  "notificar": true,
  "comandoNotificacao": "",
  "tetoHistorico": 5242880,
  "agentes": {
    "claude": {
      "programa": "claude",
      "args": ["--model", "opus"],
      "comandoRenomear": "/rename"
    }
  }
}
```

O **badge de consumo** da janela de 5 horas aparece na barra de título quando o seu
statusline do Claude Code escreve `~/.claude/tesseract-quota.json` (ou o antigo
`squad-quota.json`) no formato `{"used_percentage": 59, "resets_at": 1786955400}`. Sem o
arquivo, tudo funciona igual — só não aparece o badge.

## Quem roda dentro de uma célula

Todo processo que o Tesseract sobe recebe `TESSERACT=1` no ambiente. Ele não consegue
pintar a interface de um agente — o que a célula mostra é o que o processo escreveu, e
nada além disso. O que dá para fazer é dizer "você está aqui dentro", e deixar quem se
importa se vestir de acordo.

O statusline do Claude Code é o caso típico: ele é um comando seu, roda dentro do pty da
célula, herda o ambiente e pode abrir a linha com `⧉` e usar a paleta do Tesseract quando
a variável estiver de pé. Fora do Tesseract, nada muda — a marca se apresenta, não se
impõe.

```bash
[ -n "${TESSERACT:-}" ] && echo "esta sessão está numa célula"
```
