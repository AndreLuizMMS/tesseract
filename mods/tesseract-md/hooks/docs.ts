// Regras de quais arquivos entram na lista, do filtro e dos links entre
// docs. Porte de internal/cell/md.go.

// Pastas que não guardam documentação e são caras de varrer.
const SKIP_DIRS = new Set([
  'node_modules', '.git', 'vendor', 'dist', 'build', 'target', '.next',
  '.venv', 'venv', 'coverage', '.cache', '.idea',
])

// Profundidade máxima a partir da raiz do projeto.
export const MAX_DEPTH = 6
// Teto que impede um projeto gigante de travar a varredura.
export const MAX_FILES = 2000

export const isMarkdown = (name: string): boolean => /\.(md|markdown)$/i.test(name)

// Pasta oculta não entra, salvo `.github`, onde moram docs de projeto.
export const isSkippedDir = (name: string): boolean =>
  SKIP_DIRS.has(name) || (name.startsWith('.') && name !== '.github')

const depthOf = (path: string): number => path.split('/').length - 1

// Mais raso primeiro: o README da raiz importa mais que um a quatro níveis.
export function sortDocs(paths: readonly string[]): string[] {
  return [...paths].sort((a, b) => depthOf(a) - depthOf(b) || (a < b ? -1 : a > b ? 1 : 0))
}

// Os arquivos cujo caminho contém o filtro, sem diferenciar maiúsculas.
// O caminho conta inteiro: quem digita "docs/spec" sabe o que quer.
export function filterDocs(files: readonly string[], filter: string): string[] {
  const target = filter.trim().toLowerCase()

  return target === '' ? [...files] : files.filter(file => file.toLowerCase().includes(target))
}

// Resolve o destino de um link relativo ao doc que o contém. Devolve o
// caminho relativo à raiz, ou undefined se sai do projeto ou não é arquivo.
export function resolveLink(from: string, href: string): string | undefined {
  const target = href.split('#')[0] ?? ''
  if (target === '' || /^[a-z][a-z0-9+.-]*:/i.test(target)) return undefined

  const base = target.startsWith('/') ? [] : from.split('/').slice(0, -1)
  const parts = [...base]
  for (const part of decodeURIComponent(target).split('/')) {
    if (part === '' || part === '.') continue
    if (part !== '..') parts.push(part)
    else if (parts.pop() === undefined) return undefined
  }

  return parts.join('/')
}

export type DocLink = { label: string; href: string; path: string }

// Os links do texto que levam a outro markdown da lista, sem repetir destino.
export function docLinks(text: string, from: string, files: readonly string[]): DocLink[] {
  const known = new Set(files)

  return markdownLinks(text, from)
    .filter(link => link.state === 'inside' && known.has(link.path))
    .map(({ label, href, path }) => ({ label, href, path }))
}

export type LinkState =
  // Aponta para dentro do projeto; se o arquivo existe, quem sabe é o disco.
  | 'inside'
  // Sobe além da raiz do projeto: o leitor não alcança.
  | 'outside'

export type FoundLink = DocLink & { state: LinkState }

// Todo link do texto para um markdown local, sem repetir destino: os que
// ficam dentro do projeto e os que saem dele. Link para site, âncora e
// arquivo que não é markdown não entram.
export function markdownLinks(text: string, from: string): FoundLink[] {
  const seen = new Set<string>([from])
  const links: FoundLink[] = []
  for (const found of text.matchAll(/\[([^\]]+)\]\(\s*<?([^)\s>]+)>?[^)]*\)/g)) {
    const href = found[2] as string
    const target = href.split('#')[0] ?? ''
    if (target === '' || /^[a-z][a-z0-9+.-]*:/i.test(target) || !isMarkdown(target)) continue

    const path = resolveLink(from, href)
    // Fora do projeto não há caminho: o próprio href identifica o destino.
    const id = path ?? `outside:${target}`
    if (seen.has(id)) continue
    seen.add(id)
    links.push({ label: found[1] as string, href, path: path ?? target, state: path === undefined ? 'outside' : 'inside' })
  }

  return links
}

// Quantas linhas de tela o texto ocupa, por alto: cada linha do fonte
// dobrada na largura, e linha de tabela em dobro (o desenho põe uma régua
// entre elas). O desenho do markdown difere um pouco, então `fullRows` dá
// folga: é melhor sobrar linha em branco no fim do que o fim ficar fora.
export function roughRows(text: string, columns: number): number {
  const width = Math.max(columns, 1)

  return text.split('\n').reduce(
    (rows, line) => rows + Math.max(Math.ceil(line.length / width), 1) + (line.trimStart().startsWith('|') ? 1 : 0),
    0,
  )
}

export const fullRows = (text: string, columns: number): number => Math.ceil(roughRows(text, columns) * 1.15) + 2

// Até onde a leitura rola: o fim do texto ainda aparece.
export const maxOffset = (text: string, columns: number, room: number): number =>
  Math.max(fullRows(text, columns) - room, 0)
