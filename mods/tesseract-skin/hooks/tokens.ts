// Único lugar do mod com cor e glifo. Espelha os tokens de
// style/tesseract-brand-identity.html (:root e body.mono).
//
// O verde fósforo (#55FFA6) fica de fora de propósito: é posse do teclado, um
// por tela, e quem o acende é o mosaico (borda da célula em TYPE). Um segundo
// verde dentro da sessão quebraria a regra.

const brand = {
  bgBase: '#070B0C',
  bgRaised: '#121C1F',
  fgFaint: '#3E534E',
  fgMuted: '#6C8076',
  fluxCore: '#128C86',
  flux: '#22E0D0',
  stateWorking: '#6C8076',
  stateRead: '#7DB7E8',
  stateBlock: '#FFB454',
  stateBlockDim: '#835F30',
  stateDead: '#FF3B47',
  stateOff: '#3E534E',
} as const

export const glyph = {
  brand: '⧉',
  prompt: '›',
  working: '▸',
  responded: '⬤',
  approve: '⏵',
  crashed: '✖',
  stopped: '○',
  done: '·',
} as const

// Com cor. Tudo aqui é token da marca, então vai em hex. O texto comum não
// recebe cor: fica com a do tema do Claude Code (dark-ansi na célula). As
// chaves de tema `subtle` e `inactive` não servem para fg.muted/fg.faint:
// medidas no dark-ansi, as duas saem como ANSI 7, a cor do texto comum.
const color = {
  structure: { color: brand.flux },
  label: { color: brand.fluxCore },
  working: { color: brand.stateWorking },
  responded: { color: brand.stateRead },
  crashed: { color: brand.stateDead },
  stopped: { color: brand.stateOff },
  muted: { color: brand.fgMuted },
  faint: { color: brand.fgFaint },
  strong: { bold: true },
  userRow: { backgroundColor: brand.bgRaised },
  // Approve dentro da célula: forma e cor, sem área (a barra é do mosaico).
  approveMark: { color: brand.stateBlock, bold: true },
  // Barra sólida: fundo na cor do estado, texto em bg.base, como no mosaico.
  approve: { backgroundColor: brand.stateBlock, color: brand.bgBase, bold: true },
  // Quadro escuro do pisca: âmbar a meia força, continua sendo barra preenchida.
  approveDimmed: { backgroundColor: brand.stateBlockDim, color: brand.bgBase, bold: true },
} as const

// Sem cor (NO_COLOR): só forma e atributo. A barra de approve é a única área
// invertida, e não pisca: tirar o vídeo invertido tiraria a área preenchida.
// Dentro da célula essa área é a do mosaico; aqui a marca fica só em negrito.
const mono = {
  structure: {},
  label: {},
  working: { dimColor: true },
  responded: {},
  crashed: { bold: true },
  stopped: { dimColor: true },
  muted: { dimColor: true },
  faint: { dimColor: true },
  strong: { bold: true },
  userRow: {},
  approveMark: { bold: true },
  approve: { inverse: true, bold: true },
  approveDimmed: { inverse: true, bold: true },
} as const

export type Styles = { readonly [K in keyof typeof color]: (typeof color | typeof mono)[K] }

export const styles = (noColor: boolean): Styles => (noColor ? mono : color)
