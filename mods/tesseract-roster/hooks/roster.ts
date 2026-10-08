// Regras do rol de sessões (quem está chamando) e da busca nas mensagens.

import type { RosterCalling, RosterCounts, RosterEntry, RosterMatch } from '../types'

// De quanto em quanto cada sessão regrava a própria entrada.
export const HEARTBEAT_MS = 30_000
// Entrada sem batida há mais que isto é de sessão morta: três batidas perdidas.
export const TTL_MS = 90_000
export const KEY_PREFIX = 's:'
export const MAX_MATCHES = 60

export const keyOf = (sessionId: string): string => `${KEY_PREFIX}${sessionId}`

const STATES: readonly RosterCalling[] = ['idle', 'working', 'responded', 'approve']

// Entrada bem formada e ainda dentro do prazo. O armazenamento é escrito por
// várias sessões e versões: nada nele é confiado sem conferir.
export function isFresh(entry: unknown, now: number): entry is RosterEntry {
  if (typeof entry !== 'object' || entry === null) return false

  const { state, at } = entry as Partial<RosterEntry>

  return typeof at === 'number' && STATES.includes(state as RosterCalling) && now - at <= TTL_MS && at <= now + TTL_MS
}

// Quantas das outras sessões vivas responderam ou esperam aprovação.
export function countOthers(entries: Readonly<Record<string, unknown>>, ownKey: string, now: number): RosterCounts {
  const counts: RosterCounts = { responded: 0, approve: 0 }
  for (const [key, entry] of Object.entries(entries)) {
    if (key === ownKey || !key.startsWith(KEY_PREFIX) || !isFresh(entry, now)) continue
    if (entry.state === 'responded') counts.responded += 1
    if (entry.state === 'approve') counts.approve += 1
  }

  return counts
}

// As chaves de sessão vencidas ou malformadas, para apagar.
export function expiredKeys(entries: Readonly<Record<string, unknown>>, now: number): string[] {
  return Object.entries(entries)
    .filter(([key, entry]) => key.startsWith(KEY_PREFIX) && !isFresh(entry, now))
    .map(([key]) => key)
}

// O que acontece numa sessão e muda o estado dela para quem espera.
export type RosterEvent =
  | 'turn.start'
  | 'turn.complete'
  // Pedido de permissão ou pergunta ao usuário.
  | 'ask'
  // A ferramenta que esperava resposta terminou.
  | 'tool.done'
  // A pessoa voltou à sessão: digitou ou enviou no prompt.
  | 'attend'

// O próximo estado da sessão. "Respondeu" só dura até a pessoa voltar: sem
// isso uma sessão já atendida contaria para sempre nas outras.
export function nextState(current: RosterCalling, event: RosterEvent): RosterCalling {
  if (event === 'turn.start') return 'working'
  if (event === 'turn.complete') return 'responded'
  if (event === 'ask') return 'approve'
  if (event === 'tool.done') return current === 'approve' ? 'working' : current

  return current === 'responded' ? 'idle' : current
}

export type BandPart = { kind: 'approve' | 'responded' | 'label'; text: string }

// O que a faixa mostra: aprovação antes de resposta, estado em maiúsculas
// como na faixa da célula; estreita, só glifo e número. Sem ninguém
// chamando, nada (a faixa não existe).
export function bandParts(counts: RosterCounts, columns: number): BandPart[] {
  const isNarrow = columns < 48
  const parts: BandPart[] = []
  if (counts.approve > 0) {
    parts.push({ kind: 'approve', text: `⏵ ${counts.approve}${isNarrow ? '' : ' APPROVE'}` })
  }
  if (counts.responded > 0) {
    parts.push({ kind: 'responded', text: `⬤ ${counts.responded}${isNarrow ? '' : ' RESPONDED'}` })
  }
  if (parts.length > 0 && !isNarrow) parts.push({ kind: 'label', text: 'other sessions' })

  return parts
}

export const hitsLabel = (shown: number, total: number): string =>
  total === 0 ? '' : total > shown ? `first ${shown} of ${total} hits` : `${total} ${total === 1 ? 'hit' : 'hits'}`

// As linhas do /ts-help: o comando numa coluna e a descrição ao lado,
// dobrada com recuo quando não cabe na largura.
export function helpLines(commands: readonly { name: string; description: string }[], columns: number): string[] {
  const indent = 9
  // O host recua a saída de um comando em 5 colunas (`  ⎿  `).
  const room = Math.max(columns - indent - 6, 16)
  const lines: string[] = []
  for (const command of commands) {
    let line = ''
    const wrapped: string[] = []
    for (const word of command.description.split(/\s+/)) {
      if (line !== '' && line.length + 1 + word.length > room) {
        wrapped.push(line)
        line = word
      } else line = line === '' ? word : `${line} ${word}`
    }
    wrapped.push(line)
    wrapped.forEach((text, index) => {
      lines.push(`${index === 0 ? `/${command.name}`.padEnd(indent) : ' '.repeat(indent)}${text}`)
    })
  }

  return lines
}

type Message = { role: 'user' | 'assistant'; text: string }

// As linhas das mensagens que contêm o termo, sem diferenciar maiúsculas,
// cada uma recortada em volta dele.
export function findMatches(messages: readonly Message[], term: string): { matches: RosterMatch[]; total: number } {
  const target = term.trim().toLowerCase()
  const matches: RosterMatch[] = []
  let total = 0
  if (target === '') return { matches, total }

  messages.forEach((message, index) => {
    for (const line of message.text.split('\n')) {
      const at = line.toLowerCase().indexOf(target)
      if (at < 0) continue
      total += 1
      if (matches.length >= MAX_MATCHES) continue
      const from = Math.max(at - 24, 0)
      const cut = line.slice(from, from + 160).trim()
      matches.push({ message: index + 1, role: message.role, snippet: `${from > 0 ? '…' : ''}${cut}` })
    }
  })

  return { matches, total }
}
