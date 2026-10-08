// tesseract-docker: o Docker Compose do projeto dentro da sessão.
//
// - `/docker` abre o painel: serviços, estado e as três ações que nunca
//   apagam nada (subir, parar, reiniciar), por serviço e pela stack.
// - As ações só rodam pela tecla delas (`u` `s` `r`, `1` `2` `3`): o foco
//   anda só pelos serviços, e Enter num serviço mostra a cauda do log.
// - `p` no log manda o trecho ao prompt, sem enviar.
// - O Claude ganha duas tools: listar serviços e reiniciar um.
// - Toda chamada Bash passa pela guarda (guard.ts), que nega o que apaga
//   volume. Se a guarda falhar, o comando não roda.

import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { DockerPanel, Service } from '../types'
import type { Action } from './compose'
import {
  IGNORED_DIRS,
  actionArgs,
  attention,
  describeAction,
  firstLine,
  markOf,
  parseServices,
  scoreFile,
  summary,
  tail,
} from './compose'
import { denial, findDestructive } from './guard'
import { clampTop, cut, moveBy, wrappedRows } from './list'
import { GLYPH, ink, pad, setMono } from './tokens'

const PANE = 'tesseract-docker'
const READ_TIMEOUT_MS = 20_000
// Subir um serviço baixa imagem: precisa de bem mais tempo que uma leitura.
const ACTION_TIMEOUT_MS = 300_000
const TICK_MS = 3_000
// Com o painel fechado só a faixa de aviso precisa do dado: 1 a cada 5 ticks.
const IDLE_TICKS = 5
// Falha de ação fica na tela por este tanto de ticks (15 s), no máximo.
const FAILED_TICKS = 5
const LOG_LINES = 200
// Linhas de corpo pedidas ao abrir o log; a lista pede o que os serviços
// ocupam. É pedido: a célula dá o que tem, e o desenho se ajusta ao que veio.
const LOG_ROWS = 18
const PROMPT_LINES = 20

const EMPTY: DockerPanel = {
  file: '',
  services: [],
  error: '',
  failed: '',
  isLoading: false,
  working: '',
  selected: '',
  top: 0,
  logOf: '',
  log: [],
}

const panel = atom({ plugin: 'tesseract-docker', key: 'panel' } as const, EMPTY)

type Ran = { out: string; error: string }

const fileName = (path: string): string => path.split('/').pop() ?? path

// Abre o painel, ou o redimensiona se já está aberto, com o teclado nele.
const show = ($: EngineInterface, rows: number) =>
  $.ui.open({ id: PANE, title: 'DOCKER', focus: true, closeOnEscape: true, rows })


// Quantos serviços o último desenho mostrou e a partir de qual: a janela
// que o foco percorre.
let shownRows = 1
let shownTop = 0
// O host guarda o anel de foco pela POSIÇÃO do elemento entre os do painel,
// não pela chave: se a árvore muda, o anel cai no que estiver naquela
// posição. Por isso toda tela do painel começa com SAFE_STOPS paradas
// inofensivas (as linhas da lista e, no que sobra, botões vazios), e as
// ações vêm só depois: nenhuma mudança de tela põe o anel numa ação.
const SAFE_STOPS = 12
// A parada onde o anel está, pelo que o `ui.focus` deixou passar; -1 = fora
// das paradas do painel (a marca de fechar do host).
let ringRow = -1

const wantedRows = (services: readonly Service[]): number => Math.min(services.length, SAFE_STOPS) + 5

// Roda o docker compose do projeto. O erro volta sem o prefixo do comando:
// só a primeira linha do que o Docker reclamou.
async function compose(
  $: EngineInterface,
  file: string,
  args: readonly string[],
  timeoutMs: number,
): Promise<Ran> {
  try {
    const ran = await $.process.run(['docker', 'compose', '--file', file, ...args], {
      cwd: await $.session.root(),
      timeoutMs,
    })
    if (ran.exitCode === 0) return { out: ran.stdout, error: '' }

    return { out: '', error: firstLine(ran.stderr) || `compose exited with ${ran.exitCode}` }
  } catch {
    return { out: '', error: 'Docker did not answer. Start Docker, then repeat the action.' }
  }
}

// Acha o compose do projeto: raiz primeiro, depois as pastas de primeiro
// nível (`docker/`, `infra/`), nunca um arquivo de produção.
async function detect($: EngineInterface): Promise<string> {
  const root = await $.session.root()
  const entries = await $.fs.list(root).catch(() => [])
  let best = ''
  let bestScore = 0
  const consider = (path: string, score: number): void => {
    if (score > bestScore) {
      best = path
      bestScore = score
    }
  }

  const folders: string[] = []
  for (const entry of entries) {
    const isHidden = entry.name.startsWith('.') && entry.name !== '.docker'
    if (entry.kind === 'file') consider(`${root}/${entry.name}`, scoreFile(entry.name, true))
    else if (entry.kind === 'dir' && !IGNORED_DIRS.has(entry.name) && !isHidden) {
      folders.push(entry.name)
    }
  }
  const listings = await Promise.all(
    folders.map(folder => $.fs.list(`${root}/${folder}`).catch(() => [])),
  )
  listings.forEach((inner, index) => {
    for (const entry of inner) {
      if (entry.kind === 'file') {
        consider(`${root}/${folders[index]}/${entry.name}`, scoreFile(entry.name, false))
      }
    }
  })

  return best
}

// Grava só quando algo mudou: cada escrita redesenha o painel.
async function patch($: EngineInterface, change: Partial<DockerPanel>): Promise<void> {
  const now = await read($, panel)
  const next = { ...now, ...change }
  if (JSON.stringify(next) !== JSON.stringify(now)) await update($, panel, held => ({ ...held, ...change }))
}

// Relê os serviços. Sem leitura válida não há lista: serviço velho ao lado
// de um erro diria que algo está de pé sem ninguém ter visto.
async function refresh($: EngineInterface): Promise<void> {
  const now = await read($, panel)
  const file = now.file === '' ? await detect($) : now.file
  if (file === '') {
    await patch($, { isLoading: false })

    return
  }

  const ran = await compose($, file, ['ps', '--all', '--format', 'json'], READ_TIMEOUT_MS)
  let services: Service[] = []
  let error = ran.error
  if (error === '') {
    try {
      services = parseServices(ran.out)
    } catch {
      error = 'Could not read the answer of docker compose ps.'
    }
  }
  // O foco pode ter andado enquanto o Docker respondia.
  const held = await read($, panel)
  const index = Math.max(services.findIndex(service => service.name === held.selected), 0)
  await patch($, {
    file,
    services,
    error,
    isLoading: false,
    selected: services[index]?.name ?? '',
    // O erro de agora vence o de uma ação passada.
    ...(error === '' ? {} : { failed: '' }),
  })
}

let failedAge = 0

// Roda uma ação num serviço, ou na stack inteira quando o serviço vem vazio.
async function act($: EngineInterface, action: Action, service: string): Promise<string> {
  const now = await read($, panel)
  if (now.file === '') return 'No compose file in this project.'
  if (now.working !== '') return `Busy ${now.working}. Wait for it to finish.`

  await patch($, { working: describeAction(action, service), failed: '' })
  const ran = await compose($, now.file, actionArgs(action, service), ACTION_TIMEOUT_MS)
  failedAge = 0
  await patch($, { working: '', failed: ran.error })
  await refresh($)

  return ran.error
}

// Tira sequências ANSI e caracteres de controle: o log vira texto de painel.
const clean = (line: string): string =>
  line.replace(/\u001b\[[0-9;?]*[A-Za-z]/g, '').replace(/[\u0000-\u0008\u000b-\u001f\u007f]/g, '')

async function loadLog($: EngineInterface, service: string): Promise<void> {
  const { file } = await read($, panel)
  const ran = await compose(
    $,
    file,
    ['logs', '--no-color', '--no-log-prefix', '--tail', String(LOG_LINES), service],
    READ_TIMEOUT_MS,
  )
  // O usuário pode ter voltado à lista enquanto o log vinha.
  if ((await read($, panel)).logOf !== service) return

  failedAge = 0
  await patch($, {
    log: ran.error === '' ? tail(ran.out, LOG_LINES).map(clean) : [],
    failed: ran.error,
  })
}

async function openLog($: EngineInterface, service: string): Promise<void> {
  if (service === '') return

  await patch($, { logOf: service, log: [], failed: '', selected: service })
  await show($, LOG_ROWS)
  await loadLog($, service)
}

async function closeLog($: EngineInterface): Promise<void> {
  await patch($, { logOf: '', log: [], failed: '' })
  await show($, wantedRows((await read($, panel)).services))
}

// Põe a cauda do log no prompt, sem enviar, e devolve o teclado ao prompt.
async function sendToPrompt($: EngineInterface): Promise<void> {
  const now = await read($, panel)
  const lines = now.log.slice(-PROMPT_LINES)
  if (lines.length === 0) {
    $.ui.toast(`No log lines of ${now.logOf} yet. Nothing sent.`)

    return
  }

  const text = `Log of the \`${now.logOf}\` service, last ${lines.length} lines:\n\`\`\`\n${lines.join('\n')}\n\`\`\`\n`
  const filled = await $.prompt.fill({ text, mode: 'append' })
  if (!filled.isFilled) {
    $.ui.toast('Prompt is busy. Close its dialog, then p again.')

    return
  }
  await $.ui.close({ id: PANE })
  // O que importa vem primeiro: o aviso é cortado na largura da célula.
  $.ui.toast(`Not sent: ${lines.length} log lines of ${cut(now.logOf, 16)} are in the prompt.`)
}

let isTicking = false
let ticks = 0

// Um relógio só: com o painel aberto relê a cada tick; fechado, só de vez
// em quando, para a faixa de aviso. Falha aqui nunca derruba o mod.
async function tick($: EngineInterface): Promise<void> {
  if (isTicking) return

  isTicking = true
  try {
    ticks += 1
    const now = await read($, panel)
    if (now.file === '' || now.working !== '') return

    // Falha de ação não fica para sempre: o que vale é a leitura de agora.
    if (now.failed !== '' && ++failedAge >= FAILED_TICKS) await patch($, { failed: '' })

    const isOpen = (await $.ui.panes()).some(pane => pane.id === PANE)
    if (!isOpen && ticks % IDLE_TICKS !== 0) return

    await refresh($)
    if (isOpen && now.logOf !== '') await loadLog($, now.logOf)
  } catch {
    // ponytail: tick perdido é só dado velho por 3 s; o próximo relê.
  } finally {
    isTicking = false
  }
}

// O que fazer diante de uma leitura que falhou, em uma frase.
const nextStep = (error: string, file: string): string =>
  /daemon|did not answer|docker desktop/i.test(error)
    ? 'Start Docker. This panel rereads every 3s.'
    : `Check ${fileName(file)}. This panel rereads every 3s.`

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    try {
      setMono(await $.env.get('NO_COLOR'))
      const file = await detect($)
      if (file !== '') {
        await patch($, { file })
        void refresh($).catch(() => undefined)
        // As tools só existem onde há compose: sem ele seriam ruído no prompt.
        await $.tool.register({
          name: 'services',
          description:
            'Lists the Docker Compose services of this project with state, published port, health and uptime. Read-only.',
          inputSchema: { type: 'object', properties: {}, additionalProperties: false },
        })
        await $.tool.register({
          name: 'restart',
          description:
            'Restarts one Docker Compose service of this project by name (docker compose restart <service>). Non-destructive: no container, image or volume is removed. Call the services tool first for the names.',
          inputSchema: {
            type: 'object',
            properties: {
              service: { type: 'string', description: 'The service name, as the services tool lists it.' },
            },
            required: ['service'],
            additionalProperties: false,
          },
        })
      }
      $.clock.every(TICK_MS, () => {
        void tick($)
      })
    } catch (error) {
      $.ui.log(`docker setup failed: ${String(error).slice(0, 120)}`, { to: 'debug' })
    }
    await $.command.register({
      name: 'docker',
      description: 'Docker Compose panel: services, start, stop, restart, log tail',
    })

    return started
  })

  on('command.run', { command: 'docker' }, async $ => {
    const held = await read($, panel)
    const file = held.file === '' ? await detect($) : held.file
    if (file === '') {
      return { text: 'No compose file in this project. Add compose.yml at the root, then run /docker again.' }
    }

    await patch($, { file, logOf: '', log: [], failed: '', isLoading: held.services.length === 0 })
    const opened = await show($, wantedRows(held.services))
    void refresh($).catch(() => undefined)

    return opened.isPlaced ? {} : { text: `Docker panel is waiting for room: ${opened.reason}` }
  })

  // A guarda: julga antes de deixar passar. Se ela mesma falhar, nega.
  on('tool.call', { tool: 'Bash' }, ($, e, next) => {
    const verdict = findDestructive(typeof e.command === 'string' ? e.command : '')

    return verdict === undefined ? next(e) : { deny: denial(verdict) }
  }).catch(($, e, next) =>
    next.called
      ? next(e)
      : { deny: 'The Docker guard failed, so the command did not run. Retry it; if it fails again, tell the user.' },
  )

  on('tool.call', { tool: 'mcp__tesseract-docker__services' }, async $ => {
    await refresh($)
    const now = await read($, panel)
    if (now.file === '') return { result: 'No compose file in this project.' }
    if (now.error !== '') return { result: `docker compose ps failed: ${now.error}` }
    if (now.services.length === 0) {
      return { result: `compose file: ${now.file}\nThe stack has never come up in this project.` }
    }

    const rows = now.services.map(service =>
      [service.name, service.state, service.port || '-', service.health || '-', service.uptime || '-'].join('\t'),
    )

    return { result: [`compose file: ${now.file}`, 'service\tstate\tport\thealth\tuptime', ...rows].join('\n') }
  })

  on('tool.call', { tool: 'mcp__tesseract-docker__restart' }, async ($, e) => {
    await refresh($)
    const now = await read($, panel)
    const service = typeof e.service === 'string' ? e.service : ''
    if (now.error !== '') return { result: `docker compose ps failed: ${now.error}` }
    // Só nome que o compose listou: nada de flag disfarçada de serviço.
    if (!now.services.some(one => one.name === service)) {
      const names = now.services.map(one => one.name).join(', ')

      return { result: `No service named "${service}". Services here: ${names || 'none'}.` }
    }
    const failure = await act($, 'restart', service)

    return { result: failure === '' ? `Restarted ${service}.` : `Restart of ${service} failed: ${failure}` }
  })

  // O foco anda só pelos serviços. Uma ação nunca segura o anel: é assim que
  // Enter nunca cai num botão que muda estado. Tentar pousar depois da
  // última linha visível é "desci além da janela"; numa ação que não a
  // primeira (a volta do anel pelo fim), "subi além dela".
  on('ui.focus', { requestId: PANE }, async ($, e, next) => {
    const key = e.element
    if (key === undefined) {
      ringRow = -1

      return next(e)
    }

    const now = await read($, panel)
    const isPerson = e.origin.kind === 'person'
    const row = key.startsWith('row:') ? Number(key.slice(4)) : -1
    if (now.logOf !== '' || now.error !== '' || now.services.length === 0) {
      // Log, erro e stack vazia: as paradas são todas inofensivas, e quem já
      // está numa não precisa andar.
      if (row < 0 || (isPerson && ringRow >= 0)) return {}
      ringRow = row

      return next(e)
    }

    const service = row >= 0 && row < shownRows ? now.services[shownTop + row] : undefined
    if (service !== undefined) {
      ringRow = row
      await patch($, { selected: service.name, top: shownTop, ...(isPerson ? { failed: '' } : {}) })

      return next(e)
    }
    if (!isPerson) return {}
    // De uma parada vazia (a lista encolheu sob o anel) só se anda de volta
    // em direção às linhas, nunca mais fundo nas vazias.
    if (ringRow >= shownRows) {
      if (row < 0 || row > ringRow) return {}
      ringRow = row

      return next(e)
    }
    if (ringRow >= 0 && ringRow < shownRows) {
      const by = row >= 0 || key === 'up' ? 1 : -1
      const moved = moveBy({ selected: shownTop + ringRow, top: shownTop }, by, shownRows, now.services.length)
      await patch($, { selected: now.services[moved.selected]?.name ?? '', top: moved.top, failed: '' })
    }

    return {}
  }).catch(($, e, next) => (e.element === undefined ? next(e) : {}))

  // PgUp e PgDn andam uma janela; a linha sob o anel continua a escolhida.
  on('ui.scroll', { requestId: PANE }, async ($, e) => {
    const now = await read($, panel)
    const total = now.services.length
    if (now.logOf === '' && now.error === '' && total > 0 && e.origin.kind === 'person') {
      const top = Math.max(Math.min(shownTop + Math.sign(e.by) * shownRows, total - shownRows), 0)
      const selected = Math.min(top + Math.max(Math.min(ringRow, shownRows - 1), 0), total - 1)
      await patch($, { top, selected: now.services[selected]?.name ?? now.selected })
    }

    return {}
  }).catch(() => ({}))

  // A faixa só existe quando um serviço quebrou ou adoeceu, e some com o
  // painel aberto. Resumo neutro ("2/3") fica no cabeçalho do painel: a linha
  // de status do host desenha tudo como aviso.
  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    const below = await next(e)
    if (e.props.hasSurvey) return below

    const now = await read($, panel)
    const { crashed, unhealthy } = attention(now.services)
    if (crashed.length + unhealthy.length === 0) return below
    // Com o painel aberto a lista já mostra: a faixa só gastaria uma linha.
    if ((await $.ui.panes()).some(pane => pane.id === PANE && pane.isShown)) return below

    const { Box, Text } = $.ui.resolve(e)
    const isNarrow = e.props.bodyColumns < 48
    const names = [...crashed, ...unhealthy].map(service => service.name).join(' ')

    return (
      <Box flexDirection="column">
        {below}
        <Text wrap="truncate-end">
          <Text>{'  '}</Text>
          {crashed.length > 0 && (
            <Text {...ink('dead')} bold>{`${GLYPH.dead} ${crashed.length}${isNarrow ? '' : ' CRASHED'}`}</Text>
          )}
          {crashed.length > 0 && unhealthy.length > 0 && <Text {...ink('faint')}> · </Text>}
          {unhealthy.length > 0 && (
            <Text {...ink('orphan')} bold>{`${GLYPH.orphan} ${unhealthy.length}${isNarrow ? '' : ' UNHEALTHY'}`}</Text>
          )}
          <Text {...ink('faint')}> · </Text>
          <Text {...ink('muted')}>{names}</Text>
          <Text {...ink('faint')}> · /docker</Text>
        </Text>
      </Box>
    )
  })

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Box, Text, Button } = $.ui.resolve(e)
    const now = await read($, panel)
    const columns = e.props.bodyColumns
    const bodyRows = Math.max(e.props.scroll.bodyRows, 1)
    const hasKeys = e.props.isFocused
    const project = fileName(await $.session.root())
    const total = now.services.length
    const held = Math.max(now.services.findIndex(service => service.name === now.selected), 0)

    // Paradas vazias que completam as SAFE_STOPS: invisíveis e inofensivas.
    const pads = (from: number, to: number, onPress: () => void) =>
      Array.from({ length: Math.max(to - from, 0) }, (_, offset) => (
        <Button key={`row:${from + offset}`} plain label="" onPress={onPress} />
      ))
    const nothing = (): void => undefined
    // Tecla sem ação devolve o teclado ao prompt e é escrita lá: o host não
    // deixa um painel consumir teclas, só dar tecla a um botão. Então toda
    // letra e dígito que a tela não usa ganha um botão invisível que não
    // faz nada. Pontuação e espaço não podem ser tecla de botão, e vazam.
    const sink = (used: string) => (
      <Box display="none">
        {[...'abcdefghijklmnopqrstuvwxyz0123456789'].filter(key => !used.includes(key)).map(key => (
          <Button key={`sink:${key}`} hotkey={key} plain label="" onPress={nothing} />
        ))}
      </Box>
    )

    // Marca e título nunca encolhem; o miolo corta com reticências.
    const header = (trail: string, right: string) => (
      <Box flexDirection="row">
        <Box flexShrink={0}>
          <Text {...ink('flux')} bold>{`${GLYPH.mark} `}</Text>
          <Text {...ink('bright')} bold>DOCKER</Text>
        </Box>
        <Box flexGrow={1} flexShrink={1}>
          <Text {...ink('muted')} wrap="truncate-end">{` // ${trail}`}</Text>
        </Box>
        <Box flexShrink={0}>
          <Text {...ink('muted')}>{right === '' ? '' : ` ${right}`}</Text>
        </Box>
      </Box>
    )
    const noKeys = (
      <Text {...ink('faint')} wrap="truncate-end">
        {columns < 64 ? 'ctrl+x tab gives this panel the keys.' : 'The prompt has the keyboard. ctrl+x tab hands it to this panel.'}
      </Text>
    )
    const gap = ' '.repeat(columns >= 40 ? 2 : 1)
    // Largura de um botão com tecla, como o host o desenha: `u: up`.
    const keyed = (label: string): number => label.length + 3

    // Sem leitura válida: só o erro e o que fazer. Nada de serviço velho.
    if (now.error !== '') {
      const hasHeader = bodyRows >= 4
      const room = Math.max(bodyRows - (hasHeader ? 2 : 1), 1)
      const said = `${GLYPH.dead} ${now.error}`

      return (
        <Box flexDirection="column">
          {hasHeader && header(`${project} // ${fileName(now.file)}`, '')}
          <Box height={Math.min(wrappedRows(said, columns), room)} overflow="hidden">
            <Text {...ink('dead')}>{said}</Text>
          </Box>
          <Box flexDirection="row">
            {pads(0, SAFE_STOPS, nothing)}
            <Text {...ink('faint')} wrap="truncate-end">{`${nextStep(now.error, now.file)} Esc closes.`}</Text>
          </Box>
          {hasKeys && sink('')}
        </Box>
      )
    }

    if (now.logOf !== '') {
      // O "back" ocupa a parada onde o anel já estava (a linha do serviço):
      // abrir o log nunca deixa o anel em outro botão.
      const home = ringRow >= 0 ? Math.min(ringRow, SAFE_STOPS - 1) : Math.min(Math.max(held - now.top, 0), SAFE_STOPS - 1)
      const hasTrouble = now.failed !== ''
      const hasHeader = bodyRows - 1 - (hasTrouble ? 1 : 0) >= 3
      const room = Math.max(bodyRows - 1 - (hasTrouble ? 1 : 0) - (hasHeader ? 1 : 0), 1)
      const shown = now.log.slice(-room)

      return (
        <Box flexDirection="column">
          {hasHeader && header(`${now.logOf} // log`, shown.length === 0 ? '' : `last ${shown.length}`)}
          {shown.length === 0 && !hasTrouble && (
            <Text {...ink('faint')} wrap="truncate-end">{`No log lines from ${cut(now.logOf, 16)} yet. Rereads every 3s.`}</Text>
          )}
          {shown.map(line => (
            <Text {...ink('text')} wrap="truncate-end">{line === '' ? ' ' : line}</Text>
          ))}
          {hasTrouble && <Text {...ink('dead')} wrap="truncate-end">{`${GLYPH.dead} ${now.failed}`}</Text>}
          <Box flexDirection="row">
            {pads(0, home, () => { void closeLog($) })}
            {hasKeys
              ? <Button key={`row:${home}`} hotkey="b" plain autoFocus label="back" onPress={() => closeLog($)} />
              : <Button key={`row:${home}`} plain label="" onPress={() => closeLog($)} />}
            {pads(home + 1, SAFE_STOPS, () => { void closeLog($) })}
            {hasKeys && <Text>{gap}</Text>}
            {hasKeys && <Button key="prompt" hotkey="p" plain label={columns < 40 ? 'prompt' : 'to prompt'} onPress={() => sendToPrompt($)} />}
            {hasKeys && <Text>{gap}</Text>}
            {hasKeys && <Button key="restart" hotkey="r" plain label="restart" onPress={() => { void act($, 'restart', now.logOf) }} />}
            {!hasKeys && noKeys}
          </Box>
          {hasKeys && sink('bpr')}
        </Box>
      )
    }

    // A linha de teclas: as ações do serviço e as da stack. Cabendo, uma
    // linha só; sem caber, duas; em célula baixa e estreita ficam as do
    // serviço (as da stack voltam quando houver linha para elas).
    const isEmpty = total === 0
    const fits = (labels: readonly string[]): boolean =>
      labels.reduce((width, label) => width + keyed(label), 0) + (labels.length - 1) * gap.length <= columns
    const serviceLabels = isEmpty ? [] : fits(['up', 'stop', 'restart', 'log']) ? ['up', 'stop', 'restart', 'log'] : ['up', 'stop', 'restart']
    const stackWord = columns < 40 ? 'all' : 'stack'
    const serviceWidth = serviceLabels.reduce((width, label) => width + keyed(label) + gap.length, 0)
    const stackWidth = stackWord.length + keyed('up') + keyed('stop') + keyed('restart') + 3 * gap.length
    const isOneRow = isEmpty || serviceWidth + stackWidth <= columns
    const hasStack = isOneRow || bodyRows >= 5
    const keyRows = !hasKeys || isOneRow || !hasStack ? 1 : 2

    const isIdle = now.working === ''
    const status = !isIdle ? `${GLYPH.working} ${now.working}…` : now.failed !== '' ? `${GLYPH.dead} ${now.failed}` : ''
    let free = bodyRows - keyRows - (status === '' ? 0 : 1)
    const hasHeader = free >= (isEmpty ? 2 : 3)
    if (hasHeader) free -= 1
    // O cabeçalho de colunas só entra quando a lista inteira cabe com ele.
    const hasLabels = !isEmpty && free >= total + 1
    if (hasLabels) free -= 1
    const rows = isEmpty ? 0 : Math.max(Math.min(total, free, SAFE_STOPS), 1)
    // O escolhido é quem está sob o anel. Quando a lista muda por baixo, a
    // janela anda para manter o mesmo serviço ali; sem como, o anel vence.
    const isRingOnRow = ringRow >= 0 && ringRow < rows
    const wanted = held - ringRow
    const canKeep = isRingOnRow && wanted >= 0 && wanted <= Math.max(total - rows, 0)
    const top = canKeep ? wanted : clampTop(now.top, isRingOnRow ? Math.min(now.top + ringRow, total - 1) : held, rows, total)
    const index = isRingOnRow && !canKeep ? Math.min(top + ringRow, total - 1) : held
    const target = now.services[index]?.name ?? ''
    const hasHint = hasKeys && free - Math.max(rows, 1) >= 1
    shownRows = Math.max(rows, 1)
    shownTop = top

    // Colunas que cabem: o nome e o estado sempre; o resto some ao estreitar.
    const longest = now.services.reduce((most, service) => Math.max(most, service.name.length), 7)
    const nameWidth = Math.min(longest + 1, 24, Math.max(columns - 16, 8))
    const isScrolled = total > rows
    const stateWidth = Math.max(Math.min(15, columns - 2 - nameWidth - (isScrolled ? 2 : 0)), 4)
    const room = columns - 2 - nameWidth - stateWidth - (isScrolled ? 2 : 0)
    const hasPort = room >= 8
    const hasHealth = room >= 8 + 11
    const hasUptime = room >= 8 + 11 + 6
    const labels =
      pad('SERVICE', nameWidth) +
      pad('STATE', stateWidth) +
      (hasPort ? pad('PORT', 8) : '') +
      (hasHealth ? pad('HEALTH', 11) : '') +
      (hasUptime ? 'UPTIME' : '')

    const serviceKeys = serviceLabels.flatMap(label => [
      <Button
        key={label}
        hotkey={label[0] as string}
        plain
        label={label}
        onPress={() => { void (label === 'log' ? openLog($, target) : act($, label as Action, target)) }}
      />,
      <Text>{gap}</Text>,
    ])
    const stackKeys = [
      <Text {...ink('fluxCore')}>{`${stackWord}${gap}`}</Text>,
      <Button key="up-all" hotkey="1" plain label="up" onPress={() => { void act($, 'up', '') }} />,
      <Text>{gap}</Text>,
      <Button key="stop-all" hotkey="2" plain label="stop" onPress={() => { void act($, 'stop', '') }} />,
      <Text>{gap}</Text>,
      <Button key="restart-all" hotkey="3" plain label="restart" onPress={() => { void act($, 'restart', '') }} />,
    ]

    return (
      <Box flexDirection="column">
        {hasHeader && header(`${project} // ${fileName(now.file)}`, isEmpty ? '' : summary(now.services))}
        {hasLabels && <Text {...ink('fluxCore')} wrap="truncate-end">{`  ${labels}`}</Text>}
        {isEmpty && (
          <Box flexDirection="row">
            {pads(0, SAFE_STOPS, nothing)}
            <Text {...ink(now.isLoading ? 'faint' : 'muted')} wrap="truncate-end">
              {now.isLoading ? '  reading the stack…' : '  Stack is down. 1 brings it up.'}
            </Text>
          </Box>
        )}
        {now.services.slice(top, top + rows).map((service, row) => {
          const mark = markOf(service)
          const isOwner = hasKeys && top + row === index
          const more = row === 0 && top > 0 ? '▴' : row === rows - 1 && top + rows < total ? '▾' : ''

          return (
            <Box flexDirection="row">
              <Box width={2} flexShrink={0}><Text {...ink('flux')}>{isOwner ? `${GLYPH.cursor} ` : '  '}</Text></Box>
              <Box width={nameWidth} flexShrink={0}>
                <Button
                  key={`row:${row}`}
                  plain
                  {...(top + row === index ? { autoFocus: true as const } : {})}
                  label={cut(service.name, nameWidth - 1)}
                  onPress={() => openLog($, service.name)}
                />
              </Box>
              <Box width={stateWidth} flexShrink={0}>
                <Text {...ink(mark.token)} wrap="truncate-end">{`${mark.glyph} ${service.state}`}</Text>
              </Box>
              {hasPort && <Box width={8} flexShrink={0}><Text {...ink('muted')}>{service.port || '—'}</Text></Box>}
              {hasHealth && <Box width={11} flexShrink={0}><Text {...ink('muted')}>{service.health || '—'}</Text></Box>}
              {hasUptime && <Text {...ink('muted')} wrap="truncate-end">{service.uptime || '—'}</Text>}
              {isScrolled && <Box flexGrow={1} />}
              {isScrolled && <Text {...ink('faint')}>{` ${more}`}</Text>}
              {row === rows - 1 && pads(rows, SAFE_STOPS, nothing)}
            </Box>
          )
        })}
        {status !== '' && (
          <Text {...ink(isIdle ? 'dead' : 'working')} wrap="truncate-end">{status}</Text>
        )}
        {hasHint && (
          <Text {...ink('faint')} wrap="truncate-end">
            {now.working.startsWith('bringing up')
              ? 'Pulling images and waiting for health can take minutes.'
              : isEmpty ? 'Esc close' : '↑↓ service  Enter log  Esc close'}
          </Text>
        )}
        {!hasKeys && noKeys}
        {hasKeys && isOneRow && <Box flexDirection="row">{serviceKeys}{stackKeys}</Box>}
        {hasKeys && !isOneRow && <Box flexDirection="row">{serviceKeys}</Box>}
        {hasKeys && !isOneRow && hasStack && <Box flexDirection="row">{stackKeys}</Box>}
        {hasKeys && sink(serviceLabels.map(label => label[0]).join('') + (isOneRow || hasStack ? '123' : ''))}
      </Box>
    )
  })
}
