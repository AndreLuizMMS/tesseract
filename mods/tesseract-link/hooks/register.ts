import type { EngineInterface, Register, SessionRateLimit } from 'claude-code'

// tesseract-link: a sessão do Claude Code contando ao engine do Tesseract o
// que antes ele adivinhava lendo a tela da célula.
//
//   mod → engine   `ts report <json>`, um processo por evento
//   engine → mod   `ts link`, um processo que vive com a sessão e imprime
//                  uma linha JSON por prompt mandado do mosaico
//
// Nada sai da máquina: os dois caminhos terminam no socket local do engine.
// Fora de uma célula (sem TESSERACT=1 e TESSERACT_CELL_ID) o mod não faz nada.

// DETAIL_LIMIT acompanha o corte que o engine aplica de novo do lado dele.
const DETAIL_LIMIT = 160
// O engine responde em milissegundos; passar disso é engine travado, e o
// turno não espera por ele.
const REPORT_TIMEOUT_MS = 3000
// A linha com o engine é refeita enquanto a sessão viver: a espera dobra a
// cada queda seguida, até o teto, e volta ao começo depois de uma conexão que
// durou. Sem a linha o engine lê a tela, então esperar não quebra nada.
// Quanto o recibo de uma entrega espera pela recusa da sessão. Medida, ela
// chega em menos de 10 ms; o engine espera 2 s pelo recibo.
const REFUSAL_WAIT_MS = 250
const LINK_RETRY_MS = 2000
const LINK_RETRY_MAX_MS = 60_000
const LINK_STABLE_MS = 10_000

// Estado do módulo. Some num reload, e tudo bem: o próximo turno refaz.
// asked são as chamadas de tool que o engine mandou perguntar e ainda não
// terminaram; isWaiting diz que o diálogo chegou a ser mostrado à pessoa.
const asked = new Set<string>()
let isWaiting = false
// isTurnRunning e unread dizem em que estado a célula fica quando ninguém
// está esperando: trabalhando, ou com uma resposta ainda não avisada.
let isTurnRunning = false
let unread: string | undefined
let reportedSession = ''
let cell: string | undefined
// A sessão acabou de vez: a linha com o engine não é refeita.
let isOver = false

type Payload = {
  kind: 'state' | 'session' | 'quota' | 'title' | 'delivered'
  delivery?: string
  refused?: boolean
  state?: 'working' | 'answered' | 'approve' | 'ended'
  detail?: string
  session?: string
  percent?: number
  resetsAt?: string
}

// inCell diz se a sessão é a que a célula do Tesseract mostra. O ambiente
// não muda durante a sessão, então a resposta positiva é lida uma vez.
//
// Uma sessão de fundo do Claude Code (a conversa mandada para trás com ←, e a
// reserva que o serviço de fundo deixa aquecida) nasce do processo da célula
// e herda o ambiente e os mods dele, mas não é o que a célula mostra. Se ela
// falasse, o engine trocaria o id da conversa da célula pelo dela e mandaria
// para ela os prompts do mosaico: fica muda.
async function inCell($: EngineInterface): Promise<boolean> {
  if (cell !== undefined) return true
  if ((await $.env.get('TESSERACT')) !== '1') return false
  if ((await $.env.get('CLAUDE_CODE_SESSION_KIND')) === 'bg') return false
  cell = (await $.env.get('TESSERACT_CELL_ID')) || undefined
  return cell !== undefined
}

async function binary($: EngineInterface): Promise<string> {
  return (await $.env.get('TESSERACT_BIN')) || 'ts'
}

// report conta uma coisa ao engine. Falhar é normal (engine reiniciando,
// binário trocado) e nunca pode derrubar o turno: sem o relato, o engine
// continua lendo a tela.
async function report($: EngineInterface, payload: Payload): Promise<void> {
  if (!(await inCell($))) return
  try {
    const body = JSON.stringify({ ...payload, at: Date.now() })
    await $.process.run([await binary($), 'report', body], { timeoutMs: REPORT_TIMEOUT_MS })
  } catch {
    // de propósito: ver comentário acima
  }
}

// reportSession manda o id real da conversa, só quando muda. /clear, /resume
// e /branch trocam o id por baixo da célula; sem isto um reboot retomaria a
// conversa antiga.
async function reportSession($: EngineInterface, session: string): Promise<void> {
  if (session === '' || session === reportedSession) return
  reportedSession = session
  await report($, { kind: 'session', session })
}

function oneLine(text: string): string {
  const flat = text.replace(/\s+/g, ' ').trim()
  return flat.length > DETAIL_LIMIT ? `${flat.slice(0, DETAIL_LIMIT - 1)}…` : flat
}

// describeAsk resume o que está esperando a pessoa: a ferramenta e o que ela
// quer fazer, ou a pergunta que o agente fez.
export function describeAsk(tool: string, input: unknown): string {
  const args = (typeof input === 'object' && input !== null ? input : {}) as Record<string, unknown>
  if (tool === 'AskUserQuestion') {
    const first = Array.isArray(args.questions) ? (args.questions[0] as { question?: unknown } | undefined) : undefined
    return oneLine(typeof first?.question === 'string' ? `question: ${first.question}` : 'question')
  }
  if (tool === 'ExitPlanMode') return 'plan ready for review'
  for (const key of ['command', 'file_path', 'url', 'pattern', 'description']) {
    const value = args[key]
    if (typeof value === 'string' && value !== '') return oneLine(`${tool}: ${value}`)
  }
  return tool
}

// answerOpening é a primeira linha com conteúdo da resposta.
export function answerOpening(answer: string): string {
  return oneLine(answer.split('\n').find(line => line.trim() !== '') ?? '')
}

// reportRest diz ao engine em que pé a sessão está fora de uma espera: no
// meio de um turno, com uma resposta para ler, ou parada.
async function reportRest($: EngineInterface): Promise<void> {
  if (isTurnRunning || unread === undefined) {
    await report($, { kind: 'state', state: 'working' })
    return
  }
  const detail = unread
  unread = undefined
  await report($, { kind: 'state', state: 'answered', detail })
}

// reportQuota manda a janela de 5 horas, que é a que o selo do mosaico mostra.
async function reportQuota($: EngineInterface, limits: readonly SessionRateLimit[]): Promise<void> {
  const window = limits.find(limit => limit.kind === 'five_hour')
  if (window === undefined) return
  await report($, { kind: 'quota', percent: window.percentUsed, resetsAt: window.resetsAt })
}

// accept recebe o que veio do mosaico e põe na fila da sessão. Uma linha que
// abre com barra é comando; o resto entra como fala da própria pessoa, atrás
// do turno em curso — sem colar no terminal nem esperar o agente digerir a
// cola.
//
// O engine só dá a entrega por feita quando ouve o recibo daqui. Sem recibo
// no prazo ele cola o texto no terminal, e por isso uma entrega vencida é
// descartada: rodar as duas mandaria o prompt em dobro. O recibo também diz
// quando a sessão recusou — com a conversa mandada para o fundo (←) este
// processo só mostra a lista de agentes e não tem sessão para receber nada —
// e aí o engine avisa a pessoa em vez de colar o texto numa tela que não é a
// da conversa.
async function accept($: EngineInterface, line: string): Promise<void> {
  let delivery: { id?: unknown; text?: unknown; expires?: unknown }
  try {
    delivery = JSON.parse(line) as typeof delivery
  } catch {
    return
  }
  const { id, text, expires } = delivery
  if (typeof text !== 'string' || text.trim() === '') return

  const command = /^\/([\w:.-]+)(?:\s+([\s\S]*))?$/.exec(text.trim())
  const name = command?.[1] ?? ''
  let isRefused = false
  try {
    // Barra seguida de algo que a sessão não conhece é texto, não comando.
    const isCommand = name !== '' && (await $.command.list()).some(known => known.name === name)
    if (typeof expires === 'number' && Date.now() > expires) return
    // A entrega em si espera a sessão ficar ociosa e não é aguardada: o
    // recibo sai antes, e a linha seguinte não fica presa atrás desta.
    const queued = isCommand
      ? $.command.run({ command: name, args: command?.[2] ?? '' }).then(async () => {
          // O engine pediu o rename e está esperando para adotar o título.
          if (name === 'rename') await report($, { kind: 'title' })
        })
      : $.prompt.submit({ text, asUser: true })
    // A recusa de quem não tem sessão montada chega em milissegundos; uma
    // entrega aceita só resolve quando a sessão fica ociosa. O recibo espera
    // a primeira das duas: a recusa, ou o tempo de ela ter chegado.
    await Promise.race([
      queued.then(undefined, () => {
        isRefused = true
      }),
      $.clock.sleep(REFUSAL_WAIT_MS),
    ])
  } catch {
    isRefused = true
  }
  if (typeof id !== 'string') return
  await report($, isRefused ? { kind: 'delivered', delivery: id, refused: true } : { kind: 'delivered', delivery: id })
}

// holdLink mantém a linha engine → mod aberta enquanto a sessão viver. É
// também o sinal de vida do mod: quando o processo cai, o engine percebe a
// conexão fechar e volta a ler a tela.
async function holdLink($: EngineInterface): Promise<void> {
  const argv = [await binary($), 'link']
  let wait = LINK_RETRY_MS
  while (!isOver) {
    let pending = ''
    let startedAt = 0
    try {
      startedAt = await $.clock.now()
      for await (const piece of $.process.spawn({ argv })) {
        if (piece.stream !== 'stdout') continue
        pending += piece.text
        for (let end = pending.indexOf('\n'); end >= 0; end = pending.indexOf('\n')) {
          const line = pending.slice(0, end)
          pending = pending.slice(end + 1)
          await accept($, line)
        }
      }
    } catch {
      // o binário não subiu: tenta de novo abaixo
    }
    try {
      if ((await $.clock.now()) - startedAt >= LINK_STABLE_MS) wait = LINK_RETRY_MS
      await $.clock.sleep(wait)
      wait = Math.min(wait * 2, LINK_RETRY_MAX_MS)
    } catch {
      // o relógio da sessão acabou junto com ela
      return
    }
  }
}

export const register: Register = (on, options) => {
  const withDetail = options.notifyDetail !== false
  const withQuota = options.quota !== false

  on('session.start', async ($, e, next) => {
    const started = await next(e)
    if (await inCell($)) {
      await reportSession($, await $.session.id())
      isOver = false
      void holdLink($)
    }
    return started
  })

  // session.start não dispara de novo em /clear, /resume e /branch; o evento
  // clássico dispara em todos, e traz o id novo.
  on('classic.SessionStart', async ($, e, next) => {
    const result = await next(e)
    await reportSession($, e.session_id)
    return result
  })

  on('session.end', async ($, e, next) => {
    asked.clear()
    isWaiting = false
    isTurnRunning = false
    unread = undefined
    // Num /clear ou /resume a sessão continua com outra conversa: nada fica
    // esperando. Nos outros casos o processo está indo embora.
    const continues = e.reason === 'clear' || e.reason === 'resume'
    isOver = !continues
    await report($, { kind: 'state', state: continues ? 'working' : 'ended' })
    return next(e)
  })

  on('turn.start', async ($, e, next) => {
    const started = await next(e)
    isTurnRunning = true
    if (await inCell($)) {
      await reportSession($, await $.session.id())
      // Um diálogo de um agente em segundo plano segue na tela: o turno novo
      // não apaga a espera.
      if (!isWaiting) await report($, { kind: 'state', state: 'working' })
    }
    return started
  })

  on('turn.complete', async ($, e, next) => {
    const done = await next(e)
    // O turno de um subagente termina dentro do turno principal.
    if (e.agentId !== undefined) return done
    isTurnRunning = false
    const detail = e.reason === 'answer' ? answerOpening(e.answer) : `turn ended: ${e.reason}`
    // Quem interrompeu está olhando para a célula: não há o que avisar. Recusar
    // o diálogo com Esc é o mesmo gesto, e chega como "answer" sem texto: um
    // turno sem nada para ler nunca vira "respondeu".
    const isStopped = e.isAborted || e.reason === 'aborted' || detail === ''
    unread = isStopped ? undefined : withDetail ? detail : ''
    // Um agente em segundo plano pode ter deixado um diálogo aberto depois
    // de o turno principal acabar. A espera vale mais que a resposta, que é
    // avisada quando o diálogo for resolvido.
    if (isWaiting && asked.size > 0) return done
    isWaiting = false
    await reportRest($)
    return done
  })

  // O veredito "ask" ainda não é diálogo na tela: no modo automático quem
  // responde é o classificador. Aqui só se guarda qual chamada perguntou.
  on('tool.check', async ($, e, next) => {
    const verdict = await next(e)
    if (verdict.decision === 'ask' && e.tool_use_id !== undefined) asked.add(e.tool_use_id)
    return verdict
  })

  // Este é o momento em que o diálogo vai para a pessoa: aprovação de tool,
  // pergunta do agente e saída do modo plano passam todos por aqui.
  on('classic.PermissionRequest', async ($, e, next) => {
    const result = await next(e)
    // Um hook de settings que já decidiu dispensa o diálogo.
    if (result.decision === undefined) {
      isWaiting = true
      const detail = withDetail ? describeAsk(e.tool_name, e.tool_input) : ''
      await report($, { kind: 'state', state: 'approve', detail })
    }
    return result
  })

  // A chamada só resolve depois de aprovada e executada (ou negada): é o
  // primeiro sinal de que a espera acabou. Entre a aprovação e o fim de um
  // comando longo quem limpa o estado é o engine, pela tecla da resposta.
  on('tool.call', async ($, e, next) => {
    try {
      return await next(e)
    } finally {
      if (asked.delete(e.tool_use_id) && isWaiting && asked.size === 0) {
        isWaiting = false
        await reportRest($)
      }
    }
  })

  on('session.measure', async ($, e, next) => {
    const echoed = await next(e)
    if (withQuota) await reportQuota($, e.rateLimits)
    return echoed
  })
}
