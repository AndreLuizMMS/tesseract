import { expect, test } from 'claude-code/testing'

import { HEARTBEAT_MS, TTL_MS, bandParts, countOthers, expiredKeys, findMatches, helpLines, hitsLabel, isFresh, keyOf, nextState } from '../hooks/roster'

const NOW = 1_000_000
const entry = (state: string, age: number) => ({ state, at: NOW - age, cwd: '/x' })

test('expiração: entrada vence depois de três batidas perdidas', () => {
  expect(TTL_MS).toBe(HEARTBEAT_MS * 3)
  expect(isFresh(entry('responded', 0), NOW)).toBe(true)
  expect(isFresh(entry('responded', TTL_MS), NOW)).toBe(true)
  expect(isFresh(entry('responded', TTL_MS + 1), NOW)).toBe(false)
})

test('expiração: lixo e relógio do futuro não contam', () => {
  for (const junk of [null, undefined, 'x', 3, {}, { state: 'responded' }, { at: NOW }, { state: 'crashed', at: NOW }]) {
    expect(isFresh(junk, NOW)).toBe(false)
  }
  expect(isFresh(entry('approve', -TTL_MS - 1), NOW)).toBe(false)
})

test('contadores: só as OUTRAS sessões, vivas, que pedem ação', () => {
  const entries = {
    [keyOf('me')]: entry('approve', 0),
    [keyOf('a')]: entry('responded', 1_000),
    [keyOf('b')]: entry('approve', 2_000),
    [keyOf('c')]: entry('approve', 3_000),
    [keyOf('d')]: entry('working', 0),
    [keyOf('e')]: entry('idle', 0),
    [keyOf('dead')]: entry('approve', TTL_MS + 5),
    [keyOf('junk')]: 'oops',
    ide: entry('approve', 0),
  }
  expect(countOthers(entries, keyOf('me'), NOW)).toEqual({ responded: 1, approve: 2 })
  expect(countOthers({}, keyOf('me'), NOW)).toEqual({ responded: 0, approve: 0 })
  expect(expiredKeys(entries, NOW).sort()).toEqual([keyOf('dead'), keyOf('junk')].sort())
})

test('busca: por linha, sem diferenciar maiúsculas, com o número da mensagem', () => {
  const messages = [
    { role: 'user' as const, text: 'sobe o Docker\ne roda a migration' },
    { role: 'assistant' as const, text: 'Rodei a MIGRATION 0042.\nNada mais.' },
    { role: 'user' as const, text: 'ok' },
  ]
  expect(findMatches(messages, 'migration')).toEqual({
    matches: [
      { message: 1, role: 'user', snippet: 'e roda a migration' },
      { message: 2, role: 'assistant', snippet: 'Rodei a MIGRATION 0042.' },
    ],
    total: 2,
  })
  expect(findMatches(messages, '  ')).toEqual({ matches: [], total: 0 })
  expect(findMatches(messages, 'kubernetes').total).toBe(0)
})

test('busca: linha longa é recortada em volta do termo e a lista tem teto', () => {
  const long = `${'a'.repeat(100)} alvo ${'b'.repeat(300)}`
  const found = findMatches([{ role: 'assistant', text: long }], 'alvo').matches[0]
  expect(found?.snippet.startsWith('…')).toBe(true)
  expect(found?.snippet).toContain('alvo')
  expect(found?.snippet.length).toBeLessThanOrEqual(161)

  const many = findMatches([{ role: 'user', text: Array(200).fill('x alvo').join('\n') }], 'alvo')
  expect(many.total).toBe(200)
  expect(many.matches.length).toBe(60)
})

test('estado: "respondeu" volta a ocioso quando a pessoa retoma a sessão', () => {
  expect(nextState('idle', 'turn.start')).toBe('working')
  expect(nextState('working', 'turn.complete')).toBe('responded')
  // Digitar ou enviar no prompt é ter visto a resposta.
  expect(nextState('responded', 'attend')).toBe('idle')
  // Um turno novo também tira o "respondeu".
  expect(nextState('responded', 'turn.start')).toBe('working')
  // Digitar não tira quem trabalha nem quem espera aprovação.
  expect(nextState('working', 'attend')).toBe('working')
  expect(nextState('approve', 'attend')).toBe('approve')
  expect(nextState('idle', 'attend')).toBe('idle')
  expect(nextState('working', 'ask')).toBe('approve')
  expect(nextState('approve', 'tool.done')).toBe('working')
  expect(nextState('responded', 'tool.done')).toBe('responded')
})

test('faixa: sem ninguém chamando não há faixa; estado em maiúsculas; estreita encurta', () => {
  expect(bandParts({ responded: 0, approve: 0 }, 76)).toEqual([])
  expect(bandParts({ responded: 2, approve: 1 }, 76).map(part => part.text)).toEqual([
    '⏵ 1 APPROVE',
    '⬤ 2 RESPONDED',
    'other sessions',
  ])
  expect(bandParts({ responded: 2, approve: 1 }, 30).map(part => part.text)).toEqual(['⏵ 1', '⬤ 2'])
  expect(bandParts({ responded: 1, approve: 0 }, 76).map(part => part.kind)).toEqual(['responded', 'label'])
  // O ⧉ é da faixa da célula: aqui ele não se repete.
  expect(bandParts({ responded: 9, approve: 9 }, 76).some(part => part.text.includes('⧉'))).toBe(false)
})

test('busca: plural do contador', () => {
  expect(hitsLabel(0, 0)).toBe('')
  expect(hitsLabel(1, 1)).toBe('1 hit')
  expect(hitsLabel(2, 2)).toBe('2 hits')
  expect(hitsLabel(60, 200)).toBe('first 60 of 200 hits')
})

test('ajuda: descrição longa dobra com recuo, nunca na coluna zero', () => {
  const lines = helpLines(
    [
      { name: 'docker', description: 'Docker Compose panel: services, start, stop, restart, log tail' },
      { name: 'md', description: 'Markdown reader' },
    ],
    40,
  )
  expect(lines).toEqual([
    '/docker  Docker Compose panel:',
    '         services, start, stop,',
    '         restart, log tail',
    '/md      Markdown reader',
  ])
  // Sobram as 5 colunas do recuo que o host põe na saída do comando.
  expect(lines.every(line => line.length <= 40 - 5)).toBe(true)
  expect(helpLines([{ name: 'find', description: 'Search this session' }], 80)).toEqual(['/find    Search this session'])
})
