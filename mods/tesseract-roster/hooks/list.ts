// Lista que cabe no corpo do painel e rola com o foco, e as contas de
// largura que mantêm o desenho dentro das linhas que a célula dá.
// Duplicado em cada mod de propósito: um plugin precisa ser autocontido.
//
// O host rola sozinho uma árvore mais alta que o corpo, e aí as setas passam
// a rolar em vez de andar pelo foco, e a linha de teclas some da vista. Por
// isso o mod desenha só a janela: a árvore nunca passa do corpo.

// O primeiro índice visível de uma janela de `rows` linhas sobre `total`
// itens que mantém `selected` à vista, movendo o mínimo a partir de `top`.
export function clampTop(top: number, selected: number, rows: number, total: number): number {
  const visible = Math.max(rows, 1)
  const kept = Math.max(Math.min(top, selected), selected - visible + 1)

  return Math.max(Math.min(kept, total - visible), 0)
}

export type Place = { selected: number; top: number }

// Anda `by` itens (negativo sobe) sem sair da lista, e leva a janela junto.
export function moveBy(place: Place, by: number, rows: number, total: number): Place {
  if (total <= 0) return { selected: 0, top: 0 }

  const selected = Math.min(Math.max(place.selected + by, 0), total - 1)

  return { selected, top: clampTop(place.top, selected, rows, total) }
}

// Quantas linhas uma fileira com quebra ocupa: itens de `widths` células,
// `gap` entre eles, em `columns` de largura.
export function wrapRows(widths: readonly number[], gap: number, columns: number): number {
  let rows = widths.length === 0 ? 0 : 1
  let used = 0
  for (const width of widths) {
    const needed = used === 0 ? width : used + gap + width
    if (used > 0 && needed > columns) {
      rows += 1
      used = width
    } else used = needed
  }

  return rows
}

// Texto cortado em `width` células, com reticências quando não cabe.
export const cut = (text: string, width: number): string =>
  text.length <= width ? text : width <= 1 ? text.slice(0, Math.max(width, 0)) : `${text.slice(0, width - 1)}…`

// Quantas linhas um texto ocupa quando quebra em `columns`.
export const wrappedRows = (text: string, columns: number): number =>
  text.split('\n').reduce((rows, line) => rows + Math.max(Math.ceil(line.length / Math.max(columns, 1)), 1), 0)
