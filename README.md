<p align="center">
  <img src="themes/logo-hero.svg" alt="Tesseract — o mosaico não desmonta" width="720">
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/licen%C3%A7a-MIT-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6" alt="licença MIT"></a>
  <a href="https://github.com/AndreLuizMMS/tesseract/releases"><img src="https://img.shields.io/badge/vers%C3%A3o-0.1.0-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6" alt="versão 0.1.0"></a>
  <a href="#instalar"><img src="https://img.shields.io/badge/plataforma-Linux%20%7C%20WSL%20%7C%20macOS-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6" alt="Linux, WSL, macOS"></a>
</p>

<br>

Vários agentes de IA rodando lado a lado numa grade só. Claude Code, Cursor CLI, shells,
logs de Docker e markdown, cada um numa célula viva, todos visíveis ao mesmo tempo — sem
aba, sem alternar, sem descobrir tarde demais que um deles está travado esperando um sim.
Por baixo, um motor que é serviço da sua conta: você fecha a tela e o trabalho continua, a
máquina reinicia e a grade volta nas mesmas posições, com as conversas reatadas.

```
 ⧉ TESSERACT   ⬤ 1   ⏵ 1                                NAVEGAR
━━ DOXAR-API  /home/dev/doxar-api ─────────────────────────────────────────────────────────────────────────── ⬤1  ● 4/5
┌  claude  cursor  bash  refatora auth ─────── ⬤ RESPONDEU ┐┌  claude  cursor  bash  testes ──────────── ▸ TRABALHANDO ┐
│Movi a validação de token                                 ││$ go test ./...                                           │
│pro guard.                                                ││ok                                                        │
│Qual você prefere?                                        ││                                                          │
└──────────────────────────────────────────────────────────┘└──────────────────────────────────────────────────────────┘
── CORTZ-WEB  /home/dev/cortz-web ────────────────────────────────────────────────────────────────────────────────── ⏵1
┌  claude  cursor  bash  fix nav ─────────────────────────────────────────────────────────────────────────── ⏵ APROVAR ┐
│posso mexer no Header?                                                                                                │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
── API-LEGADO  /home/dev/api-legado ────────────────────────────────────────────────────────────────────────────────────
┌ md · spec-m7.md ─────────────────────────────────────────────────────────────────────────────────────────── ○ PARADA ┐
│# Módulo 7                                                                                                            │
└──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
 ←→ célula   ↑↓ projeto   tab aba   ↵ digitar   v lista   n criar   d docker   ? ajuda
```

`⬤ respondeu` tem algo para ler. `⏵ aprovar` **não anda** sem você — e é o único sinal que
vira barra sólida na linha inteira, piscando, porque urgência aqui é área preenchida, não
cor. Funciona de longe, no canto do olho e com `NO_COLOR=1`.

## Instalar

```bash
curl -fsSL https://raw.githubusercontent.com/AndreLuizMMS/tesseract/main/install.sh | bash
```

Uma linha. O instalador baixa o Go se a máquina não tiver um que sirva, compila o comando
`ts`, instala o serviço de usuário e sobe o motor. **Atualizar é rodar a mesma linha.**

Depois, dentro de qualquer projeto:

```bash
ts
```

## Atalhos

Andar é só com as setas — letra nenhuma mexe na grade. `↵` entra na célula e **toda** tecla
passa a ser dela; `ctrl-l` devolve o teclado. `?` abre o mapa na tela.

**[O mapa completo do teclado →](docs/atalhos.md)**

## Documentação

| | |
|---|---|
| [Manual](docs/manual.md) | projetos, células, os dois modos, Docker, configuração |
| [Atalhos](docs/atalhos.md) | o teclado inteiro e os comandos de linha |
| [Tema](docs/tema.md) | a paleta, a marca e como vesti-la no resto da mesa |
| [Arquitetura](docs/arquitetura.md) | como o motor sobrevive à tela e por que isso é pouco código |

## Licença

MIT.
