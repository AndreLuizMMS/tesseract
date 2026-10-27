import type { EngineInterface, Register, SessionRateLimit } from 'claude-code'

// tesseract-link: a sessão do Claude Code contando ao engine do Tesseract o
// que antes ele adivinhava lendo a tela da célula.
//
//   mod → engine   `ts report <json>`, um processo por evento
//   engine → mod   `ts link`, um processo que vive com a sessão e imprime
//                  uma linha JSON por prompt mandado do mosaico (e uma
//                  quando a linha é atada, para o mod dizer em que pé está)
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
// O resumo de uma linha (opção `summarize`): tamanho que cabe no cabeçalho da
// célula, quanto da resposta vai para o modelo e quanto o aviso espera por ele.
const SUMMARY_LIMIT = 60
const SUMMARY_INPUT_LIMIT = 4000
const SUMMARY_TIMEOUT_MS = 6000
// Como o Claude Code marca (CLAUDE_BG_SOURCE) a sessão de fundo que é a
// conversa da célula mandada para trás; a reserva vem como "spare".
const MOVED_FROM_CELL = 'slash'

// Estado do módulo. Some num reload, e tudo bem: o próximo turno refaz.
// asked são as chamadas de tool que o engine mandou perguntar e ainda não
// terminaram; isWaiting diz que o diálogo chegou a ser mostrado à pessoa.
const asked = new Set<string>()
let isWaiting = false
// O que o último diálogo espera, para repetir ao engine se a linha for refeita
// com ele ainda aberto.
let waiting: Pick<Payload, 'detail' | 'ask'> = {}
// isTurnRunning, unread e isStopped dizem em que estado a célula fica quando
// ninguém está esperando: trabalhando, com uma resposta ainda não avisada,
// interrompida pela pessoa, ou ociosa no prompt.
let isTurnRunning = false
let unread: Pick<Payload, 'detail' | 'summary'> | undefined
let isStopped = false
// Quando volta a janela que a última medição mostrou cheia; vai junto do
// relato de limite, para o engine saber até quando a conta fica de fora.
let limitResetsAt: string | undefined
// Último preenchimento de contexto mandado: só a mudança é reportada.
let reportedContext: number | undefined
let reportedSession = ''
let cell: string | undefined
// A sessão roda no serviço de fundo: o processo da célula é só o visor dela.
let isBackground = false
// A sessão acabou de vez: a linha com o engine não é refeita.
let isOver = false

type Payload = {
  kind: 'state' | 'session' | 'quota' | 'limit' | 'context' | 'title' | 'delivered'
  delivery?: string
  refused?: boolean
  state?: 'working' | 'idle' | 'answered' | 'approve' | 'interrupted' | 'ended'
  detail?: string
  // O que o approve espera: só "tool" se resolve do mosaico com Enter e Esc.
  ask?: 'tool' | 'question' | 'plan'
  summary?: string
  session?: string
  percent?: number
  resetsAt?: string
  weekly?: number
  context?: number
}

// inCell diz se a sessão fala pela célula do Tesseract. O ambiente não muda
// durante a sessão, então a resposta positiva é lida uma vez.
//
// As sessões de fundo do Claude Code nascem do processo da célula e herdam o
// ambiente e os mods dele. São duas espécies, e só uma é a conversa da célula:
//
//   - a conversa mandada para trás com ← (ou /bg) é um fork da que a célula
//     mostrava, e é nela que a conversa continua: esc traz ela de volta para a
//     tela e o processo da célula vira só o visor dela, sem evento nenhum.
//     Essa fala, e o engine passa a guardar o id dela;
//   - a reserva que o serviço de fundo deixa aquecida, e o que for despachado
//     da lista de agentes, não são o que a célula mostra. Se falassem, o engine
//     trocaria o id da conversa da célula pelo delas e mandaria para elas os
//     prompts do mosaico: ficam mudas.
async function inCell($: EngineInterface): Promise<boolean> {
  if (cell !== undefined) return true
  if ((await $.env.get('TESSERACT')) !== '1') return false
  isBackground = (await $.env.get('CLAUDE_CODE_SESSION_KIND')) === 'bg'
  if (isBackground && (await $.env.get('CLAUDE_BG_SOURCE')) !== MOVED_FROM_CELL) return false
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

// askKind diz que tipo de espera é: a aprovação de uma ferramenta é um sim ou
// não, que o mosaico resolve sem entrar na célula; pergunta com opções e plano
// para revisar pedem a pessoa dentro dela.
export function askKind(tool: string): 'tool' | 'question' | 'plan' {
  return tool === 'AskUserQuestion' ? 'question' : tool === 'ExitPlanMode' ? 'plan' : 'tool'
}

// summaryLine limpa o que o modelo devolveu: uma linha, sem aspas nem ponto
// final, no tamanho do cabeçalho.
export function summaryLine(text: string): string {
  const flat = text.replace(/\s+/g, ' ').trim().replace(/^["'`]+|["'`.]+$/g, '')
  return flat.length > SUMMARY_LIMIT ? `${flat.slice(0, SUMMARY_LIMIT - 1)}…` : flat
}

// summarize pede ao modelo mais barato a resposta em uma linha. Qualquer
// falha (sem rede, modelo bloqueado, prazo) devolve undefined e o aviso sai
// como sairia sem a opção.
async function summarize($: EngineInterface, answer: string): Promise<string | undefined> {
  try {
    const reply = await $.model.complete({
      model: 'haiku',
      system: `Summarize the assistant message in one line of at most ${SUMMARY_LIMIT} characters: what was done or decided, in the message's own language. Reply with the line only.`,
      prompt: answer.slice(0, SUMMARY_INPUT_LIMIT),
      maxTokens: 60,
      effort: 'low',
      timeoutMs: SUMMARY_TIMEOUT_MS,
    })
    const line = reply.isAnswered ? summaryLine(reply.text) : ''
    return line === '' ? undefined : line
  } catch {
    return undefined
  }
}

// reportRest diz ao engine em que pé a sessão está fora de uma espera: no
// meio de um turno, com uma resposta para ler, interrompida, ou ociosa no
// prompt. A resposta é avisada uma vez; o resto pode ser repetido.
async function reportRest($: EngineInterface): Promise<void> {
  if (isTurnRunning) {
    await report($, { kind: 'state', state: 'working' })
    return
  }
  if (unread === undefined) {
    await report($, { kind: 'state', state: isStopped ? 'interrupted' : 'idle' })
    return
  }
  const said = unread
  unread = undefined
  await report($, { kind: 'state', state: 'answered', ...said })
}

// reportStanding repete o estado ao engine quando a linha é atada: os relatos
// de estado só valem com ela de pé, e é assim que uma célula recém-criada,
// parada no prompt, aparece ociosa em vez de trabalhando.
async function reportStanding($: EngineInterface): Promise<void> {
  if (isWaiting) await report($, { kind: 'state', state: 'approve', ...waiting })
  else await reportRest($)
}

// reportQuota manda as janelas da assinatura: a de 5 horas, que é a do selo
// do mosaico, e a de 7 dias junto.
async function reportQuota($: EngineInterface, limits: readonly SessionRateLimit[]): Promise<void> {
  const window = limits.find(limit => limit.kind === 'five_hour')
  if (window === undefined) return
  const weekly = limits.find(limit => limit.kind === 'seven_day')?.percentUsed
  await report($, { kind: 'quota', percent: window.percentUsed, resetsAt: window.resetsAt, weekly })
}

// fullWindowReset é o reset mais distante entre as janelas que chegaram a
// 100%: a conta só volta quando todas as cheias voltarem.
function fullWindowReset(limits: readonly SessionRateLimit[]): string | undefined {
  return limits
    .filter(limit => limit.percentUsed >= 100 && limit.resetsAt !== undefined)
    .map(limit => limit.resetsAt as string)
    .sort((a, b) => Date.parse(a) - Date.parse(b))
    .at(-1)
}

// reportContext manda o preenchimento da janela de contexto, que é da
// conversa desta célula e de mais ninguém. Só a mudança de ponto inteiro.
async function reportContext($: EngineInterface, percent: number | undefined): Promise<void> {
  if (percent === undefined) return
  const context = Math.round(percent)
  if (context === reportedContext) return
  reportedContext = context
  await report($, { kind: 'context', context })
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
  let delivery: { id?: unknown; text?: unknown; expires?: unknown; event?: unknown }
  try {
    delivery = JSON.parse(line) as typeof delivery
  } catch {
    return
  }
  const { id, text, expires } = delivery
  if (delivery.event === 'attached') {
    await reportStanding($)
    return
  }
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
  // O resumo é texto da conversa: sem `notifyDetail` ele também não sai.
  const withSummary = options.summarize === true && withDetail

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
    // Conversa nova, janela vazia: a próxima medição só vem depois de um turno.
    if (withQuota && e.source === 'clear') await reportContext($, 0)
    return result
  })

  on('session.end', async ($, e, next) => {
    asked.clear()
    isWaiting = false
    isTurnRunning = false
    isStopped = false
    unread = undefined
    // Num /clear ou /resume a sessão continua com outra conversa: nada fica
    // esperando. Nos outros casos o processo está indo embora — e o de uma
    // sessão de fundo não é o da célula, que fica de pé na lista de agentes.
    const continues = e.reason === 'clear' || e.reason === 'resume'
    isOver = !continues
    await report($, { kind: 'state', state: continues || isBackground ? 'idle' : 'ended' })
    return next(e)
  })

  on('turn.start', async ($, e, next) => {
    const started = await next(e)
    isTurnRunning = true
    isStopped = false
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
    isStopped = e.isAborted || e.reason === 'aborted' || detail === ''
    unread = isStopped ? undefined : { detail: withDetail ? detail : '' }
    // O resumo custa uma chamada ao modelo e segura o aviso até ela voltar:
    // só para resposta de verdade, e só com a opção ligada.
    if (unread !== undefined && withSummary && e.reason === 'answer' && (await inCell($))) {
      const summary = await summarize($, e.answer)
      // A pessoa já mandou outro prompt enquanto o resumo vinha: o turno novo
      // é o que vale, e esta resposta não fica para avisar depois.
      if (isTurnRunning) {
        unread = undefined
        return done
      }
      unread = { ...unread, summary }
    }
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
      waiting = { detail: withDetail ? describeAsk(e.tool_name, e.tool_input) : '', ask: askKind(e.tool_name) }
      await report($, { kind: 'state', state: 'approve', ...waiting })
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

  // O limite de uso da conta acabou e o turno parou no meio: o engine leva a
  // conversa para a próxima conta da lista, quando houver uma livre.
  on('classic.StopFailure', async ($, e, next) => {
    const result = await next(e)
    if (e.error === 'rate_limit') await report($, { kind: 'limit', resetsAt: limitResetsAt })
    return result
  })

  on('session.measure', async ($, e, next) => {
    const echoed = await next(e)
    limitResetsAt = fullWindowReset(e.rateLimits)
    if (withQuota) {
      await reportQuota($, e.rateLimits)
      await reportContext($, e.context.percent)
    }
    return echoed
  })
}
