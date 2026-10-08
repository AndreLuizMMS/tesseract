import type { On, PromptEditInput, RenderElement } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'
import type { Engine } from 'claude-code/testing'

import { askLine, cut, duration, fitBand, hintCopy, toolLine } from '../hooks/format'

const PLUGIN = 'tesseract-skin'
const WIDE = { columns: 80, rows: 30 }
const NARROW = { columns: 34, rows: 30 }

// Responde no lugar do Claude Code. O `ui.render` daqui representa o desenho
// original: devolve as props que recebeu, para o teste ver uma reescrita.
const engine = (on: On, variables: Record<string, string> = { TESSERACT: '1' }) => {
  mock.env(on, variables)
  const clock = mock.clock(on, { now: 10_000 })
  on('session.cwd', () => ({ value: '/work/api' }))
  on('turn.start', ($, e) => ({ turnId: e.turnId }))
  on('turn.complete', () => ({ text: '' }))
  on('tool.check', () => ({ decision: 'ask' as const }))
  on('session.measure', ($, e) => ({ changed: e.changed }))
  on('classic.PermissionRequest', () => ({}))
  on('classic.SessionStart', () => ({}))
  on('prompt.edit', ($, e) => ({ text: e.text + e.inputText, cursor: e.cursor + e.inputText.length }))
  on('session.usage', () => ({
    value: { startedAt: 0, context: { window: 200_000, percent: 12 }, rateLimits: [{ kind: 'five_hour', percentUsed: 30 }] },
  }))
  on('ui.render', ($, e) => ({
    type: 'Text' as const,
    props: {},
    children: [`engine ${JSON.stringify(e.props)}`],
  }))

  return clock
}

const band = (bodyColumns: number) => ({
  plugin: PLUGIN,
  surface: 'terminal' as const,
  component: 'AbovePrompt' as const,
  requestId: 'above-prompt',
  viewport: { columns: bodyColumns + 5, rows: 30 },
  props: {
    hasSurvey: false,
    isWorking: false,
    maxRows: 10,
    bodyColumns,
    scroll: { offset: 0, bodyRows: 10 },
    view: {},
  },
})

const toolUse = (over: Record<string, unknown> = {}, viewport = WIDE) => ({
  plugin: PLUGIN,
  surface: 'terminal' as const,
  component: 'ToolUse' as const,
  requestId: String(over.tool_use_id ?? 'toolu_1'),
  viewport,
  props: {
    tool_use_id: 'toolu_1',
    tool: 'Bash',
    input: { command: 'pnpm migrate --name add_orders_table_and_backfill' },
    isRunning: false,
    isErrored: false,
    isInterrupted: false,
    ...over,
  },
})

// Todo texto da árvore, e toda prop de estilo que ela usa.
const flatten = (node: RenderElement | string): { text: string; props: Record<string, unknown>[] } => {
  if (typeof node === 'string') return { text: node, props: [] }
  const children = ('children' in node ? (node.children ?? []) : []) as (RenderElement | string)[]
  const parts = children.map(flatten)

  return {
    text: parts.map(part => part.text).join(''),
    props: [('props' in node ? node.props : {}) as Record<string, unknown>, ...parts.flatMap(part => part.props)],
  }
}

const startTurn = ($: Engine) => $.turn.start({ text: 'go', turnId: 't1' })

const DONE = { output: { stdout: '', stderr: '' } }
const OUTSIDE = { options: { outside_tesseract: true } }

// A chamada pergunta à pessoa e o diálogo abre.
const ask = async ($: Engine, id = 'toolu_1') => {
  await $.tool.check({ tool: 'Bash', input: {}, tool_use_id: id })
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: {} })
}

// A pessoa digita uma tecla no prompt. O kit levanta o evento, mas a tipagem
// do substantivo `prompt` não o lista.
const typeKey = ($: Engine) =>
  ($.prompt as unknown as { edit: (input: PromptEditInput) => Promise<unknown> }).edit({
    origin: { kind: 'composer' },
    text: '',
    cursor: 0,
    start: 0,
    end: 0,
    inputText: 'x',
  })

const mountOf = ($: Engine, component: string, props: Record<string, unknown>) =>
  $.ui.mount({ plugin: PLUGIN, surface: 'terminal', requestId: 'r1', viewport: WIDE, component, props } as never)

test('texto: duração, corte, cópia da dica, resumo de ferramenta', () => {
  expect(duration(3_400)).toBe('3s')
  expect(duration(64_000)).toBe('1m 4s')
  expect(duration(7_500_000)).toBe('2h 5m')
  expect(cut('tesseract', 5)).toBe('tess…')
  expect(hintCopy('? for shortcuts · esc to interrupt')).toBe('? shortcuts · Esc interrupts')
  expect(hintCopy('← 1 agent')).toBe('← 1 agent')
  expect(hintCopy('(shift+tab to cycle)')).toBe('Shift+Tab cycles')
  expect(hintCopy('shift+tab to cycle · esc to interrupt')).toBe('Shift+Tab cycles · Esc interrupts')
  expect(toolLine('Read', { file_path: '/work/api/go.mod' }, '/work/api')).toEqual({ name: 'Read', argument: 'go.mod' })
  expect(toolLine('Bash', { command: 'a\nb' }, '/work')).toEqual({ name: 'Bash', argument: 'a …' })
  expect(toolLine('mcp__github__get_me', { q: 'x' }, '/work')).toEqual({ name: 'github:get_me', argument: 'x' })
  expect(toolLine('Agent', { prompt: 'x' }, '/work')).toBeUndefined()
  expect(askLine('Bash', { command: 'pnpm migrate' }, '/work')).toEqual({
    what: 'Bash: pnpm migrate',
    keys: 'Enter approves, Esc declines',
  })
  expect(askLine('AskUserQuestion', { questions: [{ question: 'tabs or spaces?' }] }, '/work')).toEqual({
    what: 'tabs or spaces?',
    keys: 'Enter answers, Esc declines',
  })
  expect(askLine('ExitPlanMode', { plan: 'x' }, '/work').what).toBe('plan ready for review')
  expect(askLine('Agent', null, '/work').what).toBe('Agent')
})

test('faixa: quota sai antes do estado, rótulo é cortado por último', () => {
  expect(fitBand(['api/claude', '⬤ RESPONDED'], ['5h 23%', 'ctx 41%'], 80)).toEqual([
    'api/claude',
    '⬤ RESPONDED',
    '5h 23%',
    'ctx 41%',
  ])
  expect(fitBand(['api/claude', '⬤ RESPONDED'], ['5h 23%', 'ctx 41%'], 34)).toEqual([
    'api/claude',
    '⬤ RESPONDED',
    '5h 23%',
  ])
  expect(fitBand(['api/claude-refactor-long', '⬤ RESPONDED'], ['5h 23%'], 27)).toEqual([
    'api/claude-r…',
    '⬤ RESPONDED',
  ])
})

test('cada site redesenhado devolve árvore válida no terminal', async ($, on) => {
  engine(on)
  await startTurn($)

  const sites = [
    [{ component: 'Spinner', props: { word: 'Baking', message: null, suffix: '…', mode: 'thinking' } }, /^▸ WORKING · thinking · 0s$/],
    [{ component: 'TurnDuration', props: { word: 'Baked', durationMs: 42_000 } }, /^⬤ RESPONDED · 42s$/],
    [{ component: 'SessionMode', props: { modes: ['focus', 'memory paused'] } }, /^FOCUS · MEMORY PAUSED$/],
    [
      {
        component: 'UserMessage',
        props: { text: 'run the tests', origin: { kind: 'composer' }, isExpanded: false },
      },
      /^› run the tests$/,
    ],
    [
      {
        component: 'ToolGroup',
        props: {
          isActive: false,
          isExpanded: false,
          calls: [
            { tool_use_id: 'a', tool: 'Read', input: {}, isRunning: false, isErrored: false, isInterrupted: false },
            { tool_use_id: 'b', tool: 'Read', input: {}, isRunning: false, isErrored: true, isInterrupted: false },
            { tool_use_id: 'c', tool: 'Grep', input: {}, isRunning: false, isErrored: false, isInterrupted: false },
          ],
        },
      },
      /^✖ Read ×2 · Grep ×1 · 1 failed$/,
    ],
  ] as const

  for (const [site, expected] of sites) {
    const ui = await $.ui.mount({ plugin: PLUGIN, surface: 'terminal', requestId: 'r1', viewport: WIDE, ...site } as never)
    expect(flatten(await ui.drawn()).text).toMatch(expected)
    await ui.unmount()
  }

  const running = await $.ui.mount(toolUse({ isRunning: true }))
  expect(flatten(await running.drawn()).text).toBe('▸ Bash pnpm migrate --name add_orders_table_and_backfill')
  await running.redraw({ ...toolUse().props, isErrored: true })
  expect(flatten(await running.drawn()).text).toMatch(/^✖ Bash .* · failed$/)
  await running.redraw({ ...toolUse().props, isInterrupted: true, isErrored: true })
  expect(flatten(await running.drawn()).text).toMatch(/^○ Bash .* · interrupted$/)
  await running.unmount()

  const top = await $.ui.mount(band(75))
  expect(flatten(await top.drawn()).text).toBe('⧉ api · ▸ WORKING')
  await $.session.measure({
    context: { window: 200_000, tokens: 82_000, percent: 41 },
    rateLimits: [{ kind: 'five_hour', percentUsed: 23.4 }],
    changed: ['context', 'rateLimits'],
  })
  await $.turn.complete({ turnId: 't1', answer: 'ok', durationMs: 42_000, isAborted: false, reason: 'answer' })
  await top.redraw()
  expect(flatten(await top.drawn()).text).toBe('⧉ api · ⬤ RESPONDED · 5h 23% · ctx 41%')
  await top.unmount()
})

test('o que o mod não redesenha fica com o Claude Code', async ($, on) => {
  engine(on)

  // Ferramenta com linha viva própria, grupo aberto, mensagem que não é prompt.
  const agent = await $.ui.mount(toolUse({ tool: 'Agent', input: { prompt: 'x' } }))
  expect(flatten(await agent.drawn()).text).toStartWith('engine ')
  await agent.unmount()

  const group = await $.ui.mount({
    plugin: PLUGIN,
    surface: 'terminal',
    component: 'ToolGroup',
    requestId: 'g1',
    viewport: WIDE,
    props: {
      isActive: false,
      isExpanded: true,
      calls: [{ tool_use_id: 'toolu_1', tool: 'Bash', input: {}, isRunning: false, isErrored: false, isInterrupted: false }],
    },
  })
  expect(flatten(await group.drawn()).text).toStartWith('engine ')
  await group.unmount()
  // A linha de uma chamada agrupada traz o resultado embutido: fica original.
  const row = await $.ui.mount(toolUse())
  expect(flatten(await row.drawn()).text).toStartWith('engine ')
  await row.unmount()

  const notice = await $.ui.mount({
    plugin: PLUGIN,
    surface: 'terminal',
    component: 'UserMessage',
    requestId: 'm1',
    viewport: WIDE,
    props: { text: 'task done', origin: { kind: 'task-notification' }, isExpanded: false },
  } as never)
  expect(flatten(await notice.drawn()).text).toStartWith('engine ')
  await notice.unmount()
})

test('a dica do prompt é reescrita, não redesenhada', async ($, on) => {
  engine(on)
  const ui = await $.ui.mount({
    plugin: PLUGIN,
    surface: 'terminal',
    component: 'PromptHint',
    requestId: 'prompt-hint',
    viewport: WIDE,
    props: { isDraft: false, isWorking: true, hint: 'esc to interrupt' },
  })
  expect(flatten(await ui.drawn()).text).toContain('"hint":"Esc interrupts"')
  await ui.unmount()
})

test('a dica reescrita leva junto o que outro mod acrescentou', async ($, on) => {
  engine(on)
  const hint = (props: Record<string, unknown>) => mountOf($, 'PromptHint', { isDraft: false, isWorking: false, ...props })

  // Reescrever `hint` faz o Claude Code descartar `tail`: ele entra no texto.
  const rewritten = await hint({ hint: '? for shortcuts', tail: '/docker /md /open /find' })
  const text = flatten(await rewritten.drawn()).text
  expect(text).toContain('"hint":"? shortcuts · /docker /md /open /find"')
  expect(text).not.toContain('"tail"')
  await rewritten.unmount()

  // Nada a reescrever: a linha e o `tail` seguem como vieram.
  const kept = await hint({ hint: '← 1 agent', tail: '/find' })
  expect(flatten(await kept.drawn()).text).toContain('"hint":"← 1 agent","tail":"/find"')
  await kept.unmount()
})

test('fora do Tesseract o mod não age; a opção libera', async ($, on) => {
  engine(on, {})
  const ui = await $.ui.mount(band(75))
  expect(flatten(await ui.drawn()).text).toStartWith('engine ')
  await ui.unmount()
})

test('fora do Tesseract com a opção ligada', { options: { outside_tesseract: true } }, async ($, on) => {
  engine(on, {})
  const ui = await $.ui.mount(band(75))
  expect(flatten(await ui.drawn()).text).toBe('⧉ api')
  await ui.unmount()
})

test('superfície desligada fica com o Claude Code', { options: { band: false } }, async ($, on) => {
  engine(on)
  const ui = await $.ui.mount(band(75))
  expect(flatten(await ui.drawn()).text).toStartWith('engine ')
  await ui.unmount()
  // As outras continuam.
  const end = await $.ui.mount({
    plugin: PLUGIN,
    surface: 'terminal',
    component: 'TurnDuration',
    requestId: 'd1',
    viewport: WIDE,
    props: { word: 'Baked', durationMs: 3_000 },
  })
  expect(flatten(await end.drawn()).text).toBe('⬤ RESPONDED · 3s')
  await end.unmount()
})

test('sem cor: nenhuma cor na árvore, a forma continua', async ($, on) => {
  engine(on, { TESSERACT: '1', NO_COLOR: '1' })
  await startTurn($)
  await $.tool.check({ tool: 'Bash', input: {}, tool_use_id: 'toolu_1' })
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: {} })

  const drawings = [
    await $.ui.mount(band(75)),
    await $.ui.mount(toolUse()),
    await $.ui.mount(toolUse({ tool_use_id: 'toolu_2', isErrored: true })),
    await $.ui.mount({
      plugin: PLUGIN,
      surface: 'terminal',
      component: 'UserMessage',
      requestId: 'm1',
      viewport: WIDE,
      props: { text: 'go', origin: { kind: 'composer' }, isExpanded: false },
    } as never),
  ]
  for (const ui of drawings) {
    for (const props of flatten(await ui.drawn()).props) {
      expect(props.color).toBeUndefined()
      expect(props.backgroundColor).toBeUndefined()
    }
  }

  // Dentro da célula a área invertida é a do mosaico: aqui só forma e negrito.
  const [, bar, failed] = drawings
  const approve = flatten(await bar!.drawn())
  expect(approve.text).toStartWith('⏵ APPROVE  Bash: pnpm migrate')
  expect(approve.props.some(props => props.inverse === true)).toBe(false)
  expect(approve.props[1]).toMatchObject({ bold: true })
  expect(flatten(await failed!.drawn()).text).toMatch(/^✖ Bash .* · failed$/)
  for (const ui of drawings) await ui.unmount()
})

test('largura 34: faixa e barra cabem na linha', async ($, on) => {
  engine(on, { TESSERACT: '1', TESSERACT_CELL: 'api/claude-refactor-of-everything' })
  await $.session.measure({
    context: { window: 200_000, tokens: 82_000, percent: 41 },
    rateLimits: [{ kind: 'five_hour', percentUsed: 23 }],
    changed: ['context', 'rateLimits'],
  })
  await startTurn($)
  await $.turn.complete({ turnId: 't1', answer: 'ok', durationMs: 1_000, isAborted: false, reason: 'answer' })

  // A faixa recebe 29: a largura da célula menos as cinco do `[-]`.
  const top = await $.ui.mount(band(29))
  const text = flatten(await top.drawn()).text
  expect([...text].length).toBeLessThanOrEqual(29)
  expect(text).toStartWith('⧉ api/')
  expect(text).toEndWith('· ⬤ RESPONDED')
  await top.unmount()

  await $.tool.check({ tool: 'Bash', input: {}, tool_use_id: 'toolu_1' })
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: {} })
  const bar = await $.ui.mount(toolUse({}, NARROW))
  const line = flatten(await bar.drawn()).text
  expect([...line].length).toBe(33)
  expect(line).toStartWith('⏵ APPROVE  Bash: pnpm migra')
  await bar.unmount()
})

test('largura 34 fora do mosaico: a barra ocupa a linha', OUTSIDE, async ($, on) => {
  engine(on, { NO_COLOR: '1' })
  await ask($)
  const bar = await $.ui.mount(toolUse({}, NARROW))
  const drawn = flatten(await bar.drawn())
  expect([...drawn.text].length).toBe(33)
  expect(drawn.text).toStartWith(' ⏵ APPROVE  Bash: pnpm migr')
  // Sem cor, a única área invertida da tela.
  expect(drawn.props[0]).toMatchObject({ inverse: true, bold: true })
  await bar.unmount()
})

test('dentro da célula o approve é forma e cor, sem área e sem pisca', async ($, on) => {
  const clock = engine(on)
  await startTurn($)
  const ui = await $.ui.mount(toolUse())
  // Antes de o diálogo abrir a chamada está em curso, não concluída.
  expect(flatten(await ui.drawn()).text).toStartWith('▸ Bash ')

  await ask($)
  await ui.redraw()
  const line = flatten(await ui.drawn())
  expect(line.text).toBe(
    '⏵ APPROVE  Bash: pnpm migrate --name add_orders_table_and_backfill',
  )
  expect(line.props.some(props => props.backgroundColor !== undefined || props.inverse !== undefined)).toBe(false)
  expect(line.props[1]).toMatchObject({ color: '#FFB454', bold: true })

  // A barra que pisca é a do cabeçalho do mosaico: aqui nada muda com o tempo.
  await clock.advance(2000)
  await ui.redraw()
  expect(flatten(await ui.drawn()).props.some(props => props.backgroundColor !== undefined)).toBe(false)
  await ui.unmount()

  // Com espaço, a linha diz as teclas.
  const short = await $.ui.mount(toolUse({ input: { command: 'pnpm migrate' } }))
  expect(flatten(await short.drawn()).text).toBe('⏵ APPROVE  Bash: pnpm migrate  Enter approves, Esc declines')
  await short.unmount()
})

test('fila de aprovações: só a primeira pede, a segunda espera a vez', async ($, on) => {
  const clock = engine(on)
  await startTurn($)
  await ask($, 'toolu_1')
  await ask($, 'toolu_2')
  const first = await $.ui.mount(toolUse())
  const second = await $.ui.mount(toolUse({ tool_use_id: 'toolu_2' }))
  expect(flatten(await first.drawn()).text).toStartWith('⏵ APPROVE')
  expect(flatten(await second.drawn()).text).toStartWith('▸ Bash ')

  // A primeira foi aprovada e passou a rodar: a vez é da segunda.
  await first.redraw({ ...toolUse().props, isRunning: true })
  expect(flatten(await first.drawn()).text).toStartWith('▸ Bash ')
  await clock.settle()
  await second.redraw()
  expect(flatten(await second.drawn()).text).toStartWith('⏵ APPROVE')
  await first.unmount()
  await second.unmount()
})

test('esperando aprovação o spinner não diz working, e o tempo de espera não conta', async ($, on) => {
  const clock = engine(on)
  await startTurn($)
  const spinner = await mountOf($, 'Spinner', { word: 'Baking', message: null, suffix: '…', mode: 'tool-use' })
  await clock.advance(3000)
  await spinner.redraw()
  expect(flatten(await spinner.drawn()).text).toBe('▸ WORKING · tool · 3s')

  await ask($)
  await clock.advance(50_000)
  await spinner.redraw()
  expect(flatten(await spinner.drawn()).text).toBe('Blocked, waiting on you.')

  // Aprovada: a linha da chamada passa a rodar e o relógio retoma de onde parou.
  const row = await $.ui.mount(toolUse({ isRunning: true }))
  await clock.settle()
  await clock.advance(1000)
  await spinner.redraw()
  expect(flatten(await spinner.drawn()).text).toBe('▸ WORKING · tool · 4s')
  await row.unmount()
  await spinner.unmount()
})

test('faixa: o desfecho do turno cai quando a pessoa volta ao prompt', async ($, on) => {
  engine(on)
  await startTurn($)
  await $.turn.complete({ turnId: 't1', answer: 'ok', durationMs: 1_000, isAborted: false, reason: 'answer' })
  const top = await $.ui.mount(band(75))
  expect(flatten(await top.drawn()).text).toBe('⧉ api · ⬤ RESPONDED')

  await typeKey($)
  await top.redraw()
  expect(flatten(await top.drawn()).text).toBe('⧉ api')
  await top.unmount()
})

test('faixa: recusar o diálogo ou interromper não é responded', async ($, on) => {
  engine(on)
  const top = await $.ui.mount(band(75))
  await startTurn($)
  await ask($)
  // A recusa com Esc encerra o turno como "answer", sem texto.
  await $.turn.complete({ turnId: 't1', answer: '', durationMs: 1_000, isAborted: false, reason: 'answer' })
  await top.redraw()
  expect(flatten(await top.drawn()).text).toBe('⧉ api · ○ INTERRUPTED')

  await startTurn($)
  await $.turn.complete({ turnId: 't1', answer: 'meio', durationMs: 1_000, isAborted: true, reason: 'aborted' })
  await top.redraw()
  expect(flatten(await top.drawn()).text).toBe('⧉ api · ○ INTERRUPTED')
  await top.unmount()
})

test('fim de turno: a linha diz o desfecho, e a antiga não muda com o turno seguinte', async ($, on) => {
  engine(on)
  const row = (requestId: string) =>
    $.ui.mount({
      plugin: PLUGIN,
      surface: 'terminal',
      component: 'TurnDuration',
      requestId,
      viewport: WIDE,
      props: { word: 'Baked', durationMs: 4_000 },
    })

  // Linha de um turno que o mod não viu acabar (sessão retomada).
  const old = await row('d0')
  expect(flatten(await old.drawn()).text).toBe('⬤ RESPONDED · 4s')

  await startTurn($)
  await $.turn.complete({ turnId: 't1', answer: '', durationMs: 4_000, isAborted: false, reason: 'answer' })
  await old.redraw()
  expect(flatten(await old.drawn()).text).toBe('⬤ RESPONDED · 4s')
  const declined = await row('d1')
  expect(flatten(await declined.drawn()).text).toBe('○ INTERRUPTED · 4s')

  await startTurn($)
  await $.turn.complete({ turnId: 't1', answer: 'ok', durationMs: 4_000, isAborted: false, reason: 'answer' })
  const answered = await row('d2')
  expect(flatten(await answered.drawn()).text).toBe('⬤ RESPONDED · 4s')
  await declined.redraw()
  expect(flatten(await declined.drawn()).text).toBe('○ INTERRUPTED · 4s')

  await startTurn($)
  await $.turn.complete({ turnId: 't1', answer: '', durationMs: 4_000, isAborted: false, reason: 'error' })
  // A linha pode nascer antes de o turno fechar: ela espera o desfecho.
  await startTurn($)
  const failed = await row('d3')
  await $.turn.complete({ turnId: 't1', answer: '', durationMs: 4_000, isAborted: false, reason: 'error' })
  await failed.redraw()
  expect(flatten(await failed.drawn()).text).toBe('✖ FAILED · 4s')
  for (const ui of [old, declined, answered, failed]) await ui.unmount()
})

test('faixa: /clear recarrega a quota', async ($, on) => {
  engine(on)
  const top = await $.ui.mount(band(75))
  expect(flatten(await top.drawn()).text).toBe('⧉ api')
  await $.classic.SessionStart({ source: 'clear', session_id: 's2' })
  await top.redraw()
  expect(flatten(await top.drawn()).text).toBe('⧉ api · 5h 30% · ctx 12%')
  await top.unmount()
})

test('prompt mandado do mosaico é fala da pessoa', async ($, on) => {
  engine(on)
  const sent = await mountOf($, 'UserMessage', {
    text: 'cobre o menu mobile',
    origin: { kind: 'plugin', name: 'tesseract-link', asUser: true },
    isExpanded: false,
  })
  const drawn = flatten(await sent.drawn())
  expect(drawn.text).toBe('› cobre o menu mobile')
  expect(drawn.props[0]).toMatchObject({ backgroundColor: '#121C1F' })
  await sent.unmount()

  // Outro plugin falando continua com a atribuição do Claude Code.
  const other = await mountOf($, 'UserMessage', {
    text: 'oi',
    origin: { kind: 'plugin', name: 'other', asUser: true },
    isExpanded: false,
  })
  expect(flatten(await other.drawn()).text).toStartWith('engine ')
  await other.unmount()
})

test('fora do mosaico, barra de approve: só enquanto o diálogo espera, pisca a cada 2 s', OUTSIDE, async ($, on) => {
  const clock = engine(on, {})
  await startTurn($)

  const ui = await $.ui.mount(toolUse(DONE))
  // Sem diálogo, é uma linha comum.
  expect(flatten(await ui.drawn()).text).toStartWith('· Bash ')
  await ui.redraw(toolUse().props)

  await $.tool.check({ tool: 'Bash', input: {}, tool_use_id: 'toolu_1' })
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: {} })
  await ui.redraw()
  const bar = flatten(await ui.drawn())
  expect(bar.text).toBe(
    ' ⏵ APPROVE  Bash: pnpm migrate --name add_orders_table_and_backfill'.padEnd(79),
  )
  expect(bar.props[0]).toMatchObject({ backgroundColor: '#FFB454', color: '#070B0C', bold: true })

  // 2 s: quadro escuro por 200 ms, e ainda é barra (tem fundo).
  await clock.advance(2000)
  await ui.redraw()
  expect(flatten(await ui.drawn()).props[0]).toMatchObject({ backgroundColor: '#835F30', color: '#070B0C' })
  await clock.advance(200)
  await ui.redraw()
  expect(flatten(await ui.drawn()).props[0]).toMatchObject({ backgroundColor: '#FFB454' })

  // Aprovada: a chamada roda e a linha volta a ser working.
  await ui.redraw({ ...toolUse().props, isRunning: true })
  expect(flatten(await ui.drawn()).text).toStartWith('▸ Bash ')

  // Fim do turno limpa a fila mesmo se a chamada não terminou.
  await $.turn.complete({ turnId: 't1', answer: '', durationMs: 1_000, isAborted: true, reason: 'aborted' })
  await ui.redraw({ ...toolUse().props, isInterrupted: true, isErrored: true, output: 'interrupted' })
  expect(flatten(await ui.drawn()).text).toMatch(/^○ Bash .* · interrupted$/)
  await ui.unmount()
})

test('TESSERACT_NO_BLINK: a barra fica acesa', OUTSIDE, async ($, on) => {
  const clock = engine(on, { TESSERACT_NO_BLINK: '1' })
  await $.tool.check({ tool: 'Bash', input: {}, tool_use_id: 'toolu_1' })
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: {} })
  const ui = await $.ui.mount(toolUse())
  await clock.advance(2000)
  await ui.redraw()
  expect(flatten(await ui.drawn()).props[0]).toMatchObject({ backgroundColor: '#FFB454' })
  await ui.unmount()
})
