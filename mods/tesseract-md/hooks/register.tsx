// tesseract-md: o leitor de markdown do projeto dentro da sessão.
//
// - `/md` abre a lista dos `.md` do projeto com filtro por texto.
// - Enter num arquivo abre a leitura; ela relê sozinha quando o arquivo muda.
// - Links para outros docs do projeto viram teclas de 1 a 9; link quebrado
//   ou para fora do projeto aparece marcado, sem tecla.
// - O desenho cabe no corpo que a célula dá: a lista rola com o foco, e na
//   leitura o texto rola por baixo do cabeçalho e da linha de teclas.
// - Quando o Claude escreve um `.md`, um aviso oferece `/md last`. O painel
//   nunca abre sozinho.

import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { MdLink, MdList, MdReader } from '../types'
import { MAX_DEPTH, MAX_FILES, filterDocs, fullRows, isMarkdown, isSkippedDir, markdownLinks, maxOffset, sortDocs } from './docs'
import { clampTop, cut, moveBy, wrapRows } from './list'
import { GLYPH, ink, setMono } from './tokens'

const PANE = 'tesseract-md'
// O polling da leitura aberta: barato (um stat) e só com o painel na tela.
const POLL_MS = 1_500
// Um desenho comporta 100 mil caracteres; sobra margem para a moldura.
const MAX_CHARS = 90_000
// ponytail: teto de pastas listadas por varredura; monorepo maior que isso
// perde os docs mais fundos. Subir, ou trocar por `git ls-files`, se faltar.
const MAX_DIRS = 1_500
// Linhas de corpo pedidas: a célula dá o que tem, o desenho se ajusta.
const LIST_ROWS = 16
const READER_ROWS = 24
// Linhas da lista na tela, no máximo. É também o que fixa as paradas de
// foco da leitura: o host guarda o anel pela posição do elemento, então o
// "list/back" ocupa a parada onde o anel já estava ao abrir o doc.
const MAX_ROWS = 11
const STOPS = MAX_ROWS + 2
const KEYED_LINKS = 9

const NO_READER: MdReader = { path: '', text: '', isCut: false, mtimeMs: 0, error: '', trail: [], links: [], offset: 0 }
const NO_LIST: MdList = { filter: '', selected: '', top: 0, ring: -1, isScanning: false }

const files = atom({ plugin: 'tesseract-md', key: 'files' } as const, [])
const list = atom({ plugin: 'tesseract-md', key: 'list' } as const, NO_LIST)
const reader = atom({ plugin: 'tesseract-md', key: 'reader' } as const, NO_READER)
const lastWritten = atom({ plugin: 'tesseract-md', key: 'lastWritten' } as const, '')

let isPolling = false
// O que o último desenho mostrou: a janela que o foco e as teclas de página
// percorrem, e até onde a leitura pode rolar.
let shownRows = 1
let shownTop = 0
let readerRoom = 1
let readerLimit = 0

// Abre o painel, ou o redimensiona se já está aberto, com o teclado nele.
const show = ($: EngineInterface, rows: number) =>
  $.ui.open({ id: PANE, title: 'MARKDOWN', focus: true, closeOnEscape: true, rows })

// Varre o projeto em largura, um nível por vez, e devolve os markdown.
async function scan($: EngineInterface): Promise<string[]> {
  const root = await $.session.root()
  const found: string[] = []
  let level = ['']
  let listed = 0
  for (let depth = 0; depth <= MAX_DEPTH && level.length > 0 && found.length < MAX_FILES; depth++) {
    const dirs = level.slice(0, MAX_DIRS - listed)
    listed += dirs.length
    const listings = await Promise.all(
      dirs.map(dir => $.fs.list(dir === '' ? root : `${root}/${dir}`).catch(() => [])),
    )
    const deeper: string[] = []
    listings.forEach((entries, index) => {
      const prefix = dirs[index] === '' ? '' : `${dirs[index]}/`
      for (const entry of entries) {
        if (entry.kind === 'file' && isMarkdown(entry.name)) found.push(`${prefix}${entry.name}`)
        if (entry.kind === 'dir' && !isSkippedDir(entry.name)) deeper.push(`${prefix}${entry.name}`)
      }
    })
    level = deeper
  }

  return sortDocs(found).slice(0, MAX_FILES)
}

async function rescan($: EngineInterface): Promise<void> {
  await update($, list, held => ({ ...held, isScanning: true }))
  const found = await scan($).catch(() => [])
  await update($, files, () => found)
  await update($, list, held => ({ ...held, isScanning: false }))
}

// Os links do texto, cada um com seu estado: abre, quebrado ou fora.
async function linksOf($: EngineInterface, text: string, path: string): Promise<MdLink[]> {
  const root = await $.session.root()

  return Promise.all(
    markdownLinks(text, path).map(async link => {
      if (link.state === 'outside') return { path: link.path, href: link.href, state: 'outside' as const }

      const isThere = await $.fs.exists(`${root}/${link.path}`).catch(() => false)

      return { path: link.path, href: link.href, state: isThere ? ('ok' as const) : ('broken' as const) }
    }),
  )
}

// Lê o arquivo para a leitura. `trail` é de onde se veio, para o voltar;
// `offset` é onde a leitura estava, quando é só o arquivo que mudou.
async function load($: EngineInterface, path: string, trail: string[], offset = 0): Promise<void> {
  const full = `${await $.session.root()}/${path}`
  try {
    const stat = await $.fs.stat(full)
    const whole = await $.fs.read(full)
    const text = whole.slice(0, MAX_CHARS)
    const links = await linksOf($, text, path)
    await update($, reader, () => ({
      path,
      text,
      isCut: whole.length > MAX_CHARS,
      mtimeMs: stat.mtimeMs,
      error: '',
      trail,
      links,
      offset,
    }))
  } catch (error) {
    const isMissing = String(error).includes('ENOENT')
    await update($, reader, () => ({
      ...NO_READER,
      path,
      trail,
      error: isMissing
        ? `${path} is gone. It reopens here if it comes back; b goes back.`
        : `${path} could not be read (over 4 MiB, or no permission). Open it in the IDE.`,
    }))
  }
}

async function openDoc($: EngineInterface, path: string): Promise<void> {
  const now = await read($, reader)
  await load($, path, now.path === '' || now.path === path ? now.trail : [...now.trail, now.path])
  await show($, READER_ROWS)
}

// Volta ao doc anterior, ou à lista quando não há de onde se veio.
async function back($: EngineInterface): Promise<void> {
  const now = await read($, reader)
  const previous = now.trail.at(-1)
  if (previous !== undefined) {
    await load($, previous, now.trail.slice(0, -1))

    return
  }
  await update($, reader, () => NO_READER)
  // A lista volta inteira: o campo do filtro renasce vazio, e um filtro
  // velho por baixo dele esconderia arquivos sem dizer por quê.
  await update($, list, held => ({ ...held, filter: '', top: 0 }))
  await show($, LIST_ROWS)
}

// Relê a leitura aberta quando o arquivo mudou no disco, e a reabre quando
// um arquivo que tinha sumido volta.
async function poll($: EngineInterface): Promise<void> {
  if (isPolling) return

  isPolling = true
  try {
    const now = await read($, reader)
    if (now.path === '') return
    if (!(await $.ui.panes()).some(pane => pane.id === PANE)) return

    const stat = await $.fs.stat(`${await $.session.root()}/${now.path}`).catch(() => undefined)
    if (stat === undefined) {
      // Sumiu: a leitura diz isso na tela, uma vez.
      if (now.error === '') await load($, now.path, now.trail)
    } else if (now.error !== '' || stat.mtimeMs !== now.mtimeMs) {
      await load($, now.path, now.trail, now.error === '' ? now.offset : 0)
    }
  } catch {
    // ponytail: poll perdido é leitura velha por 1,5 s; o próximo relê.
  } finally {
    isPolling = false
  }
}

// Guarda o `.md` que o Claude acabou de escrever e avisa, uma vez por arquivo.
async function offer($: EngineInterface, filePath: unknown): Promise<void> {
  if (typeof filePath !== 'string' || !isMarkdown(filePath)) return

  const root = await $.session.root()
  const path = filePath.startsWith(`${root}/`) ? filePath.slice(root.length + 1) : filePath
  if (path.startsWith('/')) return // fora do projeto: o leitor não alcança

  const previous = await read($, lastWritten)
  await update($, lastWritten, () => path)
  if (previous !== path) $.ui.toast(`/md last opens ${path}.`)
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    try {
      setMono(await $.env.get('NO_COLOR'))
      $.clock.every(POLL_MS, () => {
        void poll($)
      })
    } catch (error) {
      $.ui.log(`md setup failed: ${String(error).slice(0, 120)}`, { to: 'debug' })
    }
    await $.command.register({
      name: 'md',
      description: 'Markdown reader: project docs with filter; /md last opens the one Claude just wrote',
      argumentHint: '[last | filter]',
    })

    return started
  })

  on('command.run', { command: 'md' }, async ($, e) => {
    const wanted = e.args.trim()
    // Abrir um doc direto zera o filtro: o "b" volta para a lista inteira,
    // não para o que foi filtrado da outra vez.
    const direct = async (path: string): Promise<void> => {
      await update($, list, held => ({ ...held, filter: '', top: 0 }))
      await update($, reader, () => NO_READER)
      await openDoc($, path)
    }
    if (wanted === 'last') {
      const path = await read($, lastWritten)
      if (path === '') return { text: 'No markdown written in this session yet. /md lists the project docs.' }

      await direct(path)
      void rescan($).catch(() => undefined)

      return {}
    }

    await rescan($)
    const found = await read($, files)
    if (found.includes(wanted)) {
      await direct(wanted)

      return {}
    }
    await update($, reader, () => NO_READER)
    await update($, list, held => ({ ...held, filter: wanted, top: 0 }))
    const opened = await show($, LIST_ROWS)

    return opened.isPlaced ? {} : { text: `Markdown panel is waiting for room: ${opened.reason}` }
  })

  // Depois que o Claude escreve um markdown: só observa, nunca barra.
  on('tool.call', { tool: 'Write' }, async ($, e, next) => {
    const ran = await next(e)
    if (ran.deny === undefined && ran.isError !== true) await offer($, e.file_path)

    return ran
  }).catch(($, e, next) => next(e))

  on('tool.call', { tool: 'Edit' }, async ($, e, next) => {
    const ran = await next(e)
    if (ran.deny === undefined && ran.isError !== true) await offer($, e.file_path)

    return ran
  }).catch(($, e, next) => next(e))

  // Na lista o foco anda pelo filtro e pelos arquivos visíveis: passar do
  // último visível desce a janela, subir do primeiro com arquivo acima sobe
  // a janela em vez de ir ao filtro. Na leitura o anel fica no "list/back" e
  // as setas rolam o texto: a parada seguinte é "desce", qualquer outra "sobe".
  on('ui.focus', { requestId: PANE }, async ($, e, next) => {
    const key = e.element
    const view = await read($, list)
    const doc = await read($, reader)
    const isPerson = e.origin.kind === 'person'
    const land = async (ring: number, selected = view.selected): Promise<void> => {
      if (view.ring !== ring || view.selected !== selected) {
        await update($, list, held => ({ ...held, ring, selected, top: doc.path === '' ? shownTop : held.top }))
      }
    }
    if (key === undefined) {
      // Do filtro, subir cairia na marca de fechar do host, e a próxima
      // letra iria para o prompt: o anel fica no filtro.
      if (isPerson && doc.path === '' && view.ring === 0) return {}
      await land(-1)

      return next(e)
    }

    if (doc.path !== '') {
      const stop = key.startsWith('stop:') ? Number(key.slice(5)) : -1
      if (!isPerson || view.ring < 0) {
        // Chegando de fora, qualquer parada inofensiva serve; um link não.
        if (stop < 0) return {}
        await land(stop)

        return next(e)
      }
      const by = stop === view.ring + 1 ? 1 : -1
      await update($, reader, held => ({ ...held, offset: Math.max(Math.min(held.offset + by, readerLimit), 0) }))

      return {}
    }

    const matched = filterDocs(await read($, files), view.filter)
    const wasOnRow = view.ring >= 1
    if (key.startsWith('row:')) {
      const file = matched[shownTop + Number(key.slice(4))]
      if (file === undefined) return {}
      await land(1 + Number(key.slice(4)), file)

      return next(e)
    }
    if (key === 'filter' && !(isPerson && wasOnRow && shownTop > 0)) {
      await land(0)

      return next(e)
    }
    if (isPerson && wasOnRow) {
      const place = { selected: shownTop + view.ring - 1, top: shownTop }
      const moved = moveBy(place, key === 'filter' ? -1 : 1, shownRows, matched.length)
      await update($, list, held => ({ ...held, selected: matched[moved.selected] ?? '', top: moved.top }))
    }

    return {}
  }).catch(($, e, next) => next(e))

  // PgUp e PgDn: uma janela de arquivos na lista, uma tela de texto na leitura.
  on('ui.scroll', { requestId: PANE }, async ($, e) => {
    const way = Math.sign(e.by)
    const doc = await read($, reader)
    if (doc.path !== '') {
      await update($, reader, held => ({
        ...held,
        offset: Math.max(Math.min(held.offset + way * Math.max(readerRoom - 1, 1), readerLimit), 0),
      }))

      return {}
    }

    const view = await read($, list)
    const matched = filterDocs(await read($, files), view.filter)
    if (matched.length > 0) {
      const top = Math.max(Math.min(shownTop + way * shownRows, matched.length - shownRows), 0)
      const row = Math.max(Math.min(view.ring - 1, shownRows - 1), 0)
      await update($, list, held => ({ ...held, top, selected: matched[Math.min(top + row, matched.length - 1)] ?? '' }))
    }

    return {}
  }).catch(() => ({}))

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    // Sem campo de texto (celular) a lista com filtro não tem como existir.
    if (e.surface === 'mobile') {
      const { Text } = $.ui.resolve(e)

      return <Text>/md needs a surface with a text field.</Text>
    }

    const { Box, Text, Button, Input, Markdown } = $.ui.resolve(e)
    const all = await read($, files)
    const doc = await read($, reader)
    const view = await read($, list)
    const hasKeys = e.props.isFocused
    const columns = e.props.bodyColumns
    const bodyRows = Math.max(e.props.scroll.bodyRows, 1)
    const project = (await $.session.root()).split('/').pop() ?? ''

    // Marca e título nunca encolhem; o miolo corta com reticências.
    const header = (trail: string, right: string) => (
      <Box flexDirection="row">
        <Box flexShrink={0}>
          <Text {...ink('flux')} bold>{`${GLYPH.mark} `}</Text>
          <Text {...ink('bright')} bold>MARKDOWN</Text>
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
    // Tecla sem ação devolve o teclado ao prompt e é escrita lá: o host não
    // deixa um painel consumir teclas, só dar tecla a um botão. Então toda
    // letra e dígito que a tela não usa ganha um botão invisível. Pontuação
    // e espaço não podem ser tecla de botão, e vazam.
    const sink = (used: string, onKey: (key: string) => void) => (
      <Box display="none">
        {[...'abcdefghijklmnopqrstuvwxyz0123456789'].filter(key => !used.includes(key)).map(key => (
          <Button key={`sink:${key}`} hotkey={key} plain label="" onPress={() => onKey(key)} />
        ))}
      </Box>
    )

    if (doc.path !== '') {
      const keyed = doc.links.filter(link => link.state === 'ok').slice(0, KEYED_LINKS)
      const marked = doc.links.filter(link => link.state !== 'ok')
      const nameOf = (path: string): string => cut(path.split('/').pop() ?? path, 18)
      const backLabel = doc.trail.length > 0 ? 'back' : 'list'
      const notes = marked.map(link => `${nameOf(link.path)} (${link.state === 'broken' ? 'broken' : 'outside project'})`)
      // A linha de teclas dobra quando há linhas para isso; a leitura fica
      // com pelo menos metade do corpo.
      const widths = [backLabel.length + 3, ...keyed.map(link => nameOf(link.path).length + 3), ...notes.map(note => note.length)]
      const hasHeader = bodyRows >= 6
      const keyRows = Math.max(Math.min(wrapRows(widths, 2, columns), Math.floor((bodyRows - (hasHeader ? 1 : 0)) / 2)), 1)
      // Doc curto não estica o painel: a leitura pega só as linhas que usa.
      const most = Math.max(bodyRows - keyRows - (hasHeader ? 1 : 0), 1)
      const room = doc.error !== '' || doc.text.trim() === '' ? Math.min(most, 2) : Math.min(most, fullRows(doc.text, columns))
      const limit = maxOffset(doc.text, columns, room)
      const offset = Math.min(doc.offset, limit)
      // O "list/back" fica na parada onde o anel já está.
      const home = Math.min(Math.max(view.ring, 0), STOPS - 2)
      const pads = (from: number, to: number) =>
        Array.from({ length: Math.max(to - from, 0) }, (_, at) => (
          <Button key={`stop:${from + at}`} plain label="" onPress={() => back($)} />
        ))
      readerRoom = room
      readerLimit = limit

      return (
        <Box flexDirection="column">
          {hasHeader && header(doc.path, doc.isCut ? 'cut at 90k chars' : offset > 0 ? `↓${offset}` : '')}
          <Box height={room} overflow="hidden" flexDirection="column">
            {doc.error !== '' && <Text {...ink('dead')}>{`${GLYPH.dead} ${doc.error}`}</Text>}
            {doc.error === '' && doc.text.trim() === '' && (
              <Text {...ink('muted')}>{`${doc.path} is empty. It reloads here when something is written.`}</Text>
            )}
            {doc.error === '' && doc.text.trim() !== '' && (
              <Box marginTop={-offset} flexShrink={0} flexDirection="column">
                <Markdown
                  key="doc"
                  text={doc.text}
                  pressableLinks={keyed.map(link => link.href)}
                  onLinkPress={pressed => {
                    const target = keyed.find(link => link.href === pressed.href)
                    if (target !== undefined) void openDoc($, target.path)
                  }}
                />
              </Box>
            )}
          </Box>
          <Box height={keyRows} overflow="hidden" flexDirection="row" flexWrap="wrap" columnGap={2}>
            <Box flexDirection="row">
              {pads(0, home)}
              {hasKeys
                ? <Button key={`stop:${home}`} hotkey="b" plain autoFocus label={backLabel} onPress={() => back($)} />
                : <Button key={`stop:${home}`} plain label="" onPress={() => back($)} />}
              {pads(home + 1, STOPS)}
            </Box>
            {hasKeys && keyed.map((link, index) => (
              <Button
                key={`link:${link.path}`}
                hotkey={String(index + 1)}
                plain
                label={nameOf(link.path)}
                onPress={() => openDoc($, link.path)}
              />
            ))}
            {hasKeys && notes.map(note => <Text {...ink('faint')}>{note}</Text>)}
            {hasKeys && keyed.length + notes.length === 0 && (
              <Text {...ink('faint')} wrap="truncate-end">↑↓ PgUp PgDn scroll  Esc close</Text>
            )}
            {!hasKeys && noKeys}
          </Box>
          {hasKeys && sink(`b${keyed.map((_, index) => index + 1).join('')}`, () => undefined)}
        </Box>
      )
    }

    const matched = filterDocs(all, view.filter)
    const total = matched.length
    // O filtro fica sempre. Do resto, a linha de teclas é a última a cair:
    // antes dela caem o cabeçalho e as linhas da lista (num corpo de duas
    // linhas sobram o filtro e as teclas, e Enter ainda abre o primeiro).
    const hasHeader = bodyRows >= 6
    const hasKeyLine = bodyRows >= 2
    const room = Math.max(bodyRows - 1 - (hasKeyLine ? 1 : 0) - (hasHeader ? 1 : 0), 0)
    const rows = Math.min(total, room, MAX_ROWS)
    // O escolhido é quem está sob o anel; quando a lista muda por baixo (o
    // filtro andou), a janela tenta manter o mesmo arquivo ali.
    const ringRow = view.ring - 1
    const isRingOnRow = hasKeys && ringRow >= 0 && ringRow < rows
    const held = matched.indexOf(view.selected)
    const wanted = held - ringRow
    const canKeep = isRingOnRow && held >= 0 && wanted >= 0 && wanted <= Math.max(total - rows, 0)
    const top = canKeep ? wanted : clampTop(view.top, Math.min(view.top + Math.max(ringRow, 0), Math.max(total - 1, 0)), rows, total)
    const index = isRingOnRow ? top + ringRow : -1
    shownRows = Math.max(rows, 1)
    shownTop = top

    return (
      <Box flexDirection="column">
        {hasHeader && header(project, all.length === 0 ? '' : `${total} of ${all.length}`)}
        <Input
          key="filter"
          label="filter"
          placeholder={columns < 48 ? 'name' : 'name or path'}
          value={view.filter}
          submitLabel={columns < 48 ? 'open' : 'open first'}
          autoFocus
          onInput={(value: string) => update($, list, now => ({ ...now, filter: value, top: 0 }))}
          onSubmit={(value: string) => {
            const first = filterDocs(all, value)[0]
            if (first !== undefined) void openDoc($, first)
          }}
        />
        {room > 0 && view.isScanning && all.length === 0 && <Text {...ink('faint')}>  scanning the project…</Text>}
        {room > 0 && !view.isScanning && all.length === 0 && (
          <Text {...ink('muted')} wrap="truncate-end">  No markdown here. /md last opens what Claude writes.</Text>
        )}
        {room > 0 && all.length > 0 && total === 0 && (
          <Text {...ink('muted')} wrap="truncate-end">{`  Nothing matches "${view.filter}". Backspace widens it.`}</Text>
        )}
        {matched.slice(top, top + rows).map((file, row) => {
          const slash = file.lastIndexOf('/')
          const more = row === 0 && top > 0 ? '▴' : row === rows - 1 && top + rows < total ? '▾' : ''

          return (
            <Box flexDirection="row">
              <Box width={2} flexShrink={0}><Text {...ink('flux')}>{top + row === index ? `${GLYPH.cursor} ` : '  '}</Text></Box>
              <Box flexShrink={0}>
                <Button key={`row:${row}`} plain label={cut(file.slice(slash + 1), Math.max(columns - 6, 8))} onPress={() => openDoc($, file)} />
              </Box>
              <Box flexGrow={1} flexShrink={1}>
                <Text {...ink('faint')} wrap="truncate-end">{slash < 0 ? '' : `  ${file.slice(0, slash + 1)}`}</Text>
              </Box>
              {total > rows && <Text {...ink('faint')}>{` ${more}`}</Text>}
            </Box>
          )
        })}
        {hasKeyLine && (
          <Box flexDirection="row">
            <Box flexGrow={1} flexShrink={1}>
              {hasKeys
                ? <Text {...ink('faint')} wrap="truncate-end">{rows > 0 ? 'type to filter  ↑↓ file  Enter read  Esc close' : 'type to filter  Enter opens first  Esc close'}</Text>
                : noKeys}
            </Box>
            {total > 0 && (
              <Box flexShrink={0}>
                <Text>{' '}</Text>
                <Button key="more" plain dimColor label={`${index < 0 ? top + 1 : index + 1}/${total}`} onPress={() => undefined} />
              </Box>
            )}
          </Box>
        )}
        {/* Com o anel numa linha, digitar é filtrar: a letra vai para o
            filtro e o anel volta ao campo, em vez de vazar para o prompt. */}
        {isRingOnRow && sink('', key => {
          void update($, list, now => ({ ...now, filter: now.filter + key, top: 0 }))
          void $.ui.focus({ requestId: PANE, key: 'filter' })
        })}
      </Box>
    )
  })
}
