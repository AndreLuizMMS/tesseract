// tesseract-roster: quem está chamando, a busca na conversa e as dicas.
//
// - Cada sessão grava seu estado (trabalhando, respondeu, esperando
//   aprovação) no `$.store` do plugin, que é um arquivo só para todas as
//   sessões: uma chave por sessão, regravada a cada 30 s, vencida em 90 s.
//   "Respondeu" volta a ocioso quando a pessoa retoma a sessão.
// - A faixa acima do prompt mostra quantas OUTRAS sessões pedem ação; sem
//   nenhuma, não existe. O `⧉` é da faixa da célula e não se repete aqui.
// - `/find <texto>` busca nas mensagens desta sessão e lista em painel.
// - A linha de dica do prompt ganha os comandos Tesseract carregados, e
//   `/ts-help` lista cada um.

import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { RosterCalling, RosterCounts, RosterFind } from '../types'
import { clampTop, moveBy } from './list'
import type { RosterEvent } from './roster'
import { HEARTBEAT_MS, bandParts, countOthers, expiredKeys, findMatches, helpLines, hitsLabel, keyOf, nextState } from './roster'
import { GLYPH, ink, setMono } from './tokens'

const PANE = 'tesseract-find'
const TICK_MS = 3_000
// Comandos dos mods Tesseract que a dica anuncia, quando estão carregados.
const KNOWN_COMMANDS = ['docker', 'md', 'open', 'find']
const BODY_CHARS = 4_000

const NO_FIND: RosterFind = { term: '', matches: [], total: 0, expanded: 0, body: '', selected: 0, top: 0, ring: '', bodyTop: 0 }
// Paradas de foco que a tela da mensagem aberta mantém: o host guarda o anel
// pela posição, e assim ele continua no "back" ao abrir e na linha ao voltar.
const MAX_ROWS = 12

const counts = atom({ plugin: 'tesseract-roster', key: 'counts' } as const, { responded: 0, approve: 0 })
const hints = atom({ plugin: 'tesseract-roster', key: 'hints' } as const, '')
const find = atom({ plugin: 'tesseract-roster', key: 'find' } as const, NO_FIND)

// O estado desta sessão como foi gravado por último.
let current: RosterCalling = 'idle'
let lastBeat = 0
let isTicking = false
// Quantos resultados o último desenho mostrou, e quantas linhas de texto a
// mensagem aberta teve: a janela que o foco e as teclas de página percorrem.
let shownRows = 1
let bodyRoom = 1

// Grava o estado desta sessão sob a chave dela. O id é lido a cada vez: um
// /clear troca o id sem reiniciar o mod.
async function mark($: EngineInterface, state: RosterCalling): Promise<void> {
  current = state
  lastBeat = await $.clock.now()
  await $.store.set(keyOf(await $.session.id()), { state, at: lastBeat, cwd: await $.session.cwd() })
}

// Aplica um evento ao estado; só grava quando ele muda (digitar é evento a
// cada tecla, e quase nunca muda nada).
async function advance($: EngineInterface, event: RosterEvent): Promise<void> {
  const state = nextState(current, event)
  if (state !== current) await mark($, state).catch(() => undefined)
}

// Lê as entradas de todas as sessões, apaga as vencidas e conta as outras.
async function tick($: EngineInterface): Promise<void> {
  if (isTicking) return

  isTicking = true
  try {
    const now = await $.clock.now()
    if (now - lastBeat >= HEARTBEAT_MS) await mark($, current)

    const keys = await $.store.keys()
    const values = await Promise.all(keys.map(key => $.store.get(key)))
    const entries = Object.fromEntries(keys.map((key, index) => [key, values[index]]))
    for (const key of expiredKeys(entries, now)) await $.store.delete(key)

    const others = countOthers(entries, keyOf(await $.session.id()), now)
    const held = await read($, counts)
    if (held.responded !== others.responded || held.approve !== others.approve) {
      await update($, counts, () => others)
    }

    if ((await read($, hints)) === '') {
      const loaded = new Set((await $.command.list()).map(command => command.name))
      const present = KNOWN_COMMANDS.filter(name => loaded.has(name)).map(name => `/${name}`)
      if (present.length > 0) await update($, hints, () => present.join(' '))
    }
  } catch {
    // ponytail: um tick perdido é contador velho por 3 s; o próximo relê.
  } finally {
    isTicking = false
  }
}

async function search($: EngineInterface, term: string): Promise<void> {
  const messages = await $.session.messages()
  const found = findMatches(messages, term)
  await update($, find, held => ({ ...NO_FIND, ring: held.ring, term: term.trim(), matches: found.matches, total: found.total }))
}

// Abre a mensagem inteira no lugar da lista, ou volta à lista.
async function expand($: EngineInterface, message: number): Promise<void> {
  const held = await read($, find)
  if (held.expanded !== 0) {
    await update($, find, now => ({ ...now, expanded: 0, body: '', bodyTop: 0 }))

    return
  }
  const text = (await $.session.messages())[message - 1]?.text ?? ''
  await update($, find, now => ({
    ...now,
    expanded: message,
    bodyTop: 0,
    body: text.length > BODY_CHARS ? `${text.slice(0, BODY_CHARS)}…` : text,
  }))
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    try {
      setMono(await $.env.get('NO_COLOR'))
      await mark($, 'idle')
      $.clock.every(TICK_MS, () => {
        void tick($)
      })
    } catch (error) {
      $.ui.log(`roster setup failed: ${String(error).slice(0, 120)}`, { to: 'debug' })
    }
    await $.command.register({
      name: 'find',
      description: 'Search the messages of this session; results in a panel',
      argumentHint: '<text>',
    })
    await $.command.register({ name: 'ts-help', description: 'List the Tesseract commands loaded in this session' })

    return started
  })

  on('session.end', async ($, e, next) => {
    await $.store.delete(keyOf(e.sessionId)).catch(() => undefined)

    return next(e)
  })

  // O estado do turno, direto dos eventos: sem raspar tela.
  on('turn.start', async ($, e, next) => {
    await advance($, 'turn.start')

    return next(e)
  })

  on('turn.complete', async ($, e, next) => {
    // Turno de subagente não é resposta ao usuário.
    if (e.agentId === undefined) await advance($, 'turn.complete')

    return next(e)
  })

  on('classic.PermissionRequest', async ($, e, next) => {
    await advance($, 'ask')

    return next(e)
  })

  // A pessoa voltou a esta sessão: o que ela respondeu já foi visto.
  // Este evento roda a cada tecla e tem 50 ms: a tecla passa primeiro e a
  // gravação vai solta, depois. Gravar antes estourava o prazo, e o `next`
  // que chegava atrasado era recusado.
  on('prompt.edit', ($, e, next) => {
    const edited = next(e)
    void advance($, 'attend')

    return edited
  })

  on('prompt.submit', async ($, e, next) => {
    await advance($, 'attend')

    return next(e)
  }).catch(($, e, next) => next(e))

  // Pergunta ao usuário também é "esperando você"; e quando a tool que
  // esperava aprovação termina, a sessão voltou a trabalhar.
  // ponytail: com tools em paralelo, uma terminar tira o "approve" da que
  // ainda espera, até o próximo pedido. Contar pendências se incomodar.
  on('tool.call', async ($, e, next) => {
    if (String(e.tool) === 'AskUserQuestion') await advance($, 'ask')
    const ran = await next(e)
    await advance($, 'tool.done')

    return ran
  }).catch(($, e, next) => next(e))

  on('command.run', { command: 'find' }, async ($, e) => {
    await search($, e.args)
    const found = await read($, find)
    const opened = await $.ui.open({
      id: PANE,
      title: 'FIND',
      focus: true,
      closeOnEscape: true,
      rows: Math.min(found.matches.length, MAX_ROWS) + 3,
    })
    // Com resultado, o foco vai para o primeiro; sem, fica no campo.
    await $.ui.focus({ requestId: PANE, key: found.matches.length > 0 ? 'row:0' : 'term' }).catch(() => undefined)

    return opened.isPlaced ? {} : { text: `Find panel is waiting for room: ${opened.reason}` }
  })

  on('command.run', { command: 'ts-help' }, async ($, e) => {
    const loaded = (await $.command.list()).filter(command => KNOWN_COMMANDS.includes(command.name))
    if (loaded.length === 0) return { text: 'No Tesseract command is loaded in this session.' }

    return {
      text: [
        'Tesseract commands in this session:',
        ...helpLines(loaded, e.presentation.columns),
        ...helpLines(
          [{ name: 'keys', description: 'In a panel: arrows move, Enter picks, the letter or digit before a colon presses it, Esc closes.' }],
          e.presentation.columns,
        ).map(line => line.replace('/keys', 'keys ')),
      ].join('\n'),
    }
  })

  // A dica do prompt ganha os comandos carregados, depois do que já havia:
  // só acrescenta ao `tail` e passa adiante, nunca troca a linha.
  on('ui.render', { component: 'PromptHint' }, async ($, e, next) => {
    const known = await read($, hints)
    if (known === '' || e.props.isDraft || e.props.isWorking) return next(e)

    return next({ ...e, props: { ...e.props, tail: e.props.tail === undefined ? known : `${e.props.tail} ${known}` } })
  })

  // A faixa: quantas outras sessões pedem ação. Sem nenhuma, não existe.
  // Aprovação é glifo e âmbar (sem cor: glifo e negrito), nunca invertida:
  // o badge invertido da tela é do mosaico.
  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    const others: RosterCounts = await read($, counts)
    const parts = bandParts(others, e.props.bodyColumns)
    if (e.props.hasSurvey || parts.length === 0) return next(e)

    const { Box, Text } = $.ui.resolve(e)
    const below = await next(e)

    return (
      <Box flexDirection="column">
        {below}
        <Text wrap="truncate-end">
          <Text>{'  '}</Text>
          {parts.map((part, index) => (
            <Text>
              {index > 0 && <Text {...ink('faint')}> · </Text>}
              {part.kind === 'approve' && <Text {...ink('block')} bold>{part.text}</Text>}
              {part.kind === 'responded' && <Text {...ink('read')}>{part.text}</Text>}
              {part.kind === 'label' && <Text {...ink('faint')}>{part.text}</Text>}
            </Text>
          ))}
        </Text>
      </Box>
    )
  })

  // O foco anda pelo campo e pelos resultados visíveis. Tentar passar do
  // último visível (a parada do contador) desce a janela; subir do primeiro
  // com resultado acima sobe a janela em vez de ir ao campo.
  on('ui.focus', { requestId: PANE }, async ($, e, next) => {
    const key = e.element
    const now = await read($, find)
    const isPerson = e.origin.kind === 'person'
    const land = async (ring: string, selected = now.selected): Promise<void> => {
      if (now.ring !== ring || now.selected !== selected) await update($, find, held => ({ ...held, ring, selected }))
    }
    if (key === undefined) {
      // Do campo, subir cairia na marca de fechar do host, e a próxima letra
      // iria para o prompt: o anel fica no campo.
      if (isPerson && now.expanded === 0 && now.ring === 'term') return {}
      await land('')

      return next(e)
    }
    if (now.expanded !== 0) {
      // Mensagem aberta: só o "back" segura o anel.
      if (isPerson && key !== `row:${now.selected - now.top}`) return {}
      await land(key)

      return next(e)
    }

    const wasOnRow = now.ring.startsWith('row:')
    if (key.startsWith('row:')) {
      const selected = now.top + Number(key.slice(4))
      if (selected >= now.matches.length) return {}
      await land(key, selected)

      return next(e)
    }
    if (key === 'term' && !(isPerson && wasOnRow && now.top > 0)) {
      await land('term')

      return next(e)
    }
    if (isPerson && wasOnRow) {
      const moved = moveBy(now, key === 'term' ? -1 : 1, shownRows, now.matches.length)
      await update($, find, held => ({ ...held, ...moved }))
    }

    return {}
  }).catch(($, e, next) => next(e))

  // PgUp e PgDn: uma janela de resultados, ou de linhas da mensagem aberta.
  on('ui.scroll', { requestId: PANE }, async ($, e) => {
    const now = await read($, find)
    const way = Math.sign(e.by)
    if (now.expanded !== 0) {
      await update($, find, held => ({ ...held, bodyTop: Math.max(held.bodyTop + way * bodyRoom, 0) }))
    } else if (now.matches.length > 0) {
      const last = Math.max(now.matches.length - shownRows, 0)
      const top = Math.max(Math.min(now.top + way * shownRows, last), 0)
      const selected = Math.min(top + (now.selected - now.top), now.matches.length - 1)
      await update($, find, held => ({ ...held, top, selected }))
    }

    return {}
  }).catch(() => ({}))

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    // Sem campo de texto (celular) não há como buscar daqui.
    if (e.surface === 'mobile') {
      const { Text } = $.ui.resolve(e)

      return <Text>/find needs a surface with a text field.</Text>
    }

    const { Box, Text, Button, Input } = $.ui.resolve(e)
    const now = await read($, find)
    const hasKeys = e.props.isFocused
    const columns = e.props.bodyColumns
    const bodyRows = Math.max(e.props.scroll.bodyRows, 1)
    const total = now.matches.length
    const noKeys = (
      <Text {...ink('faint')} wrap="truncate-end">
        {columns < 64 ? 'ctrl+x tab gives this panel the keys.' : 'The prompt has the keyboard. ctrl+x tab hands it to this panel.'}
      </Text>
    )
    const header = (right: string) => (
      <Box flexDirection="row">
        <Box flexShrink={0}>
          <Text {...ink('flux')} bold>{`${GLYPH.mark} `}</Text>
          <Text {...ink('bright')} bold>FIND</Text>
        </Box>
        <Box flexGrow={1} flexShrink={1}>
          <Text {...ink('muted')} wrap="truncate-end">{now.term === '' ? ' // this session' : ` // "${now.term}"`}</Text>
        </Box>
        <Box flexShrink={0}>
          <Text {...ink('muted')}>{right === '' ? '' : ` ${right}`}</Text>
        </Box>
      </Box>
    )
    // Tecla sem ação devolve o teclado ao prompt e é escrita lá: o host não
    // deixa um painel consumir teclas, só dar tecla a um botão. Então toda
    // letra e dígito que a tela não usa ganha um botão invisível que não faz
    // nada. Pontuação e espaço não podem ser tecla de botão, e vazam.
    const sink = (used: string) => (
      <Box display="none">
        {[...'abcdefghijklmnopqrstuvwxyz0123456789'].filter(key => !used.includes(key)).map(key => (
          <Button key={`sink:${key}`} hotkey={key} plain label="" onPress={() => undefined} />
        ))}
      </Box>
    )
    // Paradas vazias: seguram a posição do anel entre a lista e a mensagem.
    const pads = (from: number, to: number) =>
      Array.from({ length: Math.max(to - from, 0) }, (_, offset) => (
        <Button key={from + offset === 0 ? 'term' : `row:${from + offset - 1}`} plain label="" onPress={() => expand($, now.expanded)} />
      ))

    if (now.expanded !== 0) {
      const match = now.matches[now.selected]
      const hasHeader = bodyRows >= 5
      const room = Math.max(bodyRows - 1 - (hasHeader ? 1 : 0), 1)
      // O "back" fica na parada da linha de onde se veio (o campo é a 0).
      const home = Math.min(Math.max(now.selected - now.top, 0), MAX_ROWS - 1) + 1
      const text = now.body === '' ? 'This message is gone from the transcript.' : now.body
      bodyRoom = room

      return (
        <Box flexDirection="column">
          {hasHeader && header(`#${now.expanded} ${match?.role === 'user' ? 'you' : 'claude'}`)}
          <Box height={room} overflow="hidden" flexDirection="column">
            <Box marginTop={-now.bodyTop} flexShrink={0}>
              <Text {...ink('text')}>{text}</Text>
            </Box>
          </Box>
          <Box flexDirection="row">
            {pads(0, home)}
            {hasKeys
              ? <Button key={`row:${home - 1}`} hotkey="b" plain autoFocus label="back" onPress={() => expand($, now.expanded)} />
              : <Button key={`row:${home - 1}`} plain label="" onPress={() => expand($, now.expanded)} />}
            {pads(home + 1, MAX_ROWS + 1)}
            <Text {...ink('faint')} wrap="truncate-end">{hasKeys ? '  PgUp PgDn scroll  Esc close' : ''}</Text>
            {!hasKeys && noKeys}
          </Box>
          {hasKeys && sink('b')}
        </Box>
      )
    }

    // O campo fica sempre. Do resto, a linha de teclas é a última a cair:
    // antes dela caem o cabeçalho e as linhas dos resultados.
    const hasHeader = bodyRows >= 6
    const hasKeyLine = bodyRows >= 2
    const room = Math.max(bodyRows - 1 - (hasKeyLine ? 1 : 0) - (hasHeader ? 1 : 0), 0)
    const rows = Math.min(total, room, MAX_ROWS)
    const top = clampTop(now.top, now.selected, rows, total)
    const isOnList = hasKeys && now.ring.startsWith('row:')
    shownRows = Math.max(rows, 1)

    return (
      <Box flexDirection="column">
        {hasHeader && header(hitsLabel(total, now.total))}
        <Input
          key="term"
          label="find"
          placeholder={columns < 48 ? 'text' : 'text in this session'}
          value={now.term}
          submitLabel="search"
          {...(total === 0 || rows === 0 ? { autoFocus: true as const } : {})}
          onSubmit={value => search($, value)}
        />
        {now.term === '' && room > 0 && (
          <Text {...ink('muted')} wrap="truncate-end">  Type a term. Enter searches this session.</Text>
        )}
        {now.term !== '' && now.total === 0 && room > 0 && (
          <Text {...ink('muted')} wrap="truncate-end">{`  Nothing matches "${now.term}". Try a shorter term.`}</Text>
        )}
        {now.matches.slice(top, top + rows).map((match, row) => (
          <Box flexDirection="row">
            <Box width={2} flexShrink={0}><Text {...ink('flux')}>{isOnList && top + row === now.selected ? `${GLYPH.cursor} ` : '  '}</Text></Box>
            <Box width={6} flexShrink={0}>
              <Button
                key={`row:${row}`}
                plain
                {...(total > 0 && top + row === now.selected ? { autoFocus: true as const } : {})}
                label={`#${match.message}`}
                onPress={() => expand($, match.message)}
              />
            </Box>
            <Box width={7} flexShrink={0}>
              <Text {...ink('fluxCore')}>{match.role === 'user' ? 'you' : 'claude'}</Text>
            </Box>
            <Box width={Math.max(columns - 15, 4)} flexShrink={0}>
              <Text {...ink('text')} wrap="truncate-end">{match.snippet}</Text>
            </Box>
          </Box>
        ))}
        {hasKeyLine && (
          <Box flexDirection="row">
            <Box flexGrow={1} flexShrink={1}>
              {hasKeys
                ? <Text {...ink('faint')} wrap="truncate-end">{rows === 0 ? 'Enter search  Esc close' : '↑↓ hit  Enter whole message  Esc close'}</Text>
                : noKeys}
            </Box>
            {total > 0 && (
              <Box flexShrink={0}>
                <Text>{' '}</Text>
                <Button key="more" plain dimColor label={`${now.selected + 1}/${total}`} onPress={() => undefined} />
              </Box>
            )}
          </Box>
        )}
        {/* No campo de busca a letra é texto; só com o anel numa linha. */}
        {isOnList && sink('')}
      </Box>
    )
  })
}
