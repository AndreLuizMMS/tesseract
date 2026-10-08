import { expect, test } from 'claude-code/testing'

import { docLinks, filterDocs, isMarkdown, isSkippedDir, markdownLinks, maxOffset, resolveLink, roughRows, sortDocs } from '../hooks/docs'

const FILES = ['README.md', 'docs/arquitetura.md', 'docs/tema.md', 'docs/spec/API.md', 'CHANGELOG.markdown']

test('lista: só markdown, pastas pesadas e ocultas ficam de fora', () => {
  expect(isMarkdown('README.md')).toBe(true)
  expect(isMarkdown('notes.MARKDOWN')).toBe(true)
  expect(isMarkdown('readme.mdx')).toBe(false)
  expect(isMarkdown('md')).toBe(false)
  expect(isSkippedDir('node_modules')).toBe(true)
  expect(isSkippedDir('.git')).toBe(true)
  expect(isSkippedDir('.vscode')).toBe(true)
  expect(isSkippedDir('.github')).toBe(false)
  expect(isSkippedDir('docs')).toBe(false)
})

test('lista: mais raso primeiro, depois alfabético', () => {
  expect(sortDocs(['docs/spec/API.md', 'docs/tema.md', 'README.md', 'docs/arquitetura.md', 'CHANGELOG.markdown'])).toEqual([
    'CHANGELOG.markdown',
    'README.md',
    'docs/arquitetura.md',
    'docs/tema.md',
    'docs/spec/API.md',
  ])
})

test('filtro: nome ou caminho, sem diferenciar maiúsculas', () => {
  expect(filterDocs(FILES, '')).toEqual(FILES)
  expect(filterDocs(FILES, '   ')).toEqual(FILES)
  expect(filterDocs(FILES, 'TEMA')).toEqual(['docs/tema.md'])
  expect(filterDocs(FILES, 'docs/spec')).toEqual(['docs/spec/API.md'])
  expect(filterDocs(FILES, 'api')).toEqual(['docs/spec/API.md'])
  expect(filterDocs(FILES, 'nada')).toEqual([])
})

test('links: resolvem a partir do doc e nunca saem do projeto', () => {
  expect(resolveLink('docs/tema.md', 'arquitetura.md')).toBe('docs/arquitetura.md')
  expect(resolveLink('docs/tema.md', './spec/API.md#rotas')).toBe('docs/spec/API.md')
  expect(resolveLink('docs/spec/API.md', '../../README.md')).toBe('README.md')
  expect(resolveLink('docs/tema.md', '/README.md')).toBe('README.md')
  expect(resolveLink('README.md', '../fora.md')).toBeUndefined()
  expect(resolveLink('README.md', 'https://example.com/a.md')).toBeUndefined()
  expect(resolveLink('README.md', '#secao')).toBeUndefined()
})

test('links: só os que levam a outro markdown da lista, sem repetir', () => {
  const text = [
    'Veja a [arquitetura](docs/arquitetura.md) e o [tema](./docs/tema.md "título").',
    'De novo a [arquitetura](docs/arquitetura.md#motor), o [site](https://example.com),',
    'um [código](internal/x.go), um [sumido](docs/sumido.md) e [este](README.md).',
  ].join('\n')
  expect(docLinks(text, 'README.md', FILES).map(link => link.path)).toEqual(['docs/arquitetura.md', 'docs/tema.md'])
})

test('links: quebrado e para fora do projeto aparecem, cada um com seu estado', () => {
  const text = [
    'Veja a [arquitetura](docs/arquitetura.md), um [sumido](docs/sumido.md),',
    'o [de fora](../fora.md), de novo o [de fora](../fora.md#x), o [site](https://example.com/a.md),',
    'uma [âncora](#secao) e um [código](internal/x.go).',
  ].join('\n')
  expect(markdownLinks(text, 'README.md').map(link => `${link.path} ${link.state}`)).toEqual([
    'docs/arquitetura.md inside',
    // Sumido do disco ou não, é "dentro": quem diz se existe é o disco.
    'docs/sumido.md inside',
    '../fora.md outside',
  ])
})

test('rolagem: o limite acompanha o tamanho do texto e nunca é negativo', () => {
  expect(roughRows('a\nb\n\nc', 40)).toBe(4)
  expect(roughRows('x'.repeat(100), 40)).toBe(3)
  expect(maxOffset('curto', 40, 10)).toBe(0)
  // Linha de tabela conta em dobro: o desenho põe régua entre elas.
  expect(roughRows('| a | b |\n| - | - |\n| 1 | 2 |', 40)).toBe(6)
  expect(maxOffset(Array(100).fill('linha').join('\n'), 40, 10)).toBeGreaterThanOrEqual(90)
})
