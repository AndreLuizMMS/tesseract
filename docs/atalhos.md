# Atalhos

O mapa inteiro sai do próprio código: nenhuma tecla que não exista aparece aqui.

## Teclado

**Andar — só setas, nenhuma letra**

| Tecla | Ação |
|---|---|
| `↑` `↓` | célula anterior / próxima, atravessando projeto |
| `←` `→` | projeto anterior / próximo |
| `espaço` | pula para a próxima célula que pede atenção, atravessando projeto |
| `1`…`9` | vai direto para o projeto N |
| `tab` | troca a aba da célula: claude, cursor, shell, md (`shift-tab` volta) |

**Teclado e tela**

| Tecla | Ação |
|---|---|
| `↵` | entra em DIGITAR na célula focada |
| `ctrl-l` | devolve o teclado ao aplicativo |
| `o` | célula focada em tela cheia (liga e desliga) |
| `v` | alterna mosaico ↔ lista |

**Criar, matar, nomear**

| Tecla | Ação |
|---|---|
| `n` | criar — um formulário só, que começa na sua casa e não pergunta o tipo |
| `r` | retoma célula parada, ou sobe célula caída |
| `D` | mata a célula focada — sempre confirma |
| `R` | renomeia a célula **e propaga o nome para dentro do agente** |
| `ctrl-r` | adota na célula o nome que o agente deu à conversa |

**Agir e ler**

| Tecla | Ação |
|---|---|
| `p` | manda prompt para a célula focada sem entrar nela |
| `d` | abre o painel Docker do projeto focado |
| `ctrl-e` | abre o diretório do projeto na IDE configurada (`cursor /caminho`) |
| roda do mouse | rola o histórico da célula |
| arrastar com o mouse | marca um trecho da célula e **copia ao soltar** |
| `/` | busca no histórico da célula focada |
| `esc` | sai da rolagem / fecha o que estiver aberto |
| `?` | ajuda |
| `q` | fecha a tela — o motor continua rodando |

## Comandos

```
ts                 abre a tela, subindo o motor se preciso
ts novo <dir>      adiciona um projeto sem abrir a tela
ts status          estado do motor e resumo de projetos e células
ts stop            desliga o motor e todas as células
ts reset           apaga o estado salvo e derruba tudo, preservando a configuração
```

## Recuperação

`wsl --shutdown` mata todos os processos. Quando a WSL volta, o serviço sobe sozinho e
reconstrói a grade:

| Tipo | O que acontece |
|---|---|
| `sessao` | volta na mesma aba, com a conversa de cada agente reatada e **parada**. Nenhum prompt é disparado |
| aba `bash` | shell novo e limpo; o histórico anterior fica rolável acima da linha de queda |
| `logs` | volta a acompanhar o serviço; se a stack estiver parada, engata sozinha quando ele subir |
| `md` | relê o arquivo |
| Docker | **não sobe sozinho**. Subir stack é decisão sua |

O risco que a recuperação automática cria — agente trabalhando sem ninguém na frente da
tela — é cortado na raiz: **reatar a conversa nunca dispara trabalho**. Existe um teste que
falha se qualquer byte for escrito no teclado do agente durante a reconstituição.

## Aviso

Quem notifica é o motor, não a tela. Som e notificação do sistema com o nome da célula e do
projeto, mesmo com a tela fechada. Na WSL, o toast sai pelo PowerShell do Windows; se você
tiver `wsl-notify-send.exe` ou `notify-send`, eles são usados no lugar. Os dois avisos são
desligáveis, separadamente.
