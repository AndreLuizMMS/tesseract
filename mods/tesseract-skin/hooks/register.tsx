import { atom, read, update } from 'claude-code'
import type { EngineInterface, PluginOptions, Register, Timer } from 'claude-code'

import type { Quota, Turn, TurnPhase } from '../types'
import { askLine, cut, duration, fitBand, hintCopy, toolLine } from './format'
import { glyph, styles } from './tokens'

const turn = atom({ plugin: 'tesseract-skin', key: 'turn' } as const, { phase: 'idle' })
const now = atom({ plugin: 'tesseract-skin', key: 'now' } as const, 0)
const quota = atom({ plugin: 'tesseract-skin', key: 'quota' } as const, {})
const asking = atom({ plugin: 'tesseract-skin', key: 'asking' } as const, [])
const isDimmed = atom({ plugin: 'tesseract-skin', key: 'isDimmed' } as const, false)

// Ciclo do pisca, igual ao do mosaico: 1,8 s aceso, 200 ms escuro.
const BLINK_MS = 2000
const BLINK_DIM_MS = 200

// Como o spinner nomeia a fase do turno.
const PHASE = {
  requesting: 'requesting',
  thinking: 'thinking',
  responding: 'responding',
  'tool-input': 'tool',
  'tool-use': 'tool',
} as const

type Environment = {
  // TESSERACT=1, ou a opção de usar fora dele.
  isInside: boolean
  // Dentro de uma célula a barra sólida de approve é a do cabeçalho do
  // mosaico: a linha da chamada não inverte nem pisca (um badge por tela).
  hasMosaic: boolean
  noColor: boolean
  canBlink: boolean
  // projeto/célula, ou a pasta da sessão.
  label: string
  cwd: string
}

// Lido uma vez por carga do módulo: o ambiente não muda durante a sessão.
let environment: Promise<Environment> | undefined

const loadEnvironment = ($: EngineInterface, options: PluginOptions): Promise<Environment> =>
  (environment ??= (async () => {
    const [inside, noColor, noBlink, cell, cwd] = await Promise.all([
      $.env.get('TESSERACT'),
      $.env.get('NO_COLOR'),
      $.env.get('TESSERACT_NO_BLINK'),
      $.env.get('TESSERACT_CELL'),
      $.session.cwd(),
    ])
    const hasNoColor = noColor !== undefined && noColor !== ''

    return {
      isInside: inside === '1' || options.outside_tesseract === true,
      hasMosaic: inside === '1',
      noColor: hasNoColor,
      canBlink: !hasNoColor && noBlink === undefined && options.blink !== false,
      label: cell ?? cwd.split('/').filter(Boolean).pop() ?? cwd,
      cwd,
    }
  })())

const readQuota = (usage: {
  context: { percent?: number }
  rateLimits: ReadonlyArray<{ kind: string; percentUsed: number }>
}): Quota => ({
  fiveHour: usage.rateLimits.find(limit => limit.kind === 'five_hour')?.percentUsed,
  context: usage.context.percent,
})

// Chamadas agrupadas: quando o grupo abre (ctrl+o), cada linha traz o
// resultado embutido e só o desenho do Claude Code o mostra.
const grouped = new Set<string>()
// Última chamada de cada ferramenta que a checagem mandou perguntar à pessoa.
const undecided = new Map<string, string>()
// Como acabou o turno de cada linha de fim de turno já desenhada.
const verdicts = new Map<string, TurnPhase>()
// Desfecho do turno que acabou de fechar, à espera da linha dele.
let fresh: TurnPhase | undefined
// Chamadas aprovadas cuja saída da fila já está agendada.
const approved = new Set<string>()
let ticker: Timer | undefined
let blinker: Timer | undefined
let isAsking = false
// Quando a espera começou: o tempo de diálogo não conta como trabalho.
let askedAt: number | undefined
// A faixa ainda mostra o desfecho do último turno (responded, interrupted,
// failed). Cai na primeira edição do prompt: a pessoa voltou à sessão.
let hasVerdict = false

const isOn = async ($: EngineInterface, options: PluginOptions, surface?: string): Promise<boolean> =>
  (surface === undefined || options[surface] !== false) && (await loadEnvironment($, options)).isInside

// Tira chamadas da fila de aprovação (todas, sem `id`); vazia, o pisca para.
// Sem diálogo aberto não escreve nada: roda ao fim de toda chamada.
const stopAsking = async ($: EngineInterface, id?: string): Promise<void> => {
  if (!isAsking) return
  const left = await update($, asking, list => (id === undefined ? [] : list.filter(one => one !== id)))
  if (left.length > 0) return
  isAsking = false
  blinker?.cancel()
  blinker = undefined
  await update($, isDimmed, () => false)
  // O relógio do turno anda para a frente o tanto que o diálogo ficou aberto,
  // como a duração que o Claude Code mostra no fim.
  const waited = askedAt === undefined ? 0 : (await $.clock.now()) - askedAt
  askedAt = undefined
  if (waited > 0) await update($, turn, held => (held.since === undefined ? held : { ...held, since: held.since + waited }))
}

const loadQuota = async ($: EngineInterface): Promise<void> => {
  const usage = await $.session.usage()
  await update($, quota, () => readQuota(usage))
}

export const register: Register = (on, options) => {
  // ----- estado do turno -----

  on('session.start', async ($, e, next) => {
    if (await isOn($, options)) {
      await loadQuota($)
      // Num reload o módulo recomeça e o estado da sessão fica: uma fila de
      // aprovação antiga não teria mais quem a esvaziasse.
      if ((await read($, asking)).length > 0) await update($, asking, () => [])
    }

    return next(e)
  })

  // /clear, /resume e /branch zeram o estado da sessão sem novo session.start:
  // sem isto a faixa ficaria sem quota até a próxima medição.
  on('classic.SessionStart', async ($, e, next) => {
    const result = await next(e)
    if (await isOn($, options)) await loadQuota($).catch(() => undefined)

    return result
  })

  on('session.measure', async ($, e, next) => {
    if (await isOn($, options)) await update($, quota, () => readQuota(e))

    return next(e)
  })

  on('turn.start', async ($, e, next) => {
    if (await isOn($, options)) {
      const startedAt = await $.clock.now()
      hasVerdict = false
      await update($, now, () => startedAt)
      await update($, turn, (): Turn => ({ phase: 'working', since: startedAt }))
      ticker ??= $.clock.every(1000, () => {
        void $.clock.now().then(time => update($, now, () => time))
      })
    }

    return next(e)
  })

  on('turn.complete', async ($, e, next) => {
    // Turno de subagente não encerra o da sessão.
    if (e.agentId === undefined && (await isOn($, options))) {
      ticker?.cancel()
      ticker = undefined
      await stopAsking($)
      // Recusar o diálogo com Esc encerra o turno como "answer" sem texto: quem
      // parou foi a pessoa, não há resposta para ler.
      const isStopped = e.isAborted || e.reason === 'aborted' || (e.reason === 'answer' && e.answer.trim() === '')
      const phase: Turn['phase'] = isStopped ? 'interrupted' : e.reason === 'error' ? 'failed' : 'responded'
      hasVerdict = true
      fresh = phase
      await update($, turn, () => ({ phase }))
    }

    return next(e)
  })

  // O sinal de que a pessoa voltou à sessão é a primeira edição do prompt
  // depois do fim do turno (tecla, colagem ou movimento do cursor): a faixa
  // para de afirmar o desfecho, como o cabeçalho do mosaico ao entrar na célula.
  on('prompt.edit', ($, e, next) => {
    // A tecla passa primeiro: este evento tem 50 ms, e limpar a faixa não
    // pode segurar o que a pessoa digita.
    const edited = next(e)
    if (hasVerdict) {
      hasVerdict = false
      void update($, turn, () => ({ phase: 'idle' })).catch(() => undefined)
    }

    return edited
  })

  // ----- espera de aprovação -----
  //
  // Ordem medida numa sessão real: tool.call → tool.check (veredito "ask") →
  // classic.PermissionRequest → diálogo. Nenhum evento marca a resposta da
  // pessoa: o fim da espera é a linha da chamada passar a `isRunning`, ou a
  // chamada terminar (recusada).

  on('tool.check', async ($, e, next) => {
    const verdict = await next(e)
    // Chamada de subagente não tem linha no transcript principal.
    if (verdict.decision === 'ask' && e.tool_use_id !== undefined && e.agentId === undefined) {
      undecided.set(e.tool, e.tool_use_id)
    }

    return verdict
  })

  // O diálogo vai abrir: a chamada está bloqueada esperando a pessoa.
  on('classic.PermissionRequest', async ($, e, next) => {
    const id = undecided.get(e.tool_name)
    if (id !== undefined && (await isOn($, options, 'tools'))) {
      undecided.delete(e.tool_name)
      if (!isAsking) askedAt = await $.clock.now()
      isAsking = true
      await update($, asking, list => [...list, id])
      const { canBlink, hasMosaic } = await loadEnvironment($, options)
      if (canBlink && !hasMosaic) {
        blinker ??= $.clock.every(BLINK_MS, () => {
          void update($, isDimmed, () => true)
          $.clock.after(BLINK_DIM_MS, () => void update($, isDimmed, () => false))
        })
      }
    }

    return next(e)
  })

  on('tool.call', async ($, e, next) => {
    try {
      return await next(e)
    } finally {
      await stopAsking($, e.tool_use_id)
    }
  })

  // ----- faixa acima do prompt -----

  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    if (e.props.hasSurvey || !(await isOn($, options, 'band'))) return next(e)

    const { Text } = $.ui.resolve(e)
    const { noColor, label } = await loadEnvironment($, options)
    const style = styles(noColor)
    const width = Math.max(8, e.props.bodyColumns)
    const current = await read($, turn)

    const state = {
      idle: undefined,
      working: { mark: `${glyph.working} WORKING`, style: style.working },
      responded: { mark: `${glyph.responded} RESPONDED`, style: style.responded },
      interrupted: { mark: `${glyph.stopped} INTERRUPTED`, style: style.stopped },
      failed: { mark: `${glyph.crashed} FAILED`, style: style.crashed },
    }[current.phase]
    const used = await read($, quota)
    const figures = [
      used.fiveHour === undefined ? undefined : `5h ${Math.round(used.fiveHour)}%`,
      used.context === undefined ? undefined : `ctx ${Math.round(used.context)}%`,
    ].filter(figure => figure !== undefined)

    // Dois de largura para o glifo da marca e o espaço depois dele.
    const fixed = state === undefined ? [label] : [label, state.mark]
    const [name, ...rest] = fitBand(fixed, figures, width - 2)
    const stateText = state === undefined ? undefined : rest.shift()

    return (
      <Text wrap="truncate-end">
        <Text {...style.structure}>{glyph.brand} </Text>
        <Text {...style.label}>{name}</Text>
        {stateText !== undefined && state !== undefined && (
          <Text>
            <Text {...style.faint}> · </Text>
            <Text {...state.style}>{stateText}</Text>
          </Text>
        )}
        {rest.map(figure => (
          <Text>
            <Text {...style.faint}> · </Text>
            <Text {...(Number.parseInt(figure.split(' ')[1] ?? '', 10) >= 90 ? style.strong : style.muted)}>
              {figure}
            </Text>
          </Text>
        ))}
      </Text>
    )
  })

  // ----- linhas do turno -----

  on('ui.render', { component: 'Spinner' }, async ($, e, next) => {
    if (!(await isOn($, options, 'spinner'))) return next(e)

    const { Box, Text } = $.ui.resolve(e)
    const style = styles((await loadEnvironment($, options)).noColor)
    // Com o diálogo aberto ninguém está trabalhando, e o tempo não conta.
    if ((await read($, asking)).length > 0) {
      return (
        <Box marginTop={1}>
          <Text {...style.muted} wrap="truncate-end">
            Blocked, waiting on you.
          </Text>
        </Box>
      )
    }

    const current = await read($, turn)
    const elapsed = current.since === undefined ? '' : ` · ${duration((await read($, now)) - current.since)}`

    // Working é apagado de propósito: quem chama atenção é responded e approve.
    return (
      <Box marginTop={1}>
        <Text {...style.working} wrap="truncate-end">
          {glyph.working} WORKING · {e.props.message ?? PHASE[e.props.mode]}
          {elapsed}
        </Text>
      </Box>
    )
  })

  on('ui.render', { component: 'TurnDuration' }, async ($, e, next) => {
    if (!(await isOn($, options, 'turn_end'))) return next(e)

    const { Box, Text } = $.ui.resolve(e)
    const style = styles((await loadEnvironment($, options)).noColor)
    // A linha não diz como o turno acabou: quem sabe é o turn.complete, que
    // deixa o desfecho para a primeira linha nova. Guardado por linha, para as
    // antigas não mudarem; linha de turno que o mod não viu acabar (sessão
    // retomada, reload) fica como respondida. A leitura do turno é o que
    // redesenha a linha que nasceu antes de o turno fechar.
    const isOpen = (await read($, turn)).phase === 'working'
    let verdict = verdicts.get(e.requestId)
    if (verdict === undefined && !isOpen) {
      verdict = fresh ?? 'responded'
      fresh = undefined
      verdicts.set(e.requestId, verdict)
    }
    const [mark, markStyle] =
      verdict === 'interrupted'
        ? [`${glyph.stopped} INTERRUPTED`, style.stopped]
        : verdict === 'failed'
          ? [`${glyph.crashed} FAILED`, style.crashed]
          : [`${glyph.responded} RESPONDED`, style.responded]

    return (
      <Box marginTop={1}>
        <Text wrap="truncate-end">
          <Text {...markStyle}>{mark}</Text>
          <Text {...style.muted}> · {duration(e.props.durationMs)}</Text>
        </Text>
      </Box>
    )
  })

  // ----- ferramentas -----

  on('ui.render', { component: 'ToolUse' }, async ($, e, next) => {
    if (grouped.has(e.props.tool_use_id) || !(await isOn($, options, 'tools'))) return next(e)

    const { Text } = $.ui.resolve(e)
    const { noColor, cwd, hasMosaic } = await loadEnvironment($, options)
    const style = styles(noColor)
    const line = toolLine(e.props.tool, e.props.input, cwd)
    const id = e.props.tool_use_id
    const queue = await read($, asking)
    const isUnresolved = !e.props.isRunning && e.props.output === undefined

    // Aprovada: a chamada passou a rodar. Nenhum evento marca a resposta da
    // pessoa, então é aqui que a espera acaba (o fim da chamada pode demorar).
    // Desenhar é puro: a escrita sai daqui por um timer, uma vez por chamada.
    if (e.props.isRunning && queue.includes(id) && !approved.has(id)) {
      approved.add(id)
      $.clock.after(0, () => void stopAsking($, id).finally(() => approved.delete(id)))
    }

    // Só a primeira da fila pede aprovação: um badge por tela.
    if (queue[0] === id && isUnresolved) {
      const width = e.viewport?.columns ?? 80
      const waiting = askLine(e.props.tool, e.props.input, cwd)
      const what = waiting.what
      const keys = `  ${waiting.keys}`

      // Dentro da célula a área preenchida é a do cabeçalho do mosaico: aqui
      // ficam a forma ⏵ e o âmbar, sobre fundo normal e sem piscar.
      if (hasMosaic) {
        const mark = `${glyph.approve} APPROVE`
        const head = `${mark}  ${what}`
        const hasKeys = [...head].length + keys.length < width

        return (
          <Text wrap="truncate-end">
            <Text {...style.approveMark}>{mark}</Text>
            {cut(head, width - 1).slice(mark.length)}
            {hasKeys && <Text {...style.muted}>{keys}</Text>}
          </Text>
        )
      }

      // Fora do mosaico esta é a única barra: urgência é área preenchida, a
      // linha inteira invertida.
      const head = ` ${glyph.approve} APPROVE  ${what}`
      const text = [...head].length + keys.length < width ? head + keys : cut(head, width - 1)

      return (
        <Text {...((await read($, isDimmed)) ? style.approveDimmed : style.approve)} wrap="truncate-end">
          {text.padEnd(width - 1)}
        </Text>
      )
    }

    if (line === undefined) return next(e)

    // Forma antes de cor: cada desfecho tem glifo e palavra próprios.
    // Sem resultado e sem erro a chamada ainda não foi decidida (os instantes
    // antes de o diálogo abrir): é trabalho em curso, não chamada concluída.
    const [mark, markStyle, outcome] = e.props.isRunning || (isUnresolved && !e.props.isErrored && !e.props.isInterrupted)
      ? [glyph.working, style.working, '']
      : e.props.isInterrupted
        ? [glyph.stopped, style.stopped, ' · interrupted']
        : e.props.isErrored
          ? [glyph.crashed, style.crashed, ' · failed']
          : [glyph.done, style.faint, '']

    return (
      <Text wrap="truncate-end">
        <Text {...markStyle}>{mark} </Text>
        <Text {...style.label}>{line.name}</Text>
        {line.argument === '' ? '' : ` ${line.argument}`}
        <Text {...markStyle}>{outcome}</Text>
      </Text>
    )
  })

  on('ui.render', { component: 'ToolGroup' }, async ($, e, next) => {
    for (const call of e.props.calls) {
      if (call.tool_use_id !== undefined) grouped.add(call.tool_use_id)
    }
    if (e.props.isExpanded || !(await isOn($, options, 'tools'))) return next(e)

    const { Text } = $.ui.resolve(e)
    const style = styles((await loadEnvironment($, options)).noColor)
    const counts = new Map<string, number>()
    for (const call of e.props.calls) counts.set(call.tool, (counts.get(call.tool) ?? 0) + 1)
    const isRunning = e.props.calls.some(call => call.isRunning)
    const failed = e.props.calls.filter(call => call.isErrored).length
    const [mark, markStyle] = isRunning
      ? [glyph.working, style.working]
      : failed > 0
        ? [glyph.crashed, style.crashed]
        : [glyph.done, style.faint]

    return (
      <Text wrap="truncate-end">
        <Text {...markStyle}>{mark} </Text>
        {[...counts].map(([tool, count], index) => (
          <Text>
            {index === 0 ? '' : ' · '}
            <Text {...style.label}>{tool}</Text> ×{count}
          </Text>
        ))}
        {failed > 0 && <Text {...style.crashed}> · {failed} failed</Text>}
      </Text>
    )
  })

  // ----- prompt -----

  on('ui.render', { component: 'UserMessage' }, async ($, e, next) => {
    // Só o prompt digitado. Notificação, mensagem de outro agente e o registro
    // de um comando (que chega como marcação) ficam com o desenho original.
    // O prompt mandado do mosaico chega pelo tesseract-link como fala da pessoa.
    const { origin } = e.props
    const isOwn =
      origin.kind === 'composer' || (origin.kind === 'plugin' && origin.name === 'tesseract-link' && origin.asUser === true)
    const isTyped = isOwn && !e.props.text.startsWith('<')
    if (!isTyped || !(await isOn($, options, 'user_message'))) return next(e)

    const { Box, Text } = $.ui.resolve(e)
    const style = styles((await loadEnvironment($, options)).noColor)

    return (
      <Box {...style.userRow} flexDirection="row" marginTop={1}>
        <Text {...style.label}>{glyph.prompt} </Text>
        <Text {...style.strong}>{e.props.text}</Text>
      </Box>
    )
  })

  on('ui.render', { component: 'PromptHint' }, async ($, e, next) => {
    if (!(await isOn($, options, 'prompt_hint'))) return next(e)

    const hint = hintCopy(e.props.hint)
    if (hint === e.props.hint) return next(e)

    // Reescrever `hint` troca a linha inteira e o Claude Code descarta `tail`:
    // o que outro mod acrescentou ali segue junto, dentro do texto.
    const { tail, ...props } = e.props

    return next({ ...e, props: { ...props, hint: tail === undefined ? hint : `${hint} · ${tail}` } })
  })

  on('ui.render', { component: 'SessionMode' }, async ($, e, next) => {
    if (e.props.modes.length === 0 || !(await isOn($, options, 'session_mode'))) return next(e)

    const { Text } = $.ui.resolve(e)
    const style = styles((await loadEnvironment($, options)).noColor)

    return (
      <Text {...style.label} wrap="truncate-end">
        {e.props.modes.map(mode => mode.toUpperCase()).join(' · ')}
      </Text>
    )
  })
}
