<p align="center">
  <img src="themes/logo-hero.svg" alt="Tesseract — the mosaic doesn't fall apart" width="720">
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/licen%C3%A7a-MIT-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6" alt="MIT license"></a>
  <a href="https://github.com/AndreLuizMMS/tesseract/releases"><img src="https://img.shields.io/badge/vers%C3%A3o-0.1.0-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6" alt="version 0.1.0"></a>
  <a href="#instalar"><img src="https://img.shields.io/badge/plataforma-Linux%20%7C%20WSL%20%7C%20macOS-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6" alt="Linux, WSL, macOS"></a>
</p>

<br>

Several AI agents running side by side in a single grid. Claude Code, Cursor CLI, shells,
Docker logs and markdown, each in a living cell, all visible at the same time — no tabs,
no switching, no finding out too late that one of them is stuck waiting for a yes.
Underneath, an engine that's a service on your account: you close the screen and the work
keeps going, the machine reboots and the grid comes back in the same positions, with the
conversations picked back up.

<p align="center">
  <img src="docs/img/mosaico.svg" alt="The Tesseract mosaic: three projects, five cells, one stuck waiting for approval" width="1000">
</p>

`⬤ answered` has something to read. `⏵ approve` **won't move** without you — and it's the
only signal that turns into a solid bar across the whole line, blinking, because urgency
here is filled area, not color. Works from a distance, out of the corner of your eye, and
with `NO_COLOR=1`.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/AndreLuizMMS/tesseract/main/install.sh | bash
```

One line. The installer downloads Go if the machine doesn't have a usable one, compiles
the `ts` command, installs the user service and starts the engine. **Updating is running
the same line.**

Then, inside any project:

```bash
ts
```

## Shortcuts

Moving around is only with the arrows — no letter touches the grid. `↵` enters the cell and
**every** key becomes hers; `ctrl-l` gives back the keyboard. `?` opens the map on screen.

**[The full keyboard map →](docs/atalhos.md)**

## Documentation

| | |
|---|---|
| [Manual](docs/manual.md) | projects, cells, the two modes, Docker, configuration |
| [Shortcuts](docs/atalhos.md) | the whole keyboard and the command line |

## License

MIT.
