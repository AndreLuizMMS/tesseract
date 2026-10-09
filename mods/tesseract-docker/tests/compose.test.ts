import { expect, test } from 'claude-code/testing'

import { firstLine, parseServices, scoreFile, shortenTime, tail } from '../hooks/compose'

test('arquivo de compose: canônico na raiz vence, produção nunca entra', () => {
  expect(scoreFile('docker-compose.yml', true)).toBeGreaterThan(scoreFile('compose.yaml', true))
  expect(scoreFile('compose.yml', true)).toBeGreaterThan(scoreFile('docker-compose.yml', false))
  expect(scoreFile('compose.dev.yml', false)).toBeGreaterThan(scoreFile('compose.ci.yml', false))
  for (const name of ['docker-compose.prod.yml', 'compose.staging.yaml', 'compose.override.yml', 'compose.json', 'README.md']) {
    expect(scoreFile(name, true)).toBe(0)
  }
})

test('serviços: um objeto por linha ou a lista inteira', () => {
  const lines = [
    '{"Service":"api","State":"running","Status":"Up 2 hours (healthy)","Publishers":[{"PublishedPort":0},{"PublishedPort":8080}]}',
    '{"Service":"db","State":"exited","ExitCode":1,"Status":"Exited (1) 3 minutes ago","Publishers":null}',
  ].join('\n')
  expect(parseServices(lines)).toEqual([
    { name: 'api', state: 'up', port: ':8080', health: 'healthy', uptime: '2h' },
    { name: 'db', state: 'exited (1)', port: '', health: '', uptime: '' },
  ])
  expect(parseServices('[{"Service":"web","State":"created"}]')[0]?.state).toBe('created')
  expect(parseServices('\n')).toEqual([])
})

test('erro do compose sem o prefixo do comando', () => {
  expect(firstLine('\nno such service: nope\nusage: docker compose…')).toBe('no such service: nope')
  expect(shortenTime('2 hours 14 minutes')).toBe('2h14m')
  expect(shortenTime('Less than a second')).toBe('0s')
  expect(shortenTime('About an hour')).toBe('1h')
  expect(shortenTime('About a minute')).toBe('1m')
  expect(shortenTime('46 seconds')).toBe('46s')
  expect(shortenTime('3 weeks')).toBe('3w')
  expect(shortenTime('since forever')).toBe('since forever')
})

test('cauda do log: as últimas linhas, sem as vazias', () => {
  expect(tail('a\n\nb  \nc\n', 2)).toEqual(['b', 'c'])
  expect(tail('', 20)).toEqual([])
})
