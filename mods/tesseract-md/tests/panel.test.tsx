import type { On } from 'claude-code'
import { expect, test } from 'claude-code/testing'

const PANE = 'tesseract-md'
const STOPS = 13

const FILES: Record<string, string> = {
  'README.md': '# Proj\n\nSee [arq](docs/arq.md), [gone](docs/nope.md) and [out](../fora.md).\n',
  'docs/arq.md': '# Arq\n\nBack to [readme](../README.md).\n',
}

const MD = (args: string) => ({
  command: 'md',
  args,
  origin: { kind: 'composer' as const },
  presentation: { isFullscreen: false, columns: 80 },
})

const props = (bodyRows: number) => ({
  title: 'MARKDOWN',
  isFocused: true,
  bodyColumns: 76,
  placement: 'inline' as const,
  scroll: { offset: 0, bodyRows },
  view: {},
})

// Um projeto de mentira: dois markdown, um deles numa pasta.
function fakeProject(on: On): void {
  const entry = (name: string, kind: 'file' | 'dir') => ({ name, kind, size: 1, mtimeMs: 1, isLink: false })
  on('session.root', () => ({ value: '/proj' }))
  on('command.run', () => ({}))
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('ui.panes', () => ({ value: [] }))
  on('fs.list', (_, e) => ({
    value: e.path === '/proj' ? [entry('README.md', 'file'), entry('docs', 'dir')] : e.path === '/proj/docs' ? [entry('arq.md', 'file')] : [],
  }))
  on('fs.exists', (_, e) => ({ value: e.path.slice(6) in FILES }))
  on('fs.stat', (_, e) => (e.path.slice(6) in FILES ? { value: entry('x', 'file') } : { deny: 'ENOENT' }))
  on('fs.read', (_, e) => ({ value: FILES[e.path.slice(6)] ?? '' }))
}

const buttons = async (ui: { findAll: (query: { type: string }) => Promise<{ key: string | undefined; text: string }[]> }) =>
  (await ui.findAll({ type: 'Button' }))
    .map(button => ({ key: button.key ?? '', text: button.text }))
    // Os botões `sink:` são invisíveis: só seguram as teclas sem ação.
    .filter(button => !button.key.startsWith('sink:'))

test('leitura: as primeiras paradas são todas "voltar"; link só ganha tecla se abre', async ($, on) => {
  fakeProject(on)
  await $.command.run(MD('README.md'))
  const ui = await $.ui.mount({ plugin: PANE, surface: 'terminal', component: 'Pane', requestId: PANE, props: props(3) })

  const drawn = await buttons(ui)
  expect(drawn.slice(0, STOPS).map(button => button.key)).toEqual(Array.from({ length: STOPS }, (_, at) => `stop:${at}`))
  // O foco nasce no "list", não num link.
  expect(drawn[0]?.text).toMatch(/list/)
  // Só o link que abre tem botão; o quebrado e o de fora são texto marcado.
  expect(drawn.slice(STOPS).map(button => button.key)).toEqual(['link:docs/arq.md'])
  expect(await ui.find({ type: 'Text', text: /nope\.md \(broken\)/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /fora\.md \(outside project\)/ })).toBeDefined()

  // Seguir o link e voltar por qualquer parada das primeiras.
  await ui.press({ key: 'link:docs/arq.md' })
  await ui.redraw()
  expect((await buttons(ui))[0]?.text).toMatch(/back/)
  await ui.press({ key: 'stop:7' })
  await ui.redraw()
  expect((await buttons(ui))[0]?.text).toMatch(/list/)
  await ui.unmount()
})

test('lista: cabe no corpo de 3 linhas e voltar da leitura limpa o filtro', async ($, on) => {
  fakeProject(on)
  await $.command.run(MD('zzz'))
  const ui = await $.ui.mount({ plugin: PANE, surface: 'terminal', component: 'Pane', requestId: PANE, props: props(3) })
  expect(await ui.find({ type: 'Text', text: /Nothing matches "zzz"/ })).toBeDefined()

  // Abrir um doc direto e voltar: a lista vem inteira, não "0 of 2".
  await $.command.run(MD('docs/arq.md'))
  await ui.redraw()
  await ui.press({ key: 'stop:0' })
  await ui.redraw()
  expect(await ui.find({ type: 'Text', text: /Nothing matches/ })).toBeUndefined()
  const rows = (await buttons(ui)).filter(button => button.key.startsWith('row:'))
  // Corpo de 3 linhas: filtro, um arquivo, dicas.
  expect(rows.map(button => button.text)).toEqual(['README.md'])
  expect((await ui.find({ key: 'more' }))?.text).toBe('1/2')
  await ui.unmount()
})

// Célula de 14 linhas: o corpo do painel tem duas. O filtro fica, e a linha
// de teclas é a última coisa a cair — as linhas da lista caem antes dela.
test('lista em corpo de 2 linhas: filtro e teclas, sem linha de arquivo', async ($, on) => {
  fakeProject(on)
  await $.command.run(MD(''))
  const ui = await $.ui.mount({ plugin: PANE, surface: 'terminal', component: 'Pane', requestId: PANE, props: props(2) })

  expect(await ui.find({ type: 'Input' })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /Enter opens first {2}Esc close/ })).toBeDefined()
  expect((await buttons(ui)).filter(button => button.key.startsWith('row:'))).toEqual([])

  // Com uma linha a mais a lista volta, e as teclas continuam.
  await ui.unmount()
  const taller = await $.ui.mount({ plugin: PANE, surface: 'terminal', component: 'Pane', requestId: PANE, props: props(3) })
  expect((await buttons(taller)).filter(button => button.key.startsWith('row:')).length).toBe(1)
  expect(await taller.find({ type: 'Text', text: /Enter read {2}Esc close/ })).toBeDefined()
  await taller.unmount()
})
