import type { On } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'

import { answerOpening, askKind, describeAsk, summaryLine } from '../hooks/register'

type Report = Record<string, unknown>

const CELL = { TESSERACT: '1', TESSERACT_CELL_ID: 'c1', TESSERACT_BIN: '/opt/ts' }
const SESSION = '0b0e7e0c-58a5-4b6e-9f0c-0f6f3c1d2e3f'

// world põe o chão debaixo do mod: o ambiente da célula, o `ts` falso que
// guarda cada relato, e a linha engine → mod entregando `lines`.
// `isBound: false` é a célula com a conversa mandada para o fundo: o processo
// só lista agentes, não tem sessão montada, e entregar rejeita na hora.
function world(on: On, env: Record<string, string>, lines: readonly string[] = [], isBound = true) {
  const unbound = () => new Error('not available in this mode: no session is bound in this process')
  const reports: Report[] = []
  // Quantas vezes a linha foi aberta, e por quanto tempo cada uma vive.
  const link = { opened: 0, livesMs: 0 }
  const prompts: unknown[] = []
  const commands: unknown[] = []
  mock.env(on, env)
  const clock = mock.clock(on)
  on('process.run', (_$, e) => {
    expect(e.argv.slice(0, 2)).toEqual(['/opt/ts', 'report'])
    reports.push(JSON.parse(e.argv[2] ?? '{}') as Report)
    return { value: { exitCode: 0, stdout: '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })
  on('process.spawn', async function* (_$, e) {
    expect(e.argv).toEqual(['/opt/ts', 'link'])
    link.opened++
    if (link.livesMs > 0) await clock.sleep(link.livesMs)
    // Uma entrega chega partida no meio da linha: o mod tem de juntar.
    for (const line of lines) {
      const whole = `${line}\n`
      yield { stream: 'stdout' as const, text: whole.slice(0, 5) }
      yield { stream: 'stdout' as const, text: whole.slice(5) }
    }
    return { value: { code: 0, signal: null } }
  })
  on('prompt.submit', (_$, e) => {
    if (!isBound) throw unbound()
    prompts.push({ text: e.text, origin: e.origin })
    return { text: e.text }
  })
  on('command.list', () => ({ value: [{ name: 'rename', description: 'Rename the conversation', source: 'builtin' as const }] }))
  on('command.run', (_$, e) => {
    if (!isBound) throw unbound()
    commands.push({ command: e.command, args: e.args })
    return { text: 'renamed' }
  })
  on('session.id', () => ({ value: SESSION }))
  on('classic.SessionStart', () => ({}))
  on('classic.PermissionRequest', () => ({}))
  on('classic.StopFailure', () => ({}))
  on('session.start', (_$, e) => ({ cwd: e.cwd }))
  on('session.end', (_$, e) => ({ sessionId: e.sessionId }))
  on('turn.start', (_$, e) => ({ turnId: e.turnId }))
  on('turn.complete', (_$, e) => ({ text: e.answer }))
  on('session.measure', (_$, e) => ({ changed: e.changed }))
  on('tool.check', () => ({ decision: 'ask' as const }))
  on('tool.call', () => ({ result: { stdout: '', stderr: '' } }))
  return { reports, prompts, commands, clock, link }
}

const states = (reports: readonly Report[]) =>
  reports.filter(r => r.kind === 'state').map(r => (r.detail ? `${String(r.state)}|${String(r.detail)}` : r.state))

test('fora de uma célula o mod não fala com ninguém', async ($, on) => {
  const { reports } = world(on, { TESSERACT: '1' })
  await $.classic.SessionStart({ source: 'startup', session_id: SESSION })
  await $.turn.start({ text: 'oi', turnId: 't1' })
  await $.turn.complete({ answer: 'pronto', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  expect(reports).toEqual([])
})

test('o turno vira trabalhando, aprovação com o comando, e respondeu', async ($, on) => {
  const { reports } = world(on, CELL)
  await $.turn.start({ text: 'migra o banco', turnId: 't1' })
  await $.tool.check({ tool: 'Bash', input: { command: 'pnpm migrate' }, tool_use_id: 'tu1' })
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: { command: 'pnpm\n  migrate' } })
  expect(states(reports)).toEqual(['working', 'approve|Bash: pnpm migrate'])

  // A chamada só termina depois de aprovada e executada: a espera acabou.
  await $.tool.call({ tool: 'Bash', command: 'pnpm migrate', tool_use_id: 'tu1' })
  expect(states(reports).at(-1)).toBe('working')

  await $.turn.complete({
    answer: '\nMigrei as 3 tabelas.\nDetalhes abaixo.',
    durationMs: 1,
    isAborted: false,
    turnId: 't1',
    reason: 'answer',
  })
  expect(states(reports).at(-1)).toBe('answered|Migrei as 3 tabelas.')
  expect(typeof reports.at(-1)?.at).toBe('number')
})

test('diálogo de agente em segundo plano segura a espera depois do fim do turno', async ($, on) => {
  const { reports } = world(on, CELL)
  await $.turn.start({ text: 'roda em segundo plano', turnId: 't1' })
  await $.tool.check({ tool: 'Bash', input: { command: 'make' }, tool_use_id: 'bg1', agentId: 'a1' })
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: { command: 'make' }, agent_id: 'a1' })
  await $.turn.complete({ answer: 'Disparei o agente.', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  expect(states(reports)).toEqual(['working', 'approve|Bash: make'])

  await $.tool.call({ tool: 'Bash', command: 'make', tool_use_id: 'bg1', agentId: 'a1' } as never)
  expect(states(reports).at(-1)).toBe('answered|Disparei o agente.')
})

test('turno interrompido não avisa que respondeu', async ($, on) => {
  const { reports } = world(on, CELL)
  await $.turn.complete({ answer: '', durationMs: 1, isAborted: true, turnId: 't1', reason: 'aborted' })
  expect(states(reports)).toEqual(['interrupted'])
})

test('recusar o diálogo com Esc não é resposta e não deixa aviso', async ($, on) => {
  const { reports } = world(on, CELL)
  await $.turn.start({ text: 'apaga a pasta', turnId: 't1' })
  await $.tool.check({ tool: 'Bash', input: { command: 'rm -rf build' }, tool_use_id: 'tu1' })
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: { command: 'rm -rf build' } })
  await $.tool.call({ tool: 'Bash', command: 'rm -rf build', tool_use_id: 'tu1' })
  // A recusa encerra o turno como "answer", sem texto e sem marca de interrupção.
  await $.turn.complete({ answer: '', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  expect(states(reports)).toEqual(['working', 'approve|Bash: rm -rf build', 'working', 'interrupted'])
  // O turno seguinte tira a marca: interrompido é do turno que parou.
  await $.turn.start({ text: 'então lista', turnId: 't2' })
  await $.turn.complete({ answer: 'Listei.', durationMs: 1, isAborted: false, turnId: 't2', reason: 'answer' })
  expect(states(reports).slice(-2)).toEqual(['working', 'answered|Listei.'])
})

test('o approve diz o que espera: só ferramenta se resolve do mosaico', async ($, on) => {
  const { reports } = world(on, CELL)
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: { command: 'pnpm migrate' } })
  await $.classic.PermissionRequest({ tool_name: 'AskUserQuestion', tool_input: { questions: [{ question: 'A ou B?' }] } })
  await $.classic.PermissionRequest({ tool_name: 'ExitPlanMode', tool_input: { plan: 'x' } })
  expect(reports.map(r => r.ask)).toEqual(['tool', 'question', 'plan'])
  expect(askKind('mcp__gh__issue')).toBe('tool')
})

test('linha atada: a sessão diz em que pé está, e parada no prompt é ociosa', async ($, on) => {
  const { reports } = world(on, CELL, [JSON.stringify({ text: '', event: 'attached' })])
  await $.session.start({ cwd: '/tmp', surface: 'terminal', isInteractive: true })
  for (let turn = 0; turn < 500 && states(reports).length < 1; turn++) {
    await $.session.measure({ context: { window: 1 }, rateLimits: [], changed: ['cost'] })
  }
  expect(states(reports)).toEqual(['idle'])
})

test('linha refeita com o diálogo aberto repete o que espera', async ($, on) => {
  const { reports, clock } = world(on, CELL, [JSON.stringify({ text: '', event: 'attached' })])
  await $.turn.start({ text: 'migra', turnId: 't1' })
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: { command: 'pnpm migrate' } })
  await $.session.start({ cwd: '/tmp', surface: 'terminal', isInteractive: true })
  await clock.settle()
  for (let turn = 0; turn < 500 && states(reports).length < 3; turn++) {
    await $.session.measure({ context: { window: 1 }, rateLimits: [], changed: ['cost'] })
  }
  expect(states(reports)).toEqual(['working', 'approve|Bash: pnpm migrate', 'approve|Bash: pnpm migrate'])
  expect(reports.at(-1)?.ask).toBe('tool')
})

test('resumo ligado: a resposta vai em uma linha, do modelo mais barato', { options: { summarize: true } }, async ($, on) => {
  const { reports } = world(on, CELL)
  const asked: { model: string; prompt: string }[] = []
  on('model.complete', (_$, e) => {
    asked.push({ model: e.model, prompt: e.prompt })
    return {
      value: {
        isAnswered: true as const,
        text: '"Migrei as 3 tabelas e rodei os testes."\n',
        usage: { input_tokens: 1, output_tokens: 1, cache_creation_input_tokens: 0, cache_read_input_tokens: 0 },
      },
    }
  })
  await $.turn.start({ text: 'migra', turnId: 't1' })
  await $.turn.complete({ answer: 'Pronto.\nMigrei tudo.', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  expect(asked).toEqual([{ model: 'haiku', prompt: 'Pronto.\nMigrei tudo.' }])
  expect(reports.at(-1)?.state).toBe('answered')
  expect(reports.at(-1)?.detail).toBe('Pronto.')
  expect(reports.at(-1)?.summary).toBe('Migrei as 3 tabelas e rodei os testes')

  // Turno interrompido não tem o que resumir.
  await $.turn.complete({ answer: '', durationMs: 1, isAborted: true, turnId: 't2', reason: 'aborted' })
  expect(asked.length).toBe(1)
})

test('resumo que falha não segura nem muda o aviso', { options: { summarize: true } }, async ($, on) => {
  const { reports } = world(on, CELL)
  on('model.complete', () => ({
    value: {
      isAnswered: false as const,
      reason: 'aborted' as const,
      usage: { input_tokens: 0, output_tokens: 0, cache_creation_input_tokens: 0, cache_read_input_tokens: 0 },
    },
  }))
  await $.turn.complete({ answer: 'Pronto.', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  expect(states(reports)).toEqual(['answered|Pronto.'])
  expect(reports.at(-1)?.summary).toBe(undefined)
})

test('sem a opção nenhum modelo é chamado', async ($, on) => {
  const { reports } = world(on, CELL)
  let calls = 0
  on('model.complete', () => {
    calls++
    throw new Error('not asked for')
  })
  await $.turn.complete({ answer: 'Pronto.', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  expect(calls).toBe(0)
  expect(states(reports)).toEqual(['answered|Pronto.'])
})

test('o resumo sai limpo e no tamanho do cabeçalho', () => {
  expect(summaryLine('  "Fixed the login\n redirect."  ')).toBe('Fixed the login redirect')
  expect(summaryLine('x'.repeat(200)).length).toBe(60)
})

test('sem detalhe configurado, resposta vazia continua sem aviso', { options: { notifyDetail: false } }, async ($, on) => {
  const { reports } = world(on, CELL)
  await $.turn.complete({ answer: ' \n', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  expect(states(reports)).toEqual(['interrupted'])
})

test('a linha com o engine é refeita sem desistir, com espera crescente e teto', async ($, on) => {
  const { clock, link } = world(on, CELL)
  await $.session.start({ cwd: '/tmp', surface: 'terminal', isInteractive: true })
  await clock.settle()
  expect(link.opened).toBe(1)

  // Quedas seguidas: 2 s, 4 s, 8 s, 16 s, 32 s e daí 60 s para sempre.
  for (const wait of [2000, 4000, 8000, 16000, 32000, 60000, 60000, 60000]) {
    const before = link.opened
    await clock.advance(wait - 1)
    expect(link.opened).toBe(before)
    await clock.advance(1)
    expect(link.opened).toBe(before + 1)
  }

  // Uma conexão que durou zera a espera: a próxima queda volta em 2 s.
  link.livesMs = 15_000
  await clock.advance(60_000)
  const stable = link.opened
  link.livesMs = 0
  await clock.advance(15_000)
  expect(link.opened).toBe(stable)
  await clock.advance(2000)
  expect(link.opened).toBe(stable + 1)

  // Sessão encerrada: ninguém refaz a linha.
  await $.session.end({ reason: 'prompt_input_exit', sessionId: SESSION, resume: { id: SESSION } })
  await clock.advance(600_000)
  expect(link.opened).toBe(stable + 1)
})

test('turno de subagente não encerra o turno da célula', async ($, on) => {
  const { reports } = world(on, CELL)
  await $.turn.complete({
    answer: 'relatório',
    durationMs: 1,
    isAborted: false,
    turnId: 't1',
    reason: 'answer',
    agentId: 'a1',
  })
  expect(reports).toEqual([])
})

test('sem detalhe configurado nenhum texto da conversa sai', { options: { notifyDetail: false } }, async ($, on) => {
  const { reports } = world(on, CELL)
  await $.classic.PermissionRequest({ tool_name: 'Bash', tool_input: { command: 'cat .env' } })
  await $.turn.complete({ answer: 'segredo', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  expect(states(reports)).toEqual(['approve', 'answered'])
})

test('o id da conversa é reportado quando muda, e só quando muda', async ($, on) => {
  const { reports } = world(on, CELL)
  const other = SESSION.replace('0b0e', 'ffff')
  await $.classic.SessionStart({ source: 'startup', session_id: SESSION })
  await $.classic.SessionStart({ source: 'resume', session_id: SESSION })
  await $.session.end({ reason: 'clear', sessionId: SESSION, resume: { id: SESSION } })
  await $.classic.SessionStart({ source: 'clear', session_id: other })
  expect(reports.filter(r => r.kind === 'session').map(r => r.session)).toEqual([SESSION, other])
  // O /clear não deixa nada esperando, e a janela de contexto recomeça vazia;
  // sair de vez encerra.
  expect(states(reports)).toEqual(['idle'])
  expect(reports.filter(r => r.kind === 'context').map(r => r.context)).toEqual([0])
  await $.session.end({ reason: 'prompt_input_exit', sessionId: other, resume: { id: other } })
  expect(states(reports).at(-1)).toBe('ended')
})

test('a medição leva as duas janelas e o contexto, este só quando muda', async ($, on) => {
  const { reports } = world(on, CELL)
  const measure = (percent: number | undefined) =>
    $.session.measure({
      context: { window: 1000000, percent },
      rateLimits: [
        { kind: 'seven_day', percentUsed: 47 },
        { kind: 'five_hour', percentUsed: 14.5, resetsAt: '2026-10-08T21:10:00.000Z' },
      ],
      changed: ['rateLimits'],
    })
  await measure(undefined)
  expect(reports.map(({ at: _at, ...rest }) => rest)).toEqual([
    { kind: 'quota', percent: 14.5, resetsAt: '2026-10-08T21:10:00.000Z', weekly: 47 },
  ])
  await measure(33.4)
  await measure(33.1)
  await measure(34)
  expect(reports.filter(r => r.kind === 'context').map(r => r.context)).toEqual([33, 34])
})

test('limite de uso vira relato de limite, com o reset da janela cheia', async ($, on) => {
  const { reports } = world(on, CELL)
  await $.classic.StopFailure({ error: 'overloaded' })
  await $.session.measure({
    context: { window: 1000000 },
    rateLimits: [
      { kind: 'seven_day', percentUsed: 61, resetsAt: '2026-10-12T00:00:00.000Z' },
      { kind: 'five_hour', percentUsed: 100, resetsAt: '2026-10-08T21:10:00.000Z' },
    ],
    changed: ['rateLimits'],
  })
  await $.classic.StopFailure({ error: 'rate_limit' })
  expect(reports.filter(r => r.kind === 'limit').map(({ at: _at, ...rest }) => rest)).toEqual([
    { kind: 'limit', resetsAt: '2026-10-08T21:10:00.000Z' },
  ])
})

test('quota desligada não reporta', { options: { quota: false } }, async ($, on) => {
  const { reports } = world(on, CELL)
  await $.session.measure({
    context: { window: 1000000 },
    rateLimits: [{ kind: 'five_hour', percentUsed: 14 }],
    changed: ['rateLimits'],
  })
  expect(reports).toEqual([])
})

test('o que vem do mosaico entra como fala da pessoa, e barra é comando', async ($, on) => {
  const { reports, prompts, commands } = world(on, CELL, [
    JSON.stringify({ id: '1', text: 'cobre o menu mobile', expires: Date.now() + 60_000 }),
    JSON.stringify({ id: '2', text: '/rename' }),
    JSON.stringify({ id: '3', text: 'segundo pedido' }),
    'isto não é json',
    JSON.stringify({ id: '4', text: '   ' }),
    // Vencida: o engine já desistiu do recibo e colou o texto na célula.
    JSON.stringify({ id: '5', text: 'chegou tarde', expires: Date.now() - 1 }),
  ])
  await $.session.start({ cwd: '/tmp', surface: 'terminal', isInteractive: true })
  // A linha roda solta depois do hook: cada ida ao engine cede a vez a ela.
  // (Uma medição sem janela de 5 horas não gera relato nenhum.)
  for (let turn = 0; turn < 500 && reports.filter(r => r.kind === 'delivered').length < 3; turn++) {
    await $.session.measure({ context: { window: 1 }, rateLimits: [], changed: ['cost'] })
  }

  expect(prompts.map(p => (p as { text: string }).text)).toEqual(['cobre o menu mobile', 'segundo pedido'])
  expect((prompts[0] as { origin: { asUser?: boolean } }).origin.asUser).toBe(true)
  expect(commands).toEqual([{ command: 'rename', args: '' }])
  // O rename pedido pelo engine avisa quando terminou, para o título ser adotado.
  expect(reports.filter(r => r.kind === 'title').length).toBe(1)
  // Cada entrega aceita tem recibo, na ordem; a vazia e a vencida não têm, e
  // é a falta dele que faz o engine colar o texto em vez de perdê-lo.
  expect(reports.filter(r => r.kind === 'delivered').map(r => r.delivery)).toEqual(['1', '2', '3'])
})

test('sessão sem conversa na tela recusa a entrega em vez de engolir', async ($, on) => {
  const lines = [JSON.stringify({ id: '7', text: 'cobre o menu mobile' }), JSON.stringify({ id: '8', text: '/rename' })]
  const { reports } = world(on, CELL, lines, false)
  await $.session.start({ cwd: '/tmp', surface: 'terminal', isInteractive: true })
  const receipts = () => reports.filter(r => r.kind === 'delivered')
  for (let turn = 0; turn < 500 && receipts().length < 2; turn++) {
    await $.session.measure({ context: { window: 1 }, rateLimits: [], changed: ['cost'] })
  }
  expect(receipts().map(r => [r.delivery, r.refused])).toEqual([['7', true], ['8', true]])
  expect(reports.filter(r => r.kind === 'title')).toEqual([])
})

test('reserva do serviço de fundo herda o ambiente da célula e fica muda', async ($, on) => {
  const { reports, prompts, link } = world(on, { ...CELL, CLAUDE_CODE_SESSION_KIND: 'bg', CLAUDE_BG_SOURCE: 'spare' }, [
    JSON.stringify({ id: '1', text: 'não é para mim' }),
  ])
  await $.session.start({ cwd: '/tmp', surface: 'terminal', isInteractive: true })
  await $.classic.SessionStart({ source: 'startup', session_id: SESSION })
  await $.turn.start({ text: 'oi', turnId: 't1' })
  await $.turn.complete({ answer: 'pronto', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  // Nem relato (trocaria o id da conversa da célula pelo dela) nem linha
  // (tomaria os prompts da sessão que a célula mostra).
  expect(reports).toEqual([])
  expect(link.opened).toBe(0)
  expect(prompts).toEqual([])
})

test('conversa mandada para o fundo com ← continua falando pela célula', async ($, on) => {
  // ← faz um fork da conversa numa sessão do serviço de fundo, e esc traz
  // esse fork para a tela da célula: é ele que roda os turnos dali em diante.
  const { reports, prompts, link } = world(on, { ...CELL, CLAUDE_CODE_SESSION_KIND: 'bg', CLAUDE_BG_SOURCE: 'slash' }, [
    JSON.stringify({ id: '1', text: 'agora é com você' }),
  ])
  await $.session.start({ cwd: '/tmp', surface: 'terminal', isInteractive: true })
  await $.turn.start({ text: 'oi', turnId: 't1' })
  await $.turn.complete({ answer: 'mango', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  for (let turn = 0; turn < 500 && prompts.length < 1; turn++) {
    await $.session.measure({ context: { window: 1 }, rateLimits: [], changed: ['cost'] })
  }
  // O id do fork é o que o engine guarda, o turno aparece e o prompt chega.
  expect(reports.find(r => r.kind === 'session')?.session).toBe(SESSION)
  expect(reports.some(r => r.state === 'answered' && r.detail === 'mango')).toBe(true)
  expect(link.opened).toBe(1)
  expect(prompts.length).toBe(1)
  // Encerrada, ela sai da tela mas o processo da célula fica: nada de "ended".
  await $.session.end({ reason: 'other', sessionId: SESSION })
  expect(reports.at(-1)).toMatchObject({ kind: 'state', state: 'idle' })
})

test('o resumo do que espera cabe numa linha de aviso', () => {
  expect(describeAsk('Bash', { command: 'pnpm   migrate\n--force' })).toBe('Bash: pnpm migrate --force')
  expect(describeAsk('Edit', { file_path: '/src/a.ts', old_string: 'x' })).toBe('Edit: /src/a.ts')
  expect(describeAsk('AskUserQuestion', { questions: [{ question: 'A ou B?' }] })).toBe('question: A ou B?')
  expect(describeAsk('ExitPlanMode', { plan: 'x' })).toBe('plan ready for review')
  expect(describeAsk('mcp__gh__issue', null)).toBe('mcp__gh__issue')
  expect(describeAsk('Bash', { command: 'x'.repeat(500) }).length).toBe(160)
  expect(answerOpening('\n\n  primeira linha  \nsegunda')).toBe('primeira linha')
})
