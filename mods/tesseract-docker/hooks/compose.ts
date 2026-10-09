// Leitura do Docker Compose do projeto: achar o arquivo e ler os serviços.
// Porte de internal/docker/docker.go.

export type Service = {
  name: string
  // up, exited (1), created…
  state: string
  // Vazio quando o serviço não publica porta.
  port: string
  // Vazio quando não há healthcheck.
  health: string
  uptime: string
}

// Pastas que nunca entram na busca do compose: pesadas e sem stack.
export const IGNORED_DIRS = new Set([
  'node_modules', '.git', 'vendor', 'dist', 'build', 'target', '.next',
  '.venv', 'venv', 'coverage', 'tmp', '.cache',
])

// Arquivo com cara de produção nunca é escolhido sozinho.
const FORBIDDEN = ['prod', 'production', 'staging', 'homolog']

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

// A primeira linha da reclamação do Docker, sem o prefixo do comando: é o
// motivo, e cabe numa linha.
export function firstLine(text: string): string {
  return text.trim().split('\n')[0]?.trim() ?? ''
}

// As últimas `count` linhas não vazias de um log.
export function tail(text: string, count: number): string[] {
  return text
    .split('\n')
    .map(line => line.trimEnd())
    .filter(line => line !== '')
    .slice(-count)
}
