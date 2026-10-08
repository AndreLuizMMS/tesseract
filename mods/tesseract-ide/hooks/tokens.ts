// Tokens da identidade Tesseract (style/tesseract-brand-identity.html).
// Duplicado em cada mod de propósito: um plugin precisa ser autocontido.
//
// Leis: ciano = estrutura, verde fósforo = quem tem o teclado (um por tela),
// estado se lê pelo glifo antes da cor, e nunca em verde ou ciano.

export const HEX = {
  flux: '#22E0D0',
  fluxCore: '#128C86',
  phosphor: '#55FFA6',
  bright: '#E8F4EC',
  text: '#BFD1C6',
  muted: '#6C8076',
  faint: '#3E534E',
  working: '#6C8076',
  read: '#7DB7E8',
  block: '#FFB454',
  dead: '#FF3B47',
  off: '#3E534E',
  orphan: '#C77DFF',
} as const

export type Token = keyof typeof HEX

export const GLYPH = {
  mark: '⧉',
  cursor: '▌',
  working: '▸',
  read: '⬤',
  block: '⏵',
  dead: '✖',
  off: '○',
  orphan: '⚠',
} as const

let isMono = false

// NO_COLOR (qualquer valor não vazio) desliga toda cor; sobra glifo e negrito.
export const setMono = (value: string | undefined): void => {
  isMono = value !== undefined && value !== ''
}

export const isMonochrome = (): boolean => isMono

// Props de cor de um token, ou nada sob NO_COLOR. Espalhado no elemento
// (`<Text {...ink('flux')}>`) para nunca mandar `color: undefined`.
export const ink = (token: Token): { color?: string } =>
  isMono ? {} : { color: HEX[token] }

// Texto cortado ou preenchido até `width` células.
export const pad = (text: string, width: number): string =>
  text.length < width ? text.padEnd(width) : `${text.slice(0, Math.max(width - 2, 0))}… `
