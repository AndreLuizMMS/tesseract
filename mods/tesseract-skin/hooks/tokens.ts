// Único lugar do mod com cor e glifo. Espelha os tokens de
// internal/theme/theme.go, que é quem desenha o mosaico em volta da sessão.
//
// Duas cores do mosaico ficam de fora de propósito. O verde fósforo (#55FFA6)
// é onde a digitação cai, um por tela, e quem o acende é o mosaico (a palavra
// TYPE na célula). E o matiz do projeto é da borda da célula: aqui dentro só
// existem a escada neutra e as cores de estado.

const brand = {
  bgBase: '#070B0C',
  bgRaised: '#121C1F',
  fgFaint: '#3E534E',
  fgMuted: '#6C8076',
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
  // Turno parado no meio: o círculo pela metade, como no cabeçalho da célula.
  interrupted: '◐',
  done: '·',
} as const

// Com cor. Tudo aqui é token da marca, então vai em hex. O texto comum não
// recebe cor: fica com a do tema do Claude Code (dark-ansi na célula). As
// chaves de tema `subtle` e `inactive` não servem para fg.muted/fg.faint:
// medidas no dark-ansi, as duas saem como ANSI 7, a cor do texto comum.
const color = {
  // O ciano é só a marca: o ⧉ da faixa, fora do Tesseract.
  structure: { color: brand.flux },
  // Nome de ferramenta, rótulo, modos: um degrau abaixo do texto comum.
  label: { color: brand.fgMuted },
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
  label: { dimColor: true },
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
