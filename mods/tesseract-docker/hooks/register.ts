// tesseract-docker: o que só um mod consegue fazer pelo Docker do projeto.
// O painel é o do mosaico do Tesseract (tecla `d`); aqui não há painel.
//
// - Toda chamada Bash passa pela guarda (guard.ts), que nega o que apaga
//   volume. Se a guarda falhar, o comando não roda.
// - O Claude ganha duas tools: listar serviços e reiniciar um.
// - `/dlog <serviço>` põe as últimas 20 linhas do log no prompt, sem enviar;
//   sem argumento, lista os serviços.

import type { EngineInterface, Register } from 'claude-code'

import type { Service } from './compose'
import { IGNORED_DIRS, firstLine, parseServices, scoreFile, tail } from './compose'
import { denial, findDestructive } from './guard'

const READ_TIMEOUT_MS = 20_000
// Reiniciar espera o serviço parar: precisa de bem mais tempo que uma leitura.
const RESTART_TIMEOUT_MS = 300_000
const PROMPT_LINES = 20

type Ran = { out: string; error: string }

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
    return { out: '', error: 'Docker did not answer. Start Docker, then try again.' }
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

// Toda resposta sem lista termina no que fazer: quem leu "não tem" precisa
// saber o passo seguinte sem sair para procurar. As teclas são as do mosaico.
const NO_COMPOSE = 'No compose file in this project. Add compose.yml at its root, then /dlog lists the services.'
const NEVER_UP =
  'The stack has never come up in this project. ctrl+l leaves the cell, d opens the Docker panel and U brings it up.'

// A stack como o Docker a mostra agora, ou por que não deu para ler. Sem
// leitura válida não há lista: serviço velho ao lado de um erro diria que
// algo está de pé sem ninguém ter visto.
type Stack = { file: string; services: Service[]; trouble: string }

async function readStack($: EngineInterface): Promise<Stack> {
  const file = await detect($)
  if (file === '') return { file, services: [], trouble: NO_COMPOSE }

  const ran = await compose($, file, ['ps', '--all', '--format', 'json'], READ_TIMEOUT_MS)
  if (ran.error !== '') return { file, services: [], trouble: `docker compose ps failed: ${ran.error}` }
  try {
    return { file, services: parseServices(ran.out), trouble: '' }
  } catch {
    return { file, services: [], trouble: 'Could not read the answer of docker compose ps.' }
  }
}

// A lista em texto: serve à tool e ao `/dlog` sem argumento.
function listing(stack: Stack): string {
  if (stack.trouble !== '') return stack.trouble
  if (stack.services.length === 0) {
    return `compose file: ${stack.file}\n${NEVER_UP}`
  }

  const rows = stack.services.map(service =>
    [service.name, service.state, service.port || '-', service.health || '-', service.uptime || '-'].join('\t'),
  )

  return [`compose file: ${stack.file}`, 'service\tstate\tport\thealth\tuptime', ...rows].join('\n')
}

// Só nome que o compose listou passa: nada de flag disfarçada de serviço.
const unknown = (stack: Stack, service: string): string | undefined =>
  stack.services.some(one => one.name === service)
    ? undefined
    : `No service named "${service}". Services here: ${stack.services.map(one => one.name).join(', ') || 'none'}.`

// Tira sequências ANSI e caracteres de controle: o log vira texto de prompt.
const clean = (line: string): string =>
  line.replace(/\u001b\[[0-9;?]*[A-Za-z]/g, '').replace(/[\u0000-\u0008\u000b-\u001f\u007f]/g, '')

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    try {
      // As tools só existem onde há compose: sem ele seriam ruído no prompt.
      if ((await detect($)) !== '') {
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
    } catch (error) {
      $.ui.log(`docker setup failed: ${String(error).slice(0, 120)}`, { to: 'debug' })
    }
    await $.command.register({
      name: 'dlog',
      description: `Puts the last ${PROMPT_LINES} log lines of a Compose service in the prompt, unsent; with no service, lists them`,
      argumentHint: '[service]',
    })

    return started
  })

  on('command.run', { command: 'dlog' }, async ($, e) => {
    const stack = await readStack($)
    const service = e.args.trim()
    if (stack.trouble !== '' || stack.services.length === 0) return { text: listing(stack) }
    if (service === '') return { text: `${listing(stack)}\n/dlog <service> puts its last ${PROMPT_LINES} log lines in the prompt.` }

    const refusal = unknown(stack, service)
    if (refusal !== undefined) return { text: refusal }

    const ran = await compose(
      $,
      stack.file,
      ['logs', '--no-color', '--no-log-prefix', '--tail', String(PROMPT_LINES), service],
      READ_TIMEOUT_MS,
    )
    if (ran.error !== '') return { text: `docker compose logs failed: ${ran.error}` }

    const lines = tail(ran.out, PROMPT_LINES).map(clean)
    if (lines.length === 0) return { text: `No log lines of ${service} yet. Nothing put in the prompt.` }

    const text = `Log of the \`${service}\` service, last ${lines.length} lines:\n\`\`\`\n${lines.join('\n')}\n\`\`\`\n`
    const filled = await $.prompt.fill({ text, mode: 'append' })
    if (!filled.isFilled) return { text: 'Prompt is busy. Close its dialog, then /dlog again.' }

    // O que importa vem primeiro: o aviso é cortado na largura da célula.
    $.ui.toast(`Not sent: ${lines.length} log lines of ${service} are in the prompt.`)

    return {}
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

  on('tool.call', { tool: 'mcp__tesseract-docker__services' }, async $ => ({ result: listing(await readStack($)) }))

  on('tool.call', { tool: 'mcp__tesseract-docker__restart' }, async ($, e) => {
    const stack = await readStack($)
    const service = typeof e.service === 'string' ? e.service : ''
    if (stack.trouble !== '') return { result: stack.trouble }

    const refusal = unknown(stack, service)
    if (refusal !== undefined) return { result: refusal }

    const ran = await compose($, stack.file, ['restart', service], RESTART_TIMEOUT_MS)

    return { result: ran.error === '' ? `Restarted ${service}.` : `Restart of ${service} failed: ${ran.error}` }
  })
}
