// Contrato dos valores que o mod guarda em $.state.

export type TurnPhase = 'idle' | 'working' | 'responded' | 'interrupted' | 'failed'

export type Turn = {
  phase: TurnPhase
  // Início do turno em ms ($.clock.now), enquanto ele roda.
  since?: number
}

export type Quota = {
  // Janela de 5 h da assinatura, 0 a 100.
  fiveHour?: number
  // Preenchimento da janela de contexto, 0 a 100.
  context?: number
}

declare module 'claude-code' {
  interface PluginState {
    'tesseract-skin': {
      turn: Turn
      now: number
      quota: Quota
      // Chamadas (tool_use_id) com o diálogo de permissão aberto, em ordem.
      asking: readonly string[]
      // Quadro escuro do pisca da barra de approve.
      isDimmed: boolean
    }
  }
}
