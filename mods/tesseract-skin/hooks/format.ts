// Funções puras de texto: nada aqui toca `$`, então os testes as chamam direto.

// 3s, 1m 4s, 2h 5m.
export const duration = (ms: number): string => {
  const seconds = Math.max(0, Math.round(ms / 1000))
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}

// Corta em `width` células, com reticências. Conta por code point: os glifos
// da marca são todos de uma célula.
export const cut = (text: string, width: number): string => {
  const chars = [...text]
  if (chars.length <= width) return text
  return width <= 1 ? chars.slice(0, width).join('') : `${chars.slice(0, width - 1).join('')}…`
}

const size = (text: string): number => [...text].length

// Qual campo do input resume a chamada. Também é a lista das ferramentas que o
// mod redesenha: as demais (Agent, tarefas, perguntas) têm linha viva própria
// no Claude Code e ficam como estão.
const ARGUMENT: Readonly<Record<string, string>> = {
  Bash: 'command',
  Read: 'file_path',
  Edit: 'file_path',
  Write: 'file_path',
  NotebookEdit: 'notebook_path',
  Glob: 'pattern',
  Grep: 'pattern',
  WebFetch: 'url',
  WebSearch: 'query',
  Skill: 'skill',
  ToolSearch: 'query',
}

export type ToolLine = { name: string; argument: string }

// "Bash" + "pnpm migrate"; undefined para ferramenta que o mod não redesenha.
export const toolLine = (tool: string, input: unknown, cwd: string): ToolLine | undefined => {
  const fields: Record<string, unknown> =
    typeof input === 'object' && input !== null ? (input as Record<string, unknown>) : {}
  const mcp = /^mcp__(.+?)__(.+)$/.exec(tool)
  const field = ARGUMENT[tool]
  if (mcp === null && field === undefined) return undefined

  const raw = mcp === null ? fields[field ?? ''] : Object.values(fields).find(value => typeof value === 'string')
  const text = typeof raw === 'string' ? raw : ''
  const [first = '', ...rest] = (text.startsWith(`${cwd}/`) ? text.slice(cwd.length + 1) : text).split('\n')

  return {
    name: mcp === null ? tool : `${mcp[1]}:${mcp[2]}`,
    argument: rest.length > 0 ? `${first} …` : first,
  }
}

// O que a linha de approve diz que está esperando a pessoa, e as teclas que
// resolvem: aprovação de ferramenta, pergunta do agente ou plano pronto.
export const askLine = (tool: string, input: unknown, cwd: string): { what: string; keys: string } => {
  if (tool === 'AskUserQuestion') {
    const questions = (input as { questions?: unknown } | null)?.questions
    const first = Array.isArray(questions) ? (questions[0] as { question?: unknown } | undefined)?.question : undefined

    return { what: typeof first === 'string' ? first : 'question', keys: 'Enter answers, Esc declines' }
  }
  if (tool === 'ExitPlanMode') return { what: 'plan ready for review', keys: 'Enter approves, Esc declines' }

  const line = toolLine(tool, input, cwd)

  return { what: line === undefined ? tool : `${line.name}: ${line.argument}`, keys: 'Enter approves, Esc declines' }
}

// Partes da faixa que cabem em `width`: as opcionais (quota) saem do fim para o
// começo; se ainda não couber, o rótulo (primeira fixa) é cortado.
export const fitBand = (
  fixed: readonly string[],
  optional: readonly string[],
  width: number,
  separator = ' · ',
): string[] => {
  const total = (parts: readonly string[]): number =>
    parts.reduce((sum, part) => sum + size(part), 0) + separator.length * Math.max(0, parts.length - 1)

  const kept = [...optional]
  while (kept.length > 0 && total([...fixed, ...kept]) > width) kept.pop()

  const [label = '', ...rest] = fixed
  const room = width - (total(['', ...rest]) - 0)
  return [cut(label, Math.max(1, room)), ...rest, ...kept]
}

// Tom de voz da marca (// 08): seco e acionável, tecla primeiro.
const HINT: ReadonlyArray<readonly [RegExp, string]> = [
  [/\? for shortcuts/g, '? shortcuts'],
  [/esc to interrupt/gi, 'Esc interrupts'],
  // O Claude Code junta o modo e a dica com ` · `; os parênteses da dica
  // sobravam ali (`on · (Shift+Tab cycles)`) e, na quebra de linha de uma
  // célula estreita, ficavam colados no ponto.
  [/\(?shift\+tab to cycle\)?/gi, 'Shift+Tab cycles'],
  [/ctrl\+o to expand/gi, 'Ctrl+O expands'],
  [/ctrl\+t to (show|hide) (tasks|todos)/gi, 'Ctrl+T $1s $2'],
]

export const hintCopy = (hint: string): string =>
  HINT.reduce((text, [pattern, copy]) => text.replace(pattern, copy), hint)
