// Leitura do Docker Compose do projeto: achar o arquivo, ler os serviços e
// montar as ações. Porte de internal/docker/docker.go, sem nada destrutivo.

import type { Service } from '../types'
import type { Token } from './tokens'
import { GLYPH } from './tokens'

// Pastas que nunca entram na busca do compose: pesadas e sem stack.
export const IGNORED_DIRS = new Set([
  'node_modules', '.git', 'vendor', 'dist', 'build', 'target', '.next',
  '.venv', 'venv', 'coverage', 'tmp', '.cache',
])

// Arquivo com cara de produção nunca é escolhido sozinho.
const FORBIDDEN = ['prod', 'production', 'staging', 'homolog']

export type Action = 'up' | 'stop' | 'restart'

// Pontua um candidato a arquivo de compose; zero desqualifica. Nome canônico
// vence variante, raiz vence subpasta.
export function scoreFile(name: string, isAtRoot: boolean): number {
  const lower = name.toLowerCase()
  const extension = lower.endsWith('.yml') ? '.yml' : lower.endsWith('.yaml') ? '.yaml' : ''
  const isCandidate =
    lower.includes('compose') &&
    extension !== '' &&
    !lower.includes('override') &&
    !FORBIDDEN.some(banned => lower.includes(banned))
  if (!isCandidate) return 0

  const stem = lower.slice(0, -extension.length)
  const byName =
    stem === 'docker-compose' ? 44
    : stem === 'compose' ? 40
    : lower.includes('dev') || lower.includes('local') ? 20
    : 0

  return 10 + (extension === '.yml' ? 1 : 0) + byName + (isAtRoot ? 100 : 0)
}

type Raw = {
  Service?: string
  State?: string
  Status?: string
  Health?: string
  ExitCode?: number
  Publishers?: { PublishedPort?: number }[] | null
}

const healthOf = (raw: Raw): string => {
  const status = raw.Status ?? ''
  const inParens = /\(([^)]*)\)/.exec(status)?.[1] ?? ''
  const word = (raw.Health ?? '') !== '' ? (raw.Health as string) : inParens
  const lower = word.trim().toLowerCase()
  if (lower === 'healthy' || lower === 'unhealthy') return lower

  return lower === 'starting' || lower === 'health: starting' ? 'starting' : ''
}

// Troca "2 hours 14 minutes" por "2h14m" e "About a minute" por "1m", que é
// o que cabe na coluna.
export function shortenTime(text: string): string {
  if (text.toLowerCase().startsWith('less than')) return '0s'

  const units: [string, string][] = [
    ['second', 's'], ['minute', 'm'], ['hour', 'h'], ['day', 'd'], ['week', 'w'], ['month', 'mo'], ['year', 'y'],
  ]
  const parts = text
    .replace(/^about\s+/i, '')
    .split(/\s+/)
    .filter(part => part !== '')
  let short = ''
  for (let i = 0; i + 1 < parts.length; i += 2) {
    const count = /^an?$/i.test(parts[i] as string) ? '1' : (parts[i] as string)
    const unit = units.find(([name]) => (parts[i + 1] as string).toLowerCase().startsWith(name))
    // Formato desconhecido: melhor devolver como veio do que embaralhar.
    if (unit === undefined || !/^\d+$/.test(count)) return text
    short += `${count}${unit[1]}`
  }

  return short === '' ? text : short
}

const uptimeOf = (raw: Raw): string => {
  const status = raw.Status ?? ''
  if (!status.startsWith('Up')) return ''

  return shortenTime(status.slice(2).split('(')[0]?.trim() ?? '')
}

const translate = (raw: Raw): Service => {
  const state =
    raw.State === 'running' ? 'up'
    : raw.State === 'exited' ? `exited (${raw.ExitCode ?? 0})`
    : (raw.State ?? '') === '' ? '—'
    : (raw.State as string)
  const port = (raw.Publishers ?? []).find(one => (one.PublishedPort ?? 0) > 0)?.PublishedPort

  return {
    name: raw.Service ?? '',
    state,
    port: port === undefined ? '' : `:${port}`,
    health: healthOf(raw),
    uptime: uptimeOf(raw),
  }
}

// Lê a saída de `docker compose ps --format json`: um objeto por linha, ou
// a lista inteira em uma linha só, conforme a versão.
export function parseServices(out: string): Service[] {
  return out
    .split('\n')
    .map(line => line.trim())
    .filter(line => line !== '')
    .flatMap(line => {
      const parsed = JSON.parse(line) as Raw | Raw[]

      return Array.isArray(parsed) ? parsed.map(translate) : [translate(parsed)]
    })
}

// O que a linha de status mostra: quantos serviços estão de pé.
export function summary(services: readonly Service[]): string {
  const up = services.filter(service => service.state.startsWith('up')).length

  return up === 0 ? 'stopped' : `${up}/${services.length}`
}

// Os serviços que pedem atenção: quebrados (saíram com erro) e doentes
// (healthcheck falhando). Parado de propósito não entra.
export function attention(services: readonly Service[]): { crashed: Service[]; unhealthy: Service[] } {
  return {
    crashed: services.filter(service => markOf(service).token === 'dead'),
    unhealthy: services.filter(service => markOf(service).token === 'orphan'),
  }
}

// Glifo e cor de um serviço. Forma antes de cor, nunca verde nem ciano.
export function markOf(service: Service): { glyph: string; token: Token } {
  if (service.health === 'unhealthy') return { glyph: GLYPH.orphan, token: 'orphan' }
  // De pé é processo vivo produzindo: `▸`, como a célula e como o painel do
  // mosaico. `⬤` fica para "respondeu, tem algo para ler".
  if (service.state.startsWith('up')) return { glyph: GLYPH.working, token: 'working' }
  if (service.state === 'restarting' || service.health === 'starting') {
    return { glyph: GLYPH.working, token: 'working' }
  }
  const exit = /^exited \((\d+)\)$/.exec(service.state)?.[1]
  // 137 e 143 são SIGKILL e SIGTERM: o que um `stop` deixa. Parado, não quebrado.
  const isStopped = exit === '0' || exit === '137' || exit === '143'
  if (service.state === 'dead' || (exit !== undefined && !isStopped)) {
    return { glyph: GLYPH.dead, token: 'dead' }
  }

  return { glyph: GLYPH.off, token: 'off' }
}

// A primeira linha da reclamação do Docker, sem o prefixo do comando: o
// painel tem uma linha estreita e o motivo precisa caber nela.
export function firstLine(text: string): string {
  return text.trim().split('\n')[0]?.trim() ?? ''
}

// Os argumentos do compose para uma ação. Só existem estas três: é assim
// que nada destrutivo entra. Serviço vazio = a stack inteira.
export function actionArgs(action: Action, service: string): string[] {
  const verb = action === 'up' ? ['up', '-d'] : [action]

  return service === '' ? verb : [...verb, service]
}

export const describeAction = (action: Action, service: string): string => {
  const verb = action === 'up' ? 'bringing up' : action === 'stop' ? 'stopping' : 'restarting'

  return `${verb} ${service === '' ? 'the whole stack' : service}`
}

// As últimas `count` linhas não vazias de um log.
export function tail(text: string, count: number): string[] {
  return text
    .split('\n')
    .map(line => line.trimEnd())
    .filter(line => line !== '')
    .slice(-count)
}
