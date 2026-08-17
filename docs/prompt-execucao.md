# Implementar o Tesseract

## Objetivo

Entregar o Tesseract funcionando: um gerenciador de instâncias de agentes em terminal,
rodando na WSL como serviço, com mosaico de células por projeto, painel Docker e
recuperação automática depois de queda do WSL.

## Fonte da verdade

`~/tesseract/docs/specs/2026-08-17-tesseract-design.md` — spec aprovada, 11 seções.
Ela manda. Onde este prompt e a spec discordarem, a spec vence. Leia inteira antes
de escrever a primeira linha.

Conflito real entre spec e realidade técnica: pare, relate, proponha. Não decida sozinho.

## Contexto

Projeto novo, em `~/tesseract`, ainda vazio fora da pasta `docs/`.

Existe um fork pessoal em `~/claude-squad-andre` (Go, ~15k linhas, fork de
smtg-ai/claude-squad). **Nenhuma linha dele é reaproveitada.** Ele serve como
referência de comportamento em três pontos específicos, e só:

- `session/instance.go` e `app/notify.go` — a heurística que evita alarme falso ao
  detectar fim de turno do agente (N leituras de trabalho para armar, M de silêncio
  para declarar encerrado)
- `session/quota.go` — leitura do arquivo de consumo da janela de 5h
- `README.md` §3.5 — o formato desse arquivo

Ambiente verificado nesta máquina:

| Item | Estado |
|---|---|
| Go | 1.25.8 |
| systemd na WSL | ativo como pid 1 (`systemd=true` em `/etc/wsl.conf`) |
| Docker Compose | v2.35.1, aceita `--format json` |
| Interop Windows | ligado; `powershell.exe` e `cmd.exe` alcançáveis |
| `wsl-notify-send.exe` | **não instalado** — notificação sai por `powershell.exe` |
| Terminal | Windows Terminal (`xterm-256color`) |

## Stack — fixa

Go 1.25. Exatamente seis dependências, nenhuma a mais:

| Uso | Módulo |
|---|---|
| PTY | `github.com/creack/pty` |
| Emulação de terminal | `github.com/charmbracelet/x/vt` |
| Interface de terminal | `github.com/charmbracelet/bubbletea` v2 |
| Estilo | `github.com/charmbracelet/lipgloss` v2 |
| Markdown renderizado | `github.com/charmbracelet/glamour` |
| Arquivo mudou | `github.com/fsnotify/fsnotify` |

Todo o resto é biblioteca padrão. Docker por `exec` de `docker compose`, sem SDK.
Notificação por `exec` de `powershell.exe`, sem biblioteca. Comunicação entre motor e
tela por socket unix com JSON por linha, sem gRPC. Estado e histórico em arquivo, sem
banco. Testes com `testing` da stdlib, sem framework.

Precisar de uma sétima dependência: pare e proponha antes de adicionar.

## Estrutura — fixa

```
tesseract/
├── cmd/tess/              entrada única: sobe o motor ou conecta nele
├── internal/
│   ├── motor/             estado, projetos, ciclo de vida, persistência
│   │   └── historico/     gravação, rotação, busca
│   ├── celula/            um arquivo por tipo
│   │   ├── celula.go         o contrato: nasce, desenha, recebe tecla, tem estados
│   │   ├── processo.go       base compartilhada das que rodam algo
│   │   ├── claude.go  cursor.go  bash.go  logs.go  md.go
│   ├── docker/            ler o compose, agir na stack e no serviço
│   ├── protocolo/         contrato motor↔tela
│   ├── teclado/           o mapa de teclas, num arquivo só
│   └── tela/              mosaico.go  lista.go  docker.go  formulario.go
└── systemd/tesseract.service
```

Essa estrutura existe para sustentar três regras da spec §8. Elas são obrigatórias:

1. Tipo de célula novo é um arquivo em `celula/` mais uma linha no registro. `tela/`,
   `teclado/` e `motor/` não são tocados.
2. O mapa de teclas mora só em `teclado/`. Nenhum arquivo fora dele associa tecla a ação.
3. `tela/` desenha o que o motor manda e devolve tecla. Nenhuma regra de negócio ali.

## Como executar: seis fases com portão

Dentro de uma fase, itere sozinho: implementa, roda `go build ./... && go vet ./... &&
go test ./...`, corrige, repete até verde. Não peça nada ao humano no meio de uma fase.

No fim de cada fase: pare, entregue o roteiro manual daquela fase, e espere o retorno.
Erro reportado na validação manual volta pra dentro da fase — corrige e devolve o roteiro
de novo. Só avance quando o humano der o OK.

---

### Fase 1 — fatia vertical

Motor com uma célula `bash`: PTY, emulação de terminal em memória, histórico em arquivo,
socket unix, e uma tela que mostra essa célula em tela cheia com os dois modos.

**Testes automatizados obrigatórios**

- `protocolo`: serializar e desserializar toda mensagem devolve o valor original.
- `celula/processo`: sobe `bash`, escreve `echo tesseract\n`, e a tela em memória do
  motor passa a conter `tesseract` em até 2s.
- `motor/historico`: escreve além do teto de tamanho e o arquivo descarta do começo,
  mantendo o fim; busca por termo devolve as linhas certas com o número da linha.
- `motor`: estado desejado é escrito atomicamente; arquivo corrompido carrega o que dá,
  preserva o original com sufixo e devolve o erro.

**Validação manual**

1. `go run ./cmd/tess` — abre com uma célula `bash` em tela cheia, modo NAVEGAR, com a
   barra de título e o rodapé acesos.
2. `↵` — entra em DIGITAR: barra e rodapé apagam, aparece o selo `▓ DIGITAR ▓`, a borda
   da célula engrossa.
3. Digite `q` e `D` — as duas letras aparecem no shell, nada acontece no aplicativo.
4. `ls -la` + `↵` — a saída aparece. `ctrl-l` — volta pra NAVEGAR, chrome acende.
5. `q` — a tela fecha. `tess status` mostra o motor vivo com uma célula.
6. `go run ./cmd/tess` de novo — a mesma célula está lá, com a saída do `ls` ainda
   visível, e o shell responde.
7. Role com a roda do mouse — o histórico sobe; `esc` volta ao vivo.

---

### Fase 2 — projetos e mosaico

Projetos como coluna, tira de status para os não focados, navegação por seta, formulário
único de criação, `n` `D` `o` `v`.

**Testes automatizados obrigatórios**

- `teclado`: **nenhuma tecla tem dois significados** no mesmo modo; em DIGITAR a única
  tecla reservada é `ctrl-l`; toda tecla do mapa tem texto de ajuda. Esse teste é a
  garantia mecânica da regra 2 — se ele quebrar, a regra foi violada.
- `tela/mosaico`: com 3 projetos e largura fixa de 120 colunas, a coluna focada recebe
  largura de leitura e as outras viram tira; a saída renderizada bate com um golden file.
  Mudar o foco muda qual coluna engorda.
- `motor`: matar a última célula de um projeto remove o projeto do estado; matar uma
  célula não-última não remove.
- `motor`: criar célula em caminho inexistente ou sem permissão de escrita falha com
  erro claro e não altera o estado.

**Validação manual**

1. `n` — o formulário abre com PROJETO preenchido com o projeto focado.
2. Digite um caminho novo, `tab` completa e mostra quantas pastas casam. Escolha `bash`,
   dê um nome, `↵`.
3. O segundo projeto aparece como coluna. A coluna focada está larga; a outra virou tira
   com iniciais na vertical, contagem de células e indicador de Docker.
4. `←` `→` — a coluna vizinha engorda e a atual encolhe. `↑` `↓` andam entre células.
5. Digite `j`, `k`, `h`, `l` em NAVEGAR — nada acontece; essas letras não navegam.
6. `o` — a célula focada ocupa a tela. `shift` + arrastar seleciona texto só dela, sem
   pegar vizinho. `esc` volta ao mosaico.
7. `D` — pede confirmação. Na última célula do projeto, a confirmação avisa que o projeto
   sai da tela. Confirme: o projeto some, e o diretório continua intacto no disco.

---

### Fase 3 — os outros tipos de célula

`claude`, `cursor`, `logs`, `md`. Estados de turno com a heurística anti-alarme-falso.
`p`, `r`, `R`, `ctrl-r`, `espaço`, `1`…`9`.

**Testes automatizados obrigatórios**

- Estado de turno: alimentando a heurística com um fluxo sintético de bytes, spinner
  piscando e cursor se mexendo **não** disparam `respondeu`; trabalho seguido de silêncio
  dispara; pergunta bloqueante vira `aprovar` e não `respondeu`.
- `celula/md`: alterar o arquivo em disco atualiza a célula em até 1s; arquivo apagado
  vira estado de erro legível, não pânico.
- `celula/logs`: serviço parado deixa a célula em `parada` e ela engata quando o serviço
  sobe.
- `celula`: cada tipo implementa o contrato inteiro — teste de tabela que percorre o
  registro de tipos e falha se algum não responder a nascer, desenhar, receber tecla e
  informar estados.

**Validação manual**

1. Crie uma célula `claude` num projeto real. Ela sobe e a tela do Claude aparece.
2. `p` — manda um prompt sem entrar na célula. O Claude começa a trabalhar; o marcador
   vira trabalhando.
3. Ele termina: o marcador vira `⬤ RESPONDEU`, toca o aviso, sobe a notificação do
   sistema com o nome da célula e do projeto.
4. Peça algo que exija aprovação: o marcador vira `⏵ APROVAR`, com cor diferente de
   respondeu.
5. `espaço` de outro projeto — pula direto pra célula que chamou.
6. `R` — renomeie a célula. O nome muda na tela **e** a conversa dentro do Claude é
   renomeada. `ctrl-r` — a célula adota o nome que o Claude deu à conversa.
7. Repita 1, 2 e 6 com `cursor`.
8. Crie uma célula `md` apontando um arquivo. Peça pro Claude editar esse arquivo: o
   markdown renderizado se atualiza ao lado sozinho.
9. Mate o processo do Claude por fora (`kill`): a célula vira `✖ caiu`, avisa, e **não**
   reinicia sozinha. `r` sobe de novo retomando a conversa.

---

### Fase 4 — Docker

Painel do projeto, ações no serviço e na stack, log vira célula.

**Testes automatizados obrigatórios**

- `docker`: parsear a saída de `docker compose ps --format json` a partir de fixtures
  cobrindo serviço up, exited, sem porta e sem healthcheck.
- `docker`: detecção do arquivo de compose só na raiz do projeto, na ordem
  `docker-compose.yml`, `docker-compose.yaml`, `compose.yml`, `compose.yaml`; sem busca
  recursiva; projeto sem compose não ganha painel.
- `teclado`: o painel Docker não reusa nenhuma tecla com significado diferente do mapa
  global sem que isso esteja declarado como teclado próprio do painel.

**Validação manual**

Num projeto seu com Compose de verdade:

1. `d` — o painel abre listando os serviços com estado, porta, saúde e tempo de pé.
2. `↑` `↓` escolhem serviço — não executam ação nenhuma.
3. `s` para o serviço escolhido; `u` sobe de novo; `r` reinicia. A lista reflete cada
   mudança.
4. `U` sobe a stack inteira; `S` para tudo; `R` reinicia tudo.
5. `l` num serviço — o painel fecha e nasce uma célula `logs` daquele serviço no mosaico,
   com o log correndo.
6. Essa célula se comporta como qualquer outra: `D` mata, `R` renomeia, roda do mouse
   rola, `o` põe em tela cheia.
7. Não existe nenhuma ação que apague volume ou derrube com `-v` em lugar nenhum do painel.

---

### Fase 5 — serviço e recuperação

Unidade systemd de usuário, reconstituição depois de queda, notificação pelo motor,
badge de consumo.

**Testes automatizados obrigatórios**

- `motor`: matar o motor e subir de novo a partir do estado desejado reconstrói projetos
  e células com o mesmo tipo, nome e posição.
- `motor`: célula `claude` reconstituída **não** dispara prompt nenhum — teste que falha
  se qualquer byte for escrito no PTY do agente durante a reconstituição.
- `motor`: Docker não sobe sozinho na reconstituição.
- `motor/quota`: badge some quando o arquivo está velho; muda de faixa acima de 80%;
  ausência do arquivo não quebra nada.

**Validação manual**

1. `systemctl --user enable --now tesseract` — o serviço sobe.
2. Deixe 2 projetos com 4 células, sendo 2 `claude` com conversa em andamento.
3. Feche a tela com `q`. `systemctl --user status tesseract` mostra o serviço vivo.
4. Peça algo a um Claude por `p`, feche a tela, espere ele terminar: **a notificação do
   sistema chega com a tela fechada.**
5. No PowerShell do Windows: `wsl --shutdown`. Espere. Reabra a WSL.
6. `tess` — a mesma grade, nas mesmas posições, com os mesmos nomes. As células `bash`
   têm o histórico anterior rolável. As `claude` estão **paradas**, com a conversa
   retomada e nenhum trabalho novo iniciado.
7. A stack Docker continua onde estava — não subiu sozinha.

---

### Fase 6 — acabamento

Lista com prévia, busca `/`, ajuda `?`, os cinco comandos de linha, instalação.

**Testes automatizados obrigatórios**

- `tela`: lista e mosaico, alimentados com o mesmo estado, mostram os mesmos projetos,
  células e marcadores. Esse teste é a garantia mecânica da regra 3.
- `motor/historico`: busca devolve resultado correto em arquivo grande, incluindo termo
  que atravessa o limite de rotação.
- `cmd/tess`: cada comando de linha responde e sai com código correto; `tess` sem motor
  rodando sobe o motor.

**Validação manual**

1. `v` — alterna pra lista. Os mesmos projetos, células e marcadores do mosaico, agora em
   texto, com prévia ao vivo da célula selecionada à direita.
2. Toda tecla da fase 2 e 3 faz **a mesma coisa** na lista: `n`, `D`, `o`, `p`, `R`, `r`,
   `espaço`, setas.
3. `/` — busca no histórico da célula focada e leva até a ocorrência.
4. `?` — a ajuda lista todas as teclas, separadas por modo, sem nenhuma que não exista.
5. Feche e reabra: a tela escolhida (lista ou mosaico) foi lembrada.
6. `tess status`, `tess novo <dir>`, `tess stop`, `tess reset` — cada um faz o que a spec
   §9 diz.
7. Redimensione a janela do terminal em cada tela: nada quebra, nada renderiza torto.

---

## Fora do escopo

Não construa, mesmo que pareça natural:

- Célula de git (árvore local, diff) e de PR do GitHub.
- Auto-yes: aprovação automática de pedidos do agente, e daemon que responde por você.
- Paleta de comandos escritos.
- Reordenar célula ou projeto. A ordem é a de criação.
- Editar markdown. A célula `md` só lê.
- Temas, configuração visual, qualquer coisa de aparência além do que a spec descreve.
- Rodar fora da WSL.
- Reaproveitar código do fork.

Também não: abstração para um caso de uso só, interface com uma implementação só,
configuração para valor que nunca muda, reescrita de algo que já passou no portão.

## Restrições

- Interface, mensagem de erro e ajuda em português. Código, identificador e comentário
  em português também, seguindo os nomes da spec (`celula`, `motor`, `tela`, `projeto`).
- Commits em pt-br, subject único sem corpo, só os prefixos `feat:`, `fix:`, `refactor:`,
  sem escopo, sem rodapé de co-autoria.
- `go build ./... && go vet ./... && go test ./...` verde antes de qualquer entrega.
- Teto do histórico por célula: 5 MB, com descarte do começo, configurável.
- Se `charmbracelet/x/vt` não der conta de algum agente, troque por `hinshun/vt10x` ou
  por emulação própria do subconjunto necessário. A troca não pode tocar nada fora de
  `internal/celula/`.
- Nenhuma ação destrutiva no Docker em lugar nenhum: sem `down -v`, sem apagar volume.
- O humano não lê código. O relatório de fim de fase descreve comportamento observável,
  não implementação.

## Critério de pronto

As seis fases com portão manual aprovado, `go test ./...` verde, e o serviço sobrevivendo
a um `wsl --shutdown` com a grade intacta e nenhum agente tendo trabalhado sozinho.

---

## Decisões tomadas sem confirmação do humano

Estão valendo. Se alguma estiver errada, ele corrige antes da fase 1 começar.

1. "Recursivo até estar pronto" é loop autônomo dentro da fase e portão humano entre
   fases — a parte visual de uma tela de terminal não tem como ser validada sem olho.
2. Auto-yes fica fora do V1 e a célula `md` só lê, conforme a spec.
3. Notificação sai por `powershell.exe`, já que `wsl-notify-send.exe` não está instalado.
4. Código e identificadores em português, seguindo o vocabulário da spec.
5. A fase 1 entrega tela cheia com uma célula antes do mosaico, para haver algo rodando
   cedo em vez de duas fases sem tela.
6. Teto de histórico de 5 MB por célula — número escolhido aqui, não vem da spec.
