// tesseract-md: avisa quando o Claude escreve um markdown. Não tem painel:
// quem lê é a aba md da célula, no mosaico do Tesseract.
//
// - Dentro de uma célula, cada `.md` escrito ou editado vira um relato ao
//   engine (`ts report`, como o tesseract-link faz): o rodapé do mosaico
//   passa a oferecer a tecla que abre o arquivo no leitor.
// - Fora de célula não há engine: um aviso curto diz o caminho.

import type { EngineInterface, Register } from 'claude-code'

// O engine responde em milissegundos; passar disso é engine travado, e a
// edição não espera por ele.
const REPORT_TIMEOUT_MS = 3000

export const isMarkdown = (path: string): boolean => /\.(md|markdown)$/i.test(path)

// O binário que fala com o engine, se esta sessão é a que uma célula
// mostra; `undefined` fora do Tesseract.
//
// Uma sessão de fundo do Claude Code nasce do processo da célula e herda o
// ambiente dele, mas não é o que a célula mostra: devolve `null` e fica muda.
async function engineOf($: EngineInterface): Promise<string | null | undefined> {
  if ((await $.env.get('TESSERACT')) !== '1' || !(await $.env.get('TESSERACT_CELL_ID'))) return undefined
  if ((await $.env.get('CLAUDE_CODE_SESSION_KIND')) === 'bg') return null

  return (await $.env.get('TESSERACT_BIN')) || 'ts'
}

// O último arquivo avisado fora de célula: reescrever o mesmo não repete o
// aviso. Some num reload, e tudo bem.
let announced = ''

async function announce($: EngineInterface, filePath: unknown): Promise<void> {
  if (typeof filePath !== 'string' || !isMarkdown(filePath)) return

  const file = filePath.startsWith('/') ? filePath : `${await $.session.cwd()}/${filePath}`
  const engine = await engineOf($)
  if (engine === null) return
  if (engine !== undefined) {
    // Falhar é normal (engine reiniciando) e nunca derruba a edição: sem o
    // relato, o arquivo continua na lista da aba md.
    await $.process.run([engine, 'report', JSON.stringify({ kind: 'doc', file })], { timeoutMs: REPORT_TIMEOUT_MS })

    return
  }
  if (file === announced) return

  announced = file
  const root = await $.session.root()
  $.ui.toast(`Wrote ${file.startsWith(`${root}/`) ? file.slice(root.length + 1) : file}.`)
}

export const register: Register = on => {
  // Depois que o Claude escreve um markdown: só observa, nunca barra.
  for (const tool of ['Write', 'Edit'] as const) {
    on('tool.call', { tool }, async ($, e, next) => {
      const ran = await next(e)
      if (ran.deny === undefined && ran.isError !== true) await announce($, e.file_path).catch(() => undefined)

      return ran
    }).catch(($, e, next) => next(e))
  }
}
