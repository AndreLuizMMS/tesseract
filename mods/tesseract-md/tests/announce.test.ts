import type { On } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'

import { isMarkdown } from '../hooks/register'

const CELL = { TESSERACT: '1', TESSERACT_CELL_ID: 'c1', TESSERACT_BIN: '/opt/ts' }

// O chão debaixo do mod: o ambiente, o `ts` falso que guarda cada relato e
// os avisos que a sessão mostrou.
function world(on: On, env: Record<string, string>, isError = false) {
  const reports: unknown[] = []
  const toasts: string[] = []
  mock.env(on, env)
  on('session.root', () => ({ value: '/proj' }))
  on('session.cwd', () => ({ value: '/proj/api' }))
  on('tool.call', () => ({ result: 'ran', isError }))
  on('ui.toast', (_$, e) => {
    toasts.push(e.text)

    return { value: undefined }
  })
  on('process.run', (_$, e) => {
    expect(e.argv.slice(0, 2)).toEqual(['/opt/ts', 'report'])
    reports.push(JSON.parse(e.argv[2] ?? '{}'))

    return { value: { exitCode: 0, stdout: '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })

  return { reports, toasts }
}

test('só markdown conta', () => {
  expect(isMarkdown('/proj/README.md')).toBe(true)
  expect(isMarkdown('notes.MARKDOWN')).toBe(true)
  expect(isMarkdown('readme.mdx')).toBe(false)
  expect(isMarkdown('md')).toBe(false)
})

test('em célula: cada markdown escrito vira relato ao engine, sem aviso na sessão', async ($, on) => {
  const { reports, toasts } = world(on, CELL)

  await $.tool.call({ tool: 'Write', file_path: '/proj/docs/plan.md', content: '# plan' })
  await $.tool.call({ tool: 'Edit', file_path: 'notes.md', old_string: 'a', new_string: 'b' })
  await $.tool.call({ tool: 'Write', file_path: '/proj/main.go', content: 'package main' })

  expect(reports).toEqual([
    { kind: 'doc', file: '/proj/docs/plan.md' },
    { kind: 'doc', file: '/proj/api/notes.md' },
  ])
  expect(toasts).toEqual([])
})

test('sessão de fundo da célula fica muda', async ($, on) => {
  const { reports, toasts } = world(on, { ...CELL, CLAUDE_CODE_SESSION_KIND: 'bg' })

  await $.tool.call({ tool: 'Write', file_path: '/proj/docs/plan.md', content: '# plan' })

  expect(reports).toEqual([])
  expect(toasts).toEqual([])
})

test('fora de célula: um aviso com o caminho, uma vez por arquivo', async ($, on) => {
  const { reports, toasts } = world(on, {})

  await $.tool.call({ tool: 'Write', file_path: '/proj/docs/plan.md', content: '# plan' })
  await $.tool.call({ tool: 'Edit', file_path: '/proj/docs/plan.md', old_string: 'a', new_string: 'b' })
  await $.tool.call({ tool: 'Write', file_path: '/elsewhere/x.md', content: 'x' })

  expect(reports).toEqual([])
  expect(toasts).toEqual(['Wrote docs/plan.md.', 'Wrote /elsewhere/x.md.'])
})

test('edição que falhou não avisa', async ($, on) => {
  const { reports } = world(on, CELL, true)

  await $.tool.call({ tool: 'Write', file_path: '/proj/docs/plan.md', content: '# plan' })

  expect(reports).toEqual([])
})
