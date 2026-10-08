export type Ide = {
  // O nome do binário: code, cursor, codium…
  id: string
  label: string
  // Onde a instalação mora: dentro do WSL/Linux ou do lado Windows.
  location: 'wsl' | 'windows'
}

export type LastEdit = {
  // Caminho absoluto do arquivo.
  path: string
  line: number
}

export type IdePanel = {
  ides: Ide[]
  isLoading: boolean
  error: string
  // Parada onde o anel de foco está: a linha n da janela, ou -1 fora dela.
  // O IDE escolhido é o que está sob o anel.
  ring: number
  // Primeiro IDE visível: a lista rola com o foco.
  top: number
  // O que o Enter abre: o projeto, ou o último arquivo editado.
  wantsFile: boolean
}

declare module 'claude-code' {
  interface PluginState {
    'tesseract-ide': { panel: IdePanel; lastEdit: LastEdit | null }
  }
}
