// O que uma sessão está fazendo, do ponto de vista de quem espera por ela.
export type RosterCalling = 'idle' | 'working' | 'responded' | 'approve'

// O que cada sessão grava no armazenamento compartilhado, sob a própria chave.
export type RosterEntry = {
  state: RosterCalling
  // Quando foi gravado, em ms; entrada velha expira.
  at: number
  cwd: string
}

// Quantas OUTRAS sessões pedem ação.
export type RosterCounts = { responded: number; approve: number }

export type RosterMatch = {
  // Posição da mensagem na conversa, a partir de 1.
  message: number
  role: 'user' | 'assistant'
  // A linha onde o termo apareceu, já recortada em volta dele.
  snippet: string
}

export type RosterFind = {
  term: string
  matches: RosterMatch[]
  // Total achado, que pode passar do que a lista guarda.
  total: number
  // Mensagem aberta por inteiro; 0 = nenhuma.
  expanded: number
  // O texto da mensagem aberta.
  body: string
  // Resultado escolhido (posição em `matches`) e o primeiro visível: a
  // lista rola com o foco.
  selected: number
  top: number
  // Onde o anel de foco está: 'term', 'row:<n>' ou '' (fora do painel).
  ring: string
  // Primeira linha visível da mensagem aberta.
  bodyTop: number
}

declare module 'claude-code' {
  interface PluginState {
    'tesseract-roster': { counts: RosterCounts; hints: string; find: RosterFind }
  }
}
