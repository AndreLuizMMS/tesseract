// tesseract-ide: abre o projeto no IDE, de dentro da sessão.
//
// - `/open` (o nome `/ide` já é do Claude Code) detecta os IDEs (no WSL e, dentro do WSL, os do Windows mesmo
//   fora do PATH) e abre o seletor. Enter abre o projeto.
// - `/open file` abre o último arquivo que o Claude editou, na linha, no IDE
//   usado da última vez; sem um lembrado, cai no seletor.
// - O IDE é lançado destacado (ides.ts, DETACH_SCRIPT): a chamada volta em
//   meio segundo e o IDE segue vivo depois que a sessão fecha.

import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { Ide, IdePanel } from '../types'
import type { Probe, Target, Way } from './ides'
import { CANDIDATES, CMD_EXE, DETACH_SCRIPT, detectIdes, launchOf, lineOf, parseDistro, windowsTarget } from './ides'
import { clampTop, cut, moveBy } from './list'
import { GLYPH, ink, setMono } from './tokens'

const PANE = 'tesseract-ide'
const PROBE_TIMEOUT_MS = 8_000
const LAUNCH_TIMEOUT_MS = 15_000
// Chave do `$.store`: o IDE escolhido da última vez, entre sessões.
const REMEMBERED = 'ide'

const EMPTY: IdePanel = { ides: [], isLoading: false, error: '', ring: -1, top: 0, wantsFile: false }
// Linhas da lista na tela, no máximo.
const MAX_ROWS = 8

const panel = atom({ plugin: 'tesseract-ide', key: 'panel' } as const, EMPTY)
const lastEdit = atom({ plugin: 'tesseract-ide', key: 'lastEdit' } as const, null)

// Quantos IDEs o último desenho mostrou e a partir de qual: a janela que o
// foco percorre.
let shownRows = 1
let shownTop = 0

const run = async ($: EngineInterface, argv: readonly string[]): Promise<{ isOk: boolean; out: string }> => {
  try {
    const ran = await $.process.run(argv, { timeoutMs: PROBE_TIMEOUT_MS })

    return { isOk: ran.exitCode === 0, out: ran.stdout }
  } catch {
    return { isOk: false, out: '' }
  }
}

// As perguntas que a detecção faz à máquina, respondidas pelo host.
async function probeOf($: EngineInterface): Promise<Probe> {
  const users = (await $.fs.list('/mnt/c/Users').catch(() => []))
    .filter(entry => entry.kind === 'dir')
    .map(entry => entry.name)
  const hasWindows = await $.fs.exists(CMD_EXE).catch(() => false)

  return {
    users,
    which: async id => {
      const found = await run($, ['which', id])
      if (!found.isOk) return ''

      // O destino real: um link em /usr/local/bin pode levar ao lançador do
      // Windows, e aí a instalação é a do Windows.
      const real = await run($, ['realpath', found.out.trim()])

      return real.isOk && real.out.trim() !== '' ? real.out.trim() : found.out.trim()
    },
    mtime: path => $.fs.stat(path).then(stat => stat.mtimeMs, () => undefined),
    where: async id => hasWindows && (await run($, [CMD_EXE, '/c', 'where', id])).isOk,
  }
}

// O nome da distro WSL: a variável do ambiente, ou o que o Windows responde.
async function distroOf($: EngineInterface): Promise<string> {
  const named = await $.env.get('WSL_DISTRO_NAME')
  if (named !== undefined && named !== '') return named

  return parseDistro((await run($, [CMD_EXE, '/c', 'wsl.exe', '-l', '-q'])).out)
}

// Uma janela do IDE Windows já aberta: o CLI remoto e o socket que ainda
// responde. IDEs deixam para trás o socket de cada janela fechada, então só
// um teste de conexão separa a viva das mortas.
async function liveWindow($: EngineInterface, id: string): Promise<Way | undefined> {
  const serverDir = CANDIDATES.find(one => one.id === id)?.serverDir ?? `.${id}-server`
  const home = await $.env.get('HOME')
  const runtime = await $.env.get('XDG_RUNTIME_DIR')
  if (home === undefined || runtime === undefined || runtime === '') return undefined

  const builds = await $.fs.list(`${home}/${serverDir}/bin`).catch(() => [])
  const clis = await Promise.all(
    builds.map(async build => {
      const cli = `${home}/${serverDir}/bin/${build.name}/bin/remote-cli/${id}`
      const stat = await $.fs.stat(cli).catch(() => undefined)

      return stat === undefined ? undefined : { cli, mtimeMs: stat.mtimeMs }
    }),
  )
  const cli = clis.filter(one => one !== undefined).sort((a, b) => b.mtimeMs - a.mtimeMs)[0]?.cli
  if (cli === undefined) return undefined

  const sockets = (await $.fs.list(runtime).catch(() => []))
    .filter(entry => /^vscode-ipc-.*\.sock$/.test(entry.name))
    .map(entry => `${runtime}/${entry.name}`)
  for (const socket of sockets.slice(0, 12)) {
    const isLive = await $.http.fetch('http://localhost/', { socketPath: socket }).then(() => true, () => false)
    if (isLive) return { kind: 'window', cli, socket }
  }

  return undefined
}

// Abre o alvo no IDE. Devolve '' quando abriu, ou o motivo de não ter aberto.
async function launch($: EngineInterface, ide: Ide, wantsFile: boolean): Promise<string> {
  const project = await $.session.root()
  const edit = await read($, lastEdit)
  const target: Target = wantsFile && edit !== null ? { project, file: edit } : { project }

  let way: Way = { kind: 'wsl' }
  if (ide.location === 'windows') {
    const window = await liveWindow($, ide.id).catch(() => undefined)
    if (window !== undefined) way = window
    else {
      const distro = await distroOf($)
      if (distro === '') return 'Could not find the WSL distro name. Set WSL_DISTRO_NAME and retry.'

      const candidate = CANDIDATES.find(one => one.id === ide.id)
      const exe = candidate === undefined ? '' : await windowsTarget(candidate, await probeOf($))
      if (exe === '') return `${ide.label} is no longer installed on the Windows side.`

      way = exe === ide.id ? { kind: 'path', distro } : { kind: 'exe', exe, distro }
    }
  }

  const { argv, env } = launchOf(ide.id, way, target)
  // Sem DISPLAY um IDE Linux nasce sem tela e morre calado.
  const hasDisplay = ((await $.env.get('DISPLAY')) ?? '') !== ''
  if (!hasDisplay && (await $.fs.exists('/tmp/.X11-unix/X0').catch(() => false))) env.DISPLAY = ':0'

  try {
    const ran = await $.process.run(['sh', '-c', DETACH_SCRIPT, 'sh', ...argv], {
      cwd: project,
      env,
      timeoutMs: LAUNCH_TIMEOUT_MS,
    })
    if (ran.exitCode === 0) return ''

    const said = `${ran.stdout}\n${ran.stderr}`.split('\n').map(line => line.trim()).find(line => line !== '')

    return `${ide.label} did not open: ${said ?? `exit ${ran.exitCode}`}`
  } catch {
    return `${ide.label} did not open: the launcher could not start.`
  }
}

// Abre, lembra a escolha e devolve o teclado ao prompt; a falha fica na tela.
async function open($: EngineInterface, ide: Ide, wantsFile: boolean): Promise<string> {
  await update($, panel, held => ({ ...held, error: '', isLoading: true }))
  const failure = await launch($, ide, wantsFile)
  await update($, panel, held => ({ ...held, error: failure, isLoading: false }))
  if (failure !== '') return failure

  const edit = await read($, lastEdit)
  const project = await $.session.root()
  const what =
    wantsFile && edit !== null
      ? `${edit.path.startsWith(`${project}/`) ? edit.path.slice(project.length + 1) : edit.path}:${edit.line}`
      : (project.split('/').pop() ?? project)
  await $.store.set(REMEMBERED, { id: ide.id, location: ide.location })
  await $.ui.close({ id: PANE })
  $.ui.toast(`Opened ${what} in ${ide.label} (${ide.location}).`)

  return ''
}

async function showPicker($: EngineInterface, wantsFile: boolean): Promise<string | undefined> {
  await update($, panel, held => ({ ...held, isLoading: true, error: '', wantsFile }))
  const opened = await $.ui.open({ id: PANE, title: 'OPEN IN', focus: true, closeOnEscape: true, rows: 8 })
  const ides = await detectIdes(await probeOf($)).catch(() => [])
  const remembered = (await $.store.get(REMEMBERED).catch(() => undefined)) as Partial<Ide> | undefined
  // O lembrado sobe para o topo: é onde o anel de foco nasce.
  const first = ides.find(ide => ide.id === remembered?.id && ide.location === remembered?.location)
  const ordered = first === undefined ? ides : [first, ...ides.filter(ide => ide !== first)]
  await update($, panel, held => ({ ...held, ides: ordered, isLoading: false, top: 0 }))

  return opened.isPlaced ? undefined : `IDE picker is waiting for room: ${opened.reason}`
}

// Guarda o arquivo e a linha do que o Claude acabou de editar.
async function remember($: EngineInterface, filePath: unknown, written: unknown): Promise<void> {
  if (typeof filePath !== 'string' || filePath === '') return

  const text = typeof written === 'string' && written !== '' ? await $.fs.read(filePath).catch(() => '') : ''
  await update($, lastEdit, () => ({
    path: filePath,
    // O começo do trecho novo; só as primeiras linhas, que bastam para achar.
    line: lineOf(text, typeof written === 'string' ? written.split('\n').slice(0, 3).join('\n') : ''),
  }))
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    setMono(await $.env.get('NO_COLOR').catch(() => undefined))
    await $.command.register({
      name: 'open',
      description: 'Open the project in a detected IDE; /open file opens the last file Claude edited, at the line',
      argumentHint: '[file]',
    })

    return started
  })

  on('command.run', { command: 'open' }, async ($, e) => {
    const wantsFile = e.args.trim() === 'file'
    if (wantsFile && (await read($, lastEdit)) === null) {
      return { text: 'Claude has not edited a file in this session yet. /open opens the project.' }
    }
    if (wantsFile) {
      const held = (await $.store.get(REMEMBERED).catch(() => undefined)) as Partial<Ide> | undefined
      const candidate = CANDIDATES.find(one => one.id === held?.id)
      if (candidate !== undefined && (held?.location === 'wsl' || held?.location === 'windows')) {
        const failure = await open($, { id: candidate.id, label: candidate.label, location: held.location }, true)

        return failure === '' ? {} : { text: `${failure} /open picks another IDE.` }
      }
    }
    const waiting = await showPicker($, wantsFile)

    return waiting === undefined ? {} : { text: waiting }
  })

  // Depois de cada edição: só observa, nunca barra.
  on('tool.call', { tool: 'Edit' }, async ($, e, next) => {
    const ran = await next(e)
    if (ran.deny === undefined && ran.isError !== true) await remember($, e.file_path, e.new_string)

    return ran
  }).catch(($, e, next) => next(e))

  on('tool.call', { tool: 'Write' }, async ($, e, next) => {
    const ran = await next(e)
    if (ran.deny === undefined && ran.isError !== true) await remember($, e.file_path, '')

    return ran
  }).catch(($, e, next) => next(e))

  // O foco anda só pelos IDEs; o alvo troca por tecla (`p`, `f`). Passar do
  // último visível (a parada "project") desce a janela; qualquer outra
  // parada que não seja linha é a volta do anel pelo fim: sobe.
  on('ui.focus', { requestId: PANE }, async ($, e, next) => {
    const key = e.element
    const now = await read($, panel)
    const land = async (ring: number): Promise<void> => {
      if (now.ring !== ring || now.top !== shownTop) await update($, panel, held => ({ ...held, ring, top: shownTop }))
    }
    if (key === undefined) {
      await land(-1)

      return next(e)
    }
    if (key.startsWith('row:')) {
      const row = Number(key.slice(4))
      if (now.ides[shownTop + row] === undefined) return {}
      await land(row)

      return next(e)
    }
    if (e.origin.kind === 'person' && now.ring >= 0) {
      const moved = moveBy({ selected: shownTop + now.ring, top: shownTop }, key === 'project' ? 1 : -1, shownRows, now.ides.length)
      await update($, panel, held => ({ ...held, top: moved.top }))
    }

    return {}
  }).catch(($, e, next) => (e.element === undefined || e.element.startsWith('row:') ? next(e) : {}))

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Box, Text, Button } = $.ui.resolve(e)
    const now = await read($, panel)
    const edit = await read($, lastEdit)
    const hasKeys = e.props.isFocused
    const columns = e.props.bodyColumns
    const bodyRows = Math.max(e.props.scroll.bodyRows, 1)
    const project = await $.session.root()
    const projectName = project.split('/').pop() ?? project
    const editName =
      edit === null ? '' : `${edit.path.startsWith(`${project}/`) ? edit.path.slice(project.length + 1) : edit.path}:${edit.line}`
    const wantsFile = now.wantsFile && edit !== null
    const total = now.ides.length
    const status =
      now.error !== '' ? `${GLYPH.dead} ${now.error}`
      : now.isLoading ? `${GLYPH.working} ${total === 0 ? 'looking for IDEs…' : 'opening…'}`
      : total === 0 ? 'No IDE found, in WSL or on Windows. Install VS Code or Cursor, then /open again.'
      : ''
    let free = bodyRows - 1 - (status === '' ? 0 : 1)
    const hasHeader = free >= Math.min(total, 2) + 1
    if (hasHeader) free -= 1
    const rows = total === 0 ? 0 : Math.max(Math.min(total, free, MAX_ROWS), 1)
    const top = clampTop(now.top, Math.min(now.top + Math.max(now.ring, 0), Math.max(total - 1, 0)), rows, total)
    const widest = Math.min(now.ides.reduce((most, ide) => Math.max(most, ide.label.length), 8), Math.max(columns - 14, 8))
    // O alvo ativo vai entre colchetes na própria linha de teclas.
    // A linha de teclas tem de caber inteira: o nome do arquivo cede.
    const lead = total === 0 ? 'Esc close' : columns < 48 ? '' : 'Enter opens '
    const fileLabel = cut(editName.split('/').pop() ?? editName, Math.max(columns - lead.length - 24, 4))
    shownRows = Math.max(rows, 1)
    shownTop = top
    // O anel guardado pode ser de uma lista que já mudou (a detecção terminou
    // com menos IDEs, a janela encolheu): fora das linhas à vista ele não
    // marcava nenhuma e o contador passava do total.
    const ring = Math.min(Math.max(now.ring, 0), Math.max(rows - 1, 0))

    return (
      <Box flexDirection="column">
        {hasHeader && (
          <Box flexDirection="row">
            <Box flexShrink={0}>
              <Text {...ink('flux')} bold>{`${GLYPH.mark} `}</Text>
              <Text {...ink('bright')} bold>OPEN IN</Text>
            </Box>
            <Box flexGrow={1} flexShrink={1}>
              <Text {...ink('muted')} wrap="truncate-end">{` // ${wantsFile ? editName : projectName}`}</Text>
            </Box>
          </Box>
        )}
        {now.ides.slice(top, top + rows).map((ide, row) => {
          const isOwner = hasKeys && row === ring
          const more = row === 0 && top > 0 ? '▴' : row === rows - 1 && top + rows < total ? '▾' : ''

          return (
            <Box flexDirection="row">
              <Box width={2} flexShrink={0}><Text {...ink('flux')}>{isOwner ? `${GLYPH.cursor} ` : '  '}</Text></Box>
              <Box width={widest + 2} flexShrink={0}>
                <Button
                  key={`row:${row}`}
                  plain
                  {...(row === ring ? { autoFocus: true as const } : {})}
                  label={cut(ide.label, widest)}
                  onPress={() => { void open($, ide, wantsFile) }}
                />
              </Box>
              <Text {...ink('muted')}>{ide.location}</Text>
              {total > rows && <Text {...ink('faint')}>{` ${more}`}</Text>}
            </Box>
          )
        })}
        {status !== '' && (
          <Text {...ink(now.error !== '' ? 'dead' : now.isLoading ? 'working' : 'muted')} wrap="truncate-end">{status}</Text>
        )}
        {!hasKeys && (
          <Text {...ink('faint')} wrap="truncate-end">
            {columns < 64 ? 'ctrl+x tab gives this panel the keys.' : 'The prompt has the keyboard. ctrl+x tab hands it to this panel.'}
          </Text>
        )}
        {hasKeys && (
          <Box flexDirection="row">
            <Box flexShrink={0}><Text {...ink('faint')}>{lead}</Text></Box>
            {total > 0 && (
              <Button
                key="project"
                hotkey="p"
                plain
                dimColor={wantsFile}
                label={wantsFile ? 'project' : '[project]'}
                onPress={() => update($, panel, held => ({ ...held, wantsFile: false }))}
              />
            )}
            {total > 0 && edit !== null && <Text>{' '}</Text>}
            {total > 0 && edit !== null && (
              <Button
                key="file"
                hotkey="f"
                plain
                dimColor={!wantsFile}
                label={wantsFile ? `[${fileLabel}]` : fileLabel}
                onPress={() => update($, panel, held => ({ ...held, wantsFile: true }))}
              />
            )}
            {total > 0 && <Box flexGrow={1} />}
            {total > 0 && (
              <Button key="more" plain dimColor label={`${top + ring + 1}/${total}`} onPress={() => undefined} />
            )}
          </Box>
        )}
        {/* Tecla sem ação devolve o teclado ao prompt e é escrita lá: o host
            não deixa um painel consumir teclas, só dar tecla a um botão. Toda
            letra e dígito sem uso ganha um botão invisível que não faz nada.
            Pontuação e espaço não podem ser tecla de botão, e vazam. */}
        {hasKeys && (
          <Box display="none">
            {[...'abcdefghijklmnopqrstuvwxyz0123456789']
              .filter(key => total === 0 || (key !== 'p' && (key !== 'f' || edit === null)))
              .map(key => <Button key={`sink:${key}`} hotkey={key} plain label="" onPress={() => undefined} />)}
          </Box>
        )}
      </Box>
    )
  })
}
