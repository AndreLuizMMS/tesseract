// tesseract-ide: conta ao engine do Tesseract onde o Claude mexeu por último.
// Não tem seletor nem lança IDE: quem faz isso é o mosaico (`ctrl+e`), que
// ganha a tecla `f` para abrir este arquivo, na linha.
//
// - Depois de cada Edit ou Write, um relato (`ts report`, como o
//   tesseract-link faz) leva o caminho e a linha onde a mudança começa.
// - Fora de uma célula, e numa sessão de fundo, o mod não faz nada.

import type { EngineInterface, Register } from 'claude-code'

// O engine responde em milissegundos; passar disso é engine travado, e a
// edição não espera por ele.
const REPORT_TIMEOUT_MS = 3000

// A linha (a partir de 1) onde `needle` começa em `text`; 1 se não achar.
export function lineOf(text: string, needle: string): number {
  const at = needle === '' ? -1 : text.indexOf(needle)

  return at < 0 ? 1 : text.slice(0, at).split('\n').length
}

// O binário que fala com o engine, se esta sessão é a que uma célula mostra.
// Uma sessão de fundo do Claude Code herda o ambiente da célula, mas não é o
// que ela mostra: fica muda.
async function engineOf($: EngineInterface): Promise<string | undefined> {
  if ((await $.env.get('TESSERACT')) !== '1' || !(await $.env.get('TESSERACT_CELL_ID'))) return undefined
  if ((await $.env.get('CLAUDE_CODE_SESSION_KIND')) === 'bg') return undefined

  return (await $.env.get('TESSERACT_BIN')) || 'ts'
}

// Relata o arquivo e a linha do que o Claude acabou de editar. Falhar é
// normal (engine reiniciando) e nunca derruba a edição.
async function report($: EngineInterface, filePath: unknown, written: string): Promise<void> {
  const engine = await engineOf($)
  if (engine === undefined || typeof filePath !== 'string' || filePath === '') return

  const file = filePath.startsWith('/') ? filePath : `${await $.session.cwd()}/${filePath}`
  // O começo do trecho novo; só as primeiras linhas, que bastam para achar.
  const opening = written.split('\n').slice(0, 3).join('\n')
  const line = opening === '' ? 1 : lineOf(await $.fs.read(file).catch(() => ''), opening)
  await $.process.run([engine, 'report', JSON.stringify({ kind: 'edit', file, line })], { timeoutMs: REPORT_TIMEOUT_MS })
}

export const register: Register = on => {
  // Depois de cada edição: só observa, nunca barra.
  on('tool.call', { tool: 'Edit' }, async ($, e, next) => {
    const ran = await next(e)
    if (ran.deny === undefined && ran.isError !== true) {
      await report($, e.file_path, typeof e.new_string === 'string' ? e.new_string : '').catch(() => undefined)
    }

    return ran
  }).catch(($, e, next) => next(e))

  on('tool.call', { tool: 'Write' }, async ($, e, next) => {
    const ran = await next(e)
    if (ran.deny === undefined && ran.isError !== true) await report($, e.file_path, '').catch(() => undefined)

    return ran
  }).catch(($, e, next) => next(e))
}
