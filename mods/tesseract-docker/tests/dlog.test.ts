import type { On } from 'claude-code'
import { expect, test } from 'claude-code/testing'

const DLOG = {
  command: 'dlog',
  origin: { kind: 'composer' as const },
  presentation: { isFullscreen: false, columns: 80 },
}

const row = (name: string) => JSON.stringify({ Service: name, State: 'running', Status: 'Up 46 seconds', ExitCode: 0 })
const TWO = ['api', 'db'].map(row).join('\n')

// Um projeto com compose e um Docker de mentira: `answer` decide o que o
// `docker compose` devolve, `ran` guarda cada chamada e `filled` o que foi
// posto no prompt.
function fakeDocker(on: On, answer: { ps: string; logs: string; error: string }, hasCompose = true) {
  const ran: string[][] = []
  const filled: string[] = []
  on('session.root', () => ({ value: '/proj' }))
  on('command.run', () => ({}))
  on('fs.list', (_, e) => ({ value: hasCompose && e.path === '/proj' ? [{ name: 'compose.yml', kind: 'file' as const, size: 1, mtimeMs: 0, isLink: false }] : [] }))
  on('prompt.fill', (_, e) => {
    filled.push(e.text)

    return { isFilled: true }
  })
  on('process.run', (_, e) => {
    ran.push([...e.argv])

    return {
      value: {
        exitCode: answer.error === '' ? 0 : 1,
        stdout: e.argv.includes('ps') ? answer.ps : answer.logs,
        stderr: answer.error,
        isStdoutTruncated: false,
        isStderrTruncated: false,
      },
    }
  })

  return { ran, filled }
}

test('/dlog sem serviço: lista em texto, nada vai ao prompt', async ($, on) => {
  const { ran, filled } = fakeDocker(on, { ps: TWO, logs: '', error: '' })
  const out = await $.command.run({ ...DLOG, args: '' })

  expect(out.text).toContain('api\tup')
  expect(out.text).toContain('db\tup')
  expect(out.text).toContain('/dlog <service>')
  expect(filled).toEqual([])
  expect(ran.some(argv => argv.includes('logs'))).toBe(false)
})

test('/dlog com stack que nunca subiu termina nas teclas que sobem', async ($, on) => {
  fakeDocker(on, { ps: '', logs: '', error: '' })
  const out = await $.command.run({ ...DLOG, args: '' })

  expect(out.text).toContain('has never come up')
  expect(out.text).toContain('U brings it up')
})

test('/dlog em projeto sem compose diz onde pôr o arquivo', async ($, on) => {
  fakeDocker(on, { ps: '', logs: '', error: '' }, false)
  const out = await $.command.run({ ...DLOG, args: '' })

  expect(out.text).toContain('No compose file in this project. Add compose.yml at its root')
})

test('/dlog <serviço>: as últimas 20 linhas no prompt, sem cor nem controle', async ($, on) => {
  const logs = Array.from({ length: 30 }, (_, index) => `\u001b[32mline ${index + 1}\u001b[0m`).join('\n')
  const { ran, filled } = fakeDocker(on, { ps: TWO, logs, error: '' })
  const out = await $.command.run({ ...DLOG, args: ' api ' })

  expect(out.text).toBeUndefined()
  expect(ran.at(-1)).toEqual(['docker', 'compose', '--file', '/proj/compose.yml', 'logs', '--no-color', '--no-log-prefix', '--tail', '20', 'api'])
  expect(filled).toHaveLength(1)
  expect(filled[0]).toContain('Log of the `api` service, last 20 lines:')
  expect(filled[0]).toContain('\nline 11\n')
  expect(filled[0]).toContain('\nline 30\n')
  expect(filled[0]).not.toContain('line 10\n')
  expect(filled[0]).not.toContain('\u001b')
})

test('/dlog só aceita nome que o compose listou: flag não vira argumento', async ($, on) => {
  const { ran, filled } = fakeDocker(on, { ps: TWO, logs: 'x', error: '' })
  const out = await $.command.run({ ...DLOG, args: '--follow' })

  expect(out.text).toBe('No service named "--follow". Services here: api, db.')
  expect(filled).toEqual([])
  expect(ran.some(argv => argv.includes('logs'))).toBe(false)
})

test('Docker fora do ar: só o erro, sem lista velha', async ($, on) => {
  const { filled } = fakeDocker(on, { ps: TWO, logs: '', error: 'Cannot connect to the Docker daemon at unix:///var/run/docker.sock.' })
  const out = await $.command.run({ ...DLOG, args: 'api' })

  expect(out.text).toBe('docker compose ps failed: Cannot connect to the Docker daemon at unix:///var/run/docker.sock.')
  expect(filled).toEqual([])
})

test('tool restart: só reinicia serviço listado', async ($, on) => {
  const { ran } = fakeDocker(on, { ps: TWO, logs: '', error: '' })

  const refused = await $.tool.call({ tool: 'mcp__tesseract-docker__restart', service: 'down -v' })
  expect(refused.result).toBe('No service named "down -v". Services here: api, db.')
  expect(ran.some(argv => argv.includes('restart'))).toBe(false)

  const done = await $.tool.call({ tool: 'mcp__tesseract-docker__restart', service: 'db' })
  expect(done.result).toBe('Restarted db.')
  expect(ran.at(-1)).toEqual(['docker', 'compose', '--file', '/proj/compose.yml', 'restart', 'db'])
})
