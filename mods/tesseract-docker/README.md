# tesseract-docker

Mod do Claude Code: o que só um mod consegue fazer pelo Docker Compose do
projeto. Não tem painel: o painel é o do mosaico do Tesseract (tecla `d`),
alcançável de qualquer célula. Funciona em célula e em qualquer `claude`.

## O que faz

- **Guarda no Bash**: nega o que apaga volume, imagem ou cache, com mensagem
  que diz o que usar no lugar em cada caso. Se a guarda falhar, o comando não
  roda.
- **Tools para o Claude**: `services` (lista serviços com estado, porta,
  health e uptime) e `restart` (reinicia um serviço que o compose listou). Só
  existem em projeto com compose.
- **`/dlog <serviço>`**: põe as últimas 20 linhas do log do serviço no
  prompt, sem enviar, sem cor nem caractere de controle. Você completa a
  frase e manda. Só aceita nome que o compose listou.
- **`/dlog`** sem argumento lista os serviços como texto.

Exemplo: `/dlog api` deixa no prompt

````
Log of the `api` service, last 20 lines:
```
…
```
````

e avisa `Not sent: 20 log lines of api are in the prompt.`

## O que foi para o mosaico

Subir, parar, reiniciar e reconstruir serviço ou stack, log ao vivo (vira
célula) e shell no contêiner: painel `d` do mosaico
(`internal/screen/docker.go`). O comando `/docker`, o painel e a faixa de
aviso deste mod deixaram de existir. O arquivo de compose é escolhido pela
mesma regra dos dois lados: raiz primeiro, depois pastas de primeiro nível,
nunca um arquivo de produção.

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

## Arquivos

- `hooks/register.ts`: guarda no `tool.call`, tools, `/dlog`.
- `hooks/guard.ts`: parser do comando de shell e a regra destrutiva.
- `hooks/compose.ts`: escolha do arquivo e leitura dos serviços.
- `tests/`: `claude plugin test .`

## Rodar

```sh
claude --plugin-dir mods/tesseract-docker
claude plugin validate mods/tesseract-docker --strict
claude plugin test mods/tesseract-docker
```
