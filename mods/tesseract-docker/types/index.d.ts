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

export type DockerPanel = {
  // Caminho do compose escolhido; vazio enquanto não há um.
  file: string
  services: Service[]
  // Falha da leitura dos serviços (docker compose ps).
  error: string
  // Falha da última ação ou do log; some na ação seguinte, quando o foco
  // anda, quando a leitura falha (o erro atual vence) ou depois de 15 s.
  failed: string
  isLoading: boolean
  // O que está rodando agora; vazio = painel ocioso.
  working: string
  // Serviço sob o foco; as ações por tecla agem nele.
  selected: string
  // Primeiro serviço visível: a lista rola com o foco.
  top: number
  // Serviço cujo log está aberto; vazio = lista.
  logOf: string
  log: string[]
}

declare module 'claude-code' {
  interface PluginState {
    'tesseract-docker': { panel: DockerPanel }
  }
}
