import type { PromptEditInput } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'

// `prompt.edit` roda a cada tecla e tem 50 ms. O mod só observa: a tecla
// tem de passar mesmo que a gravação do estado nunca termine.
test('digitar não espera a gravação do estado', async ($, on) => {
  mock.clock(on, { now: 10_000 })
  let writes = 0
  on('session.id', () => ({ value: 's1' }))
  on('session.cwd', () => ({ value: '/work/api' }))
  on('turn.complete', () => ({ text: '' }))
  on('prompt.edit', (_$, e) => ({ text: e.text + e.inputText, cursor: e.cursor + e.inputText.length }))
  // A primeira gravação (o turno respondeu) termina; as seguintes travam.
  on('store.set', () => (writes++ === 0 ? { value: undefined } : new Promise<never>(() => undefined)))

  await $.turn.complete({ answer: 'pronto', durationMs: 1, isAborted: false, turnId: 't1', reason: 'answer' })
  // O kit levanta o evento, mas a tipagem do substantivo `prompt` não o lista.
  const edited = await ($.prompt as unknown as { edit: (input: PromptEditInput) => Promise<unknown> }).edit({
    origin: { kind: 'composer' },
    text: 'o',
    cursor: 1,
    start: 1,
    end: 1,
    inputText: 'i',
  })

  expect(edited).toEqual({ text: 'oi', cursor: 2 })
  // A pessoa voltou à sessão: a gravação é pedida, só não é esperada.
  for (let turn = 0; turn < 200 && writes < 2; turn++) await Promise.resolve()
  expect(writes).toBe(2)
})
