# Arquitetura

## Como isso não vira um Frankenstein

Três regras duras, e cada uma tem um teste que quebra se ela for violada:

1. **Tipo de célula é uma peça fechada.** Um tipo declara como nasce, como se desenha, o que
   faz com uma tecla e quais estados tem. Adicionar um tipo é escrever um arquivo em
   `internal/celula/` e uma linha no registro — o mosaico, a lista, os atalhos e o motor não
   são tocados. A sessão com abas é só mais um tipo, feito dos outros.
2. **Feature nova não ganha tecla nova.** O mapa de teclas mora num arquivo só. Um teste
   percorre o mapa inteiro e falha se a mesma tecla tiver dois significados no mesmo modo,
   se qualquer tecla ficar sem texto de ajuda, ou se alguma letra andar pela grade.
3. **A tela não decide nada.** Ela desenha o que o motor manda e devolve tecla. Um teste
   alimenta a lista e o mosaico com o mesmo estado e exige que os dois mostrem os mesmos
   projetos, células e marcadores.

## Como o código se divide

```
cmd/tess/              o comando ts: sobe o motor ou conecta nele
internal/
  motor/               estado, projetos, ciclo de vida, persistência, avisos
    historico/         gravação, rotação e busca por célula
  celula/              um arquivo por tipo, sobre um contrato só
  docker/              lê o compose e age na stack, nunca destrutivamente
  protocolo/           contrato motor ↔ tela, JSON por linha em socket unix
  teclado/             o mapa de teclas, num arquivo só
  tela/                mosaico, lista, painel, formulário
systemd/               a unidade de usuário que mantém o motor de pé
```

O motor mantém **a tela interna de cada célula em memória** — ele já sabe o que está escrito
em cada uma, sem perguntar a ninguém a cada quadro. É isso que mantém o mosaico fluido com
quinze células vivas.

## Stack

Go 1.25 e seis dependências, nenhuma a mais:

| Uso | Módulo |
|---|---|
| PTY | `github.com/creack/pty` |
| Emulação de terminal | `github.com/charmbracelet/x/vt` |
| Interface de terminal | `charm.land/bubbletea/v2` |
| Estilo | `charm.land/lipgloss/v2` |
| Markdown renderizado | `charm.land/glamour/v2` |
| Arquivo mudou | `github.com/fsnotify/fsnotify` |

Docker sai por `docker compose`, notificação por `powershell.exe`, comunicação por socket
unix com JSON por linha, estado em arquivo, testes com a biblioteca padrão. Sem SDK, sem
gRPC, sem banco, sem framework de teste.

## Desenvolver

```bash
make gate      # build + vet + testes, que é o portão de tudo
make build     # compila ./ts aqui mesmo
make instalar  # instala o comando e o serviço
```

Os testes sobem shells de verdade, stacks de Docker de verdade e a tela dentro de um
pseudo terminal de verdade — os que dependem de Docker se pulam sozinhos quando ele não
está disponível.
