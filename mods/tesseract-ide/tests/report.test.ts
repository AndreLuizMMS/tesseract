import type { On } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'

import { lineOf } from '../hooks/register'

const CELL = { TESSERACT: '1', TESSERACT_CELL_ID: 'c1', TESSERACT_BIN: '/opt/ts' }
const SOURCE = 'package main\n\nfunc main() {\n\tprintln("hi")\n}\n'

// O chão debaixo do mod: o ambiente, o arquivo no disco e o `ts` falso que
// guarda cada relato.
function world(on: On, env: Record<string, string>, isError = false) {
  const reports: unknown[] = []
  mock.env(on, env)
  on('session.cwd', () => ({ value: '/proj' }))
  on('tool.call', () => ({ result: 'ran', isError }))
  on('fs.read', () => ({ value: SOURCE }))
  on('process.run', (_$, e) => {
    expect(e.argv.slice(0, 2)).toEqual(['/opt/ts', 'report'])
    reports.push(JSON.parse(e.argv[2] ?? '{}'))

    return { value: { exitCode: 0, stdout: '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })

  return reports
}

test('linha: onde o trecho começa, 1 quando não acha', () => {
  expect(lineOf(SOURCE, '\tprintln("hi")')).toBe(4)
  expect(lineOf(SOURCE, 'package')).toBe(1)
  expect(lineOf(SOURCE, 'nope')).toBe(1)
  expect(lineOf(SOURCE, '')).toBe(1)
})

test('em célula: Edit relata o arquivo e a linha do trecho novo; Write, a linha 1', async ($, on) => {
  const reports = world(on, CELL)

  await $.tool.call({ tool: 'Edit', file_path: '/proj/main.go', old_string: 'x', new_string: '\tprintln("hi")\n}' })
  await $.tool.call({ tool: 'Write', file_path: 'notes/todo.txt', content: 'a' })

  expect(reports).toEqual([
    { kind: 'edit', file: '/proj/main.go', line: 4 },
    { kind: 'edit', file: '/proj/notes/todo.txt', line: 1 },
  ])
})

test('fora de célula: nada', async ($, on) => {
  const loose = world(on, {})
  await $.tool.call({ tool: 'Write', file_path: '/proj/main.go', content: 'a' })
  expect(loose).toEqual([])
})

test('sessão de fundo da célula fica muda', async ($, on) => {
  const reports = world(on, { ...CELL, CLAUDE_CODE_SESSION_KIND: 'bg' })
  await $.tool.call({ tool: 'Write', file_path: '/proj/main.go', content: 'a' })
  expect(reports).toEqual([])
})

test('edição que falhou não relata', async ($, on) => {
  const reports = world(on, CELL, true)
  await $.tool.call({ tool: 'Edit', file_path: '/proj/main.go', old_string: 'x', new_string: 'y' })
  expect(reports).toEqual([])
})
