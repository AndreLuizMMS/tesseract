export type MdList = {
  // Texto do filtro, como digitado.
  filter: string
  // Arquivo escolhido: fica sob o anel quando a lista muda por baixo.
  selected: string
  // Primeiro arquivo visível: a lista rola com o foco.
  top: number
  // Parada onde o anel de foco está: 0 = filtro, 1 + n = linha n da janela,
  // -1 = fora do painel. Vale também na leitura (o host guarda a posição).
  ring: number
  isScanning: boolean
}

export type MdReader = {
  // Caminho relativo à raiz do projeto; vazio = a lista está na tela.
  path: string
  text: string
  // O texto passou do que um desenho comporta e foi cortado.
  isCut: boolean
  mtimeMs: number
  error: string
  // Docs de onde se chegou aqui, para o "voltar".
  trail: string[]
  // Links do texto para outros markdown locais, com o estado de cada um.
  links: MdLink[]
  // Primeira linha de tela visível: a leitura rola por baixo do cabeçalho.
  offset: number
}

export type MdLink = {
  // Caminho relativo à raiz (ou o href, quando sai do projeto).
  path: string
  href: string
  // ok abre por tecla; os outros só aparecem, marcados.
  state: 'ok' | 'broken' | 'outside'
}

declare module 'claude-code' {
  interface PluginState {
    'tesseract-md': {
      files: string[]
      list: MdList
      reader: MdReader
      // Último `.md` que o Claude escreveu nesta sessão.
      lastWritten: string
    }
  }
}
