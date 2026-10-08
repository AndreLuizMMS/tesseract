# tesseract-docker

Mod do Claude Code: o Docker Compose do projeto dentro da sessão. Feito para
rodar em célula do Tesseract (80 colunas, sem mouse), mas funciona em
qualquer `claude`.

## O que faz

- **`/docker`** abre o painel: serviços do compose do projeto, estado, porta,
  health e uptime. Relê a cada 3 s enquanto está aberto.
- **Ações que nunca apagam nada**: subir, parar e reiniciar, por serviço e
  pela stack. Só rodam pela tecla delas; o foco não passa por elas.
- **Cauda de log** de um serviço dentro do painel, e `p` manda as últimas 20
  linhas ao prompt, sem enviar.
- **Tools para o Claude**: `services` (lista) e `restart` (reinicia um
  serviço que o compose listou). Só existem em projeto com compose.
- **Guarda no Bash**: nega o que apaga volume, imagem ou cache, com mensagem
  que diz o que usar no lugar em cada caso. Se a guarda falhar, o comando não
  roda.
- **Faixa de aviso**, só quando um serviço quebrou ou adoeceu:
  `✖ 1 CRASHED · db · /docker`. Stack saudável ou parada não desenha nada, e
  a faixa some com o painel aberto. O resumo (`2/3`) fica no cabeçalho do
  painel: a linha de status do host desenha qualquer texto como aviso
  (`⚠ plugin: …` em amarelo), então o mod não a usa.

## Teclas do painel

| Tecla | Efeito |
|---|---|
| `↑` `↓` / `Tab` | escolhe o serviço; a lista rola com o foco |
| `PgUp` `PgDn` | anda uma janela |
| `Enter` | cauda do log do serviço escolhido (no log: volta) |
| `u` `s` `r` | sobe, para, reinicia o serviço escolhido |
| `l` | cauda do log (some da linha de teclas em célula estreita; `Enter` faz o mesmo) |
| `1` `2` `3` | sobe, para, reinicia a stack inteira |
| `p` (no log) | manda o trecho ao prompt |
| `b` (no log) | volta à lista |
| `Esc` | fecha |

Enter nunca muda estado: numa linha abre o log, no log volta. O host guarda
o anel de foco pela posição do elemento, não pela chave; por isso toda tela
do painel começa com 12 paradas inofensivas (as linhas e, no que sobra,
botões vazios) e as ações vêm depois, e o mod recusa o foco em qualquer ação.

Tecla sem ação: o host não deixa um painel consumir teclas, só dar tecla a
um botão, e uma tecla que nenhum botão tem devolve o teclado ao prompt e é
escrita lá. Por isso cada tela do painel tem um botão invisível e inerte para
toda letra e dígito sem uso. Pontuação e espaço não podem ser tecla de
botão: esses ainda vazam para o prompt.

## Célula baixa ou estreita

O painel desenha só o que cabe nas linhas que a célula dá (numa célula de 16
linhas sobram 3 de corpo): a lista primeiro, uma linha de teclas sempre à
vista, e cabeçalho, nomes de coluna e dica extra só quando sobra linha. `▴` e
`▾` na borda da lista dizem que há mais acima ou abaixo.

Abaixo de 68 colunas as teclas da stack vão para uma segunda linha; se nem
essa linha existe (corpo com menos de 5 linhas), ficam só as do serviço, e
`1` `2` `3` voltam quando a célula crescer.

Sem leitura válida (Docker fora do ar, compose com erro) o painel mostra só
o erro e o que fazer, nunca a lista da leitura anterior.

## Como a guarda julga

O comando é quebrado em comandos simples e só o programa e seus argumentos
contam. `echo "docker compose down -v"`, `grep 'system prune' compose.yml` e
`git commit -m '... docker compose down -v ...'` passam;
`cd infra && sudo docker compose -f x.yml down --volumes` é negado.

O que ela nega:

| Comando | Por quê |
|---|---|
| `compose down -v` / `--volumes` / `-v=true` | apaga os volumes da stack |
| `compose down --rmi` | apaga as imagens da stack |
| `compose rm -v`, `docker rm -v`, `container rm -v` | apaga volumes anônimos |
| `compose up -V` / `--renew-anon-volumes` | recria volumes anônimos |
| `volume rm` / `remove` / `prune` | apaga volumes |
| `system prune`, `container` / `image` / `builder` / `buildx` / `network prune` | apaga recursos de todos os projetos da máquina |
| o mesmo com `docker-compose`, `podman`, `podman compose`, `podman-compose`; `podman system reset` | idem |

Fail-closed em dois pontos:

- **Texto entregue a um shell é lido como outro script**: `bash -c`, `sh -c`,
  `su -c`, `eval`, `watch`, `script -c`, `env -S`, `| sh`, `| bash`,
  `| xargs`, `<<<`, heredoc, `<(...)`, `$(...)` e crases (inclusive dentro
  de heredoc sem aspas no delimitador). `sudo`, `doas`, `env`, `nohup`,
  `time`, `timeout`, `nice`, `xargs` e `find -exec` são atravessados, e um
  `docker` no meio do argv de qualquer outro programa também é julgado.
- **Expansão que a guarda não resolve, num subcomando perigoso, é negada**:
  `docker compose down $(echo -v)`, `down $FLAGS`, `down -{v,}`,
  `docker rm $(docker ps -aq)`, `... | xargs docker rm`, `docker compose
  $ACTION`. A recusa diz o motivo e pede o comando escrito por extenso.
  `$'docker'` é lido como `docker`. Expansão entre aspas no meio de um nome
  (`docker rm "api-$ID"`) passa: não tem como virar flag.

Limites conhecidos (é uma rede contra o engano comum, não uma sandbox):

- Script em arquivo (`./reset.sh`, `sh reset.sh`, `curl ... | sh`), alias e
  função definidos fora do comando.
- Outro interpretador: `python3 -c "subprocess.run(['docker', ...])"`,
  `node -e`, `perl -e`, `awk 'BEGIN{system(...)}'`.
- Texto codificado (`base64 -d <<< ... | sh`) e `bash -c "$CMD"` com o
  script inteiro numa variável.
- Comando em outra máquina ou contêiner (`ssh host "docker ..."`,
  `docker exec x sh -c "docker ..."`).
- A API do Docker por fora do CLI (`curl --unix-socket /var/run/docker.sock`).
- Falso positivo aceito: um `echo "<docker destrutivo>"` no mesmo comando de
  um `| sh` é negado mesmo que não seja ele quem alimenta o shell.

## Desenhado pelo host

Fora do alcance do mod, e por isso fora da identidade Tesseract: o anel de
foco invertido do botão, a letra da tecla (`u:`) e os links de markdown em
azul ANSI 12, e a moldura do painel na cor padrão do terminal.

## Arquivos

- `hooks/register.tsx`: comando, painel, faixa de aviso, tools, guarda no `tool.call`.
- `hooks/guard.ts`: parser do comando de shell e a regra destrutiva.
- `hooks/compose.ts`: escolha do arquivo, leitura dos serviços, ações.
- `hooks/list.ts`: janela da lista que rola com o foco e contas de largura.
- `hooks/tokens.ts`: cores e glifos da identidade Tesseract; respeita `NO_COLOR`.
- `tests/`: `claude plugin test .`

## Rodar

```sh
claude --plugin-dir mods/tesseract-docker
claude plugin validate mods/tesseract-docker --strict
claude plugin test mods/tesseract-docker
```
