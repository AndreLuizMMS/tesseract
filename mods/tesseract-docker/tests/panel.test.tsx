import type { On } from 'claude-code'
import { expect, test } from 'claude-code/testing'

const PANE = 'tesseract-docker'
const ACTIONS = ['up', 'stop', 'restart', 'log', 'up-all', 'stop-all', 'restart-all', 'prompt']

const DOCKER = {
  command: 'docker',
  args: '',
  origin: { kind: 'composer' as const },
  presentation: { isFullscreen: false, columns: 80 },
}

const row = (name: string, state = 'running', status = 'Up 46 seconds') =>
  JSON.stringify({ Service: name, State: state, Status: status, ExitCode: state === 'exited' ? 1 : 0 })
const FIVE = ['api', 'cache', 'db', 'web', 'worker'].map(name => row(name)).join('\n')

const props = (bodyRows: number, bodyColumns = 76) => ({
  title: 'DOCKER',
  isFocused: true,
  bodyColumns,
  placement: 'inline' as const,
  scroll: { offset: 0, bodyRows },
  view: {},
})

// Um projeto com compose e um Docker de mentira: `answer` decide o que o
// `docker compose` devolve, e `ran` guarda cada chamada.
function fakeDocker(on: On, answer: { out: string; error: string }) {
  const ran: string[][] = []
  on('session.root', () => ({ value: '/proj' }))
  on('command.run', () => ({}))
  on('clock.after', () => ({ value: undefined }))
  on('fs.list', (_, e) => ({ value: e.path === '/proj' ? [{ name: 'compose.yml', kind: 'file' as const, size: 1, mtimeMs: 0, isLink: false }] : [] }))
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('ui.panes', () => ({ value: [] }))
  on('process.run', (_, e) => {
    ran.push([...e.argv])
    const isRead = e.argv.includes('ps') || e.argv.includes('logs')

    return {
      value: {
        exitCode: isRead && answer.error !== '' ? 1 : 0,
        stdout: e.argv.includes('ps') ? answer.out : 'line one\nline two\n',
        stderr: isRead ? answer.error : '',
        isStdoutTruncated: false,
        isStderrTruncated: false,
      },
    }
  })

  return ran
}

// As paradas e ações que a pessoa vê. Os botões `sink:` são invisíveis: só
// seguram as teclas sem ação, e têm teste próprio.
const keysOf = (buttons: readonly { key?: string }[]): string[] =>
  buttons.map(button => button.key ?? '').filter(key => !key.startsWith('sink:'))
const sunk = (buttons: readonly { key?: string }[]): string =>
  buttons.map(button => button.key ?? '').filter(key => key.startsWith('sink:')).map(key => key.slice(5)).join('')

test('lista: as 12 primeiras paradas são linhas, as ações vêm depois e cabem no corpo', async ($, on) => {
  fakeDocker(on, { out: FIVE, error: '' })
  await $.command.run(DOCKER)
  const ui = await $.ui.mount({ plugin: PANE, surface: 'terminal', component: 'Pane', requestId: PANE, props: props(3) })

  const keys = keysOf(await ui.findAll({ type: 'Button' }))
  expect(keys.slice(0, 12)).toEqual(Array.from({ length: 12 }, (_, index) => `row:${index}`))
  expect(keys.slice(12)).toEqual(['up', 'stop', 'restart', 'log', 'up-all', 'stop-all', 'restart-all'])
  // As teclas com ação continuam delas; o resto do alfabeto não faz nada.
  expect(sunk(await ui.findAll({ type: 'Button' }))).toBe('abcdefghijkmnopqtvwxyz0456789')
  // Corpo de 3 linhas: duas de lista, uma de teclas. As outras paradas são vazias.
  const labels = (await ui.findAll({ type: 'Button' })).map(button => button.text ?? '')
  expect(labels.slice(0, 3)).toEqual(['api', 'cache', ''])
  await ui.unmount()
})

test('log: Enter em qualquer parada das 12 volta à lista, nunca reinicia', async ($, on) => {
  const ran = fakeDocker(on, { out: FIVE, error: '' })
  await $.command.run(DOCKER)
  const ui = await $.ui.mount({ plugin: PANE, surface: 'terminal', component: 'Pane', requestId: PANE, props: props(3) })

  for (let stop = 0; stop < 12; stop++) {
    await ui.press({ key: 'row:1' })
    await ui.redraw()
    const keys = keysOf(await ui.findAll({ type: 'Button' }))
    expect(keys.slice(0, 12)).toEqual(Array.from({ length: 12 }, (_, index) => `row:${index}`))
    expect(keys.slice(12)).toEqual(['prompt', 'restart'])
    expect((await ui.find({ key: 'row:1' }))?.text).toMatch(/back/)

    await ui.press({ key: `row:${stop}` })
    await ui.redraw()
    expect(keysOf(await ui.findAll({ type: 'Button' })).slice(12)).toContain('up')
  }
  expect(ran.some(argv => ACTIONS.some(action => argv.includes(action) && action !== 'log'))).toBe(false)
  await ui.unmount()
})

test('ação: só o botão da ação roda, no serviço escolhido', async ($, on) => {
  const ran = fakeDocker(on, { out: FIVE, error: '' })
  await $.command.run(DOCKER)
  const ui = await $.ui.mount({ plugin: PANE, surface: 'terminal', component: 'Pane', requestId: PANE, props: props(3) })

  await ui.press({ key: 'restart' })
  expect(ran.filter(argv => argv.includes('restart'))).toEqual([['docker', 'compose', '--file', '/proj/compose.yml', 'restart', 'api']])
  await ui.unmount()
})

test('Docker fora do ar: só o erro e o que fazer, sem serviço velho nem ação', async ($, on) => {
  const answer = { out: FIVE, error: '' }
  fakeDocker(on, answer)
  await $.command.run(DOCKER)
  const ui = await $.ui.mount({ plugin: PANE, surface: 'terminal', component: 'Pane', requestId: PANE, props: props(3) })
  expect(await ui.find({ type: 'Text', text: /up/ })).toBeDefined()

  answer.error = 'Cannot connect to the Docker daemon at unix:///var/run/docker.sock.'
  await $.tool.call({ tool: 'mcp__tesseract-docker__services' })
  await ui.redraw()
  expect(await ui.find({ type: 'Text', text: /Cannot connect to the Docker daemon/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /Start Docker/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /▸ up/ })).toBeUndefined()
  const keys = keysOf(await ui.findAll({ type: 'Button' }))
  expect(keys).toEqual(Array.from({ length: 12 }, (_, index) => `row:${index}`))
  // Nenhuma tecla tem ação aqui: todas ficam no painel em vez de irem para o
  // prompt (era `u2r` digitado na tela de erro virando texto para o modelo).
  expect(sunk(await ui.findAll({ type: 'Button' }))).toBe('abcdefghijklmnopqrstuvwxyz0123456789')

  // Consertou: a lista volta e o erro some.
  answer.error = ''
  await $.tool.call({ tool: 'mcp__tesseract-docker__services' })
  await ui.redraw()
  expect(await ui.find({ type: 'Text', text: /Cannot connect/ })).toBeUndefined()
  expect(await ui.find({ type: 'Text', text: /▸ up/ })).toBeDefined()
  await ui.unmount()
})

test('célula estreita: nome com reticências, uma linha de teclas, nada além do corpo', async ($, on) => {
  fakeDocker(on, { out: [row('a-very-long-service-name'), row('db')].join('\n'), error: '' })
  await $.command.run(DOCKER)
  const ui = await $.ui.mount({ plugin: PANE, surface: 'terminal', component: 'Pane', requestId: PANE, props: props(3, 30) })

  expect((await ui.find({ key: 'row:0' }))?.text).toBe('a-very-long-…')
  expect(keysOf(await ui.findAll({ type: 'Button' })).slice(12)).toEqual(['up', 'stop', 'restart'])
  await ui.unmount()
})
