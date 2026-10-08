import { expect, test } from 'claude-code/testing'

import { actionArgs, attention, firstLine, markOf, parseServices, scoreFile, shortenTime, summary } from '../hooks/compose'

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
  const services = parseServices(lines)
  expect(services).toEqual([
    { name: 'api', state: 'up', port: ':8080', health: 'healthy', uptime: '2h' },
    { name: 'db', state: 'exited (1)', port: '', health: '', uptime: '' },
  ])
  expect(parseServices('[{"Service":"web","State":"created"}]')[0]?.state).toBe('created')
  expect(parseServices('\n')).toEqual([])
  expect(summary(services)).toBe('1/2')
  expect(summary([])).toBe('stopped')
})

test('estado vira glifo antes de cor', () => {
  const service = { name: 'x', port: '', health: '', uptime: '' }
  expect(markOf({ ...service, state: 'up' }).glyph).toBe('▸')
  expect(markOf({ ...service, state: 'exited (1)' }).glyph).toBe('✖')
  expect(markOf({ ...service, state: 'exited (0)' }).glyph).toBe('○')
  expect(markOf({ ...service, state: 'created' }).glyph).toBe('○')
  expect(markOf({ ...service, state: 'exited (137)' }).glyph).toBe('○')
  expect(markOf({ ...service, state: 'exited (143)' }).glyph).toBe('○')
  expect(markOf({ ...service, state: 'restarting' }).glyph).toBe('▸')
  expect(markOf({ ...service, state: 'up', health: 'unhealthy' }).glyph).toBe('⚠')
})

test('ações: só as três, nunca destrutivas', () => {
  expect(actionArgs('up', 'api')).toEqual(['up', '-d', 'api'])
  expect(actionArgs('stop', '')).toEqual(['stop'])
  expect(actionArgs('restart', 'db')).toEqual(['restart', 'db'])
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

test('atenção: só o que quebrou ou adoeceu, parado de propósito não', () => {
  const service = { port: '', health: '', uptime: '' }
  const found = attention([
    { ...service, name: 'api', state: 'up' },
    { ...service, name: 'db', state: 'exited (1)' },
    { ...service, name: 'cache', state: 'exited (0)' },
    { ...service, name: 'web', state: 'up', health: 'unhealthy' },
  ])
  expect(found.crashed.map(one => one.name)).toEqual(['db'])
  expect(found.unhealthy.map(one => one.name)).toEqual(['web'])
})
