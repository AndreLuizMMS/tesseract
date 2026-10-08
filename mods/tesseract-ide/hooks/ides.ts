// Detecção dos IDEs e montagem do comando que abre um. Porte de
// internal/engine/actions.go (DetectIDEs, OpenInEditor).

import type { Ide } from '../types'

export type Candidate = {
  id: string
  label: string
  windowsExe: string
  // Pasta em ~ onde o IDE Windows instala seu servidor remoto no WSL.
  serverDir: string
}

// Instalador do Windows não se põe no PATH de forma confiável: o binário é
// procurado no disco, e o PATH é só um sinal a mais.
export const CANDIDATES: readonly Candidate[] = [
  { id: 'code', label: 'VS Code', windowsExe: 'Microsoft VS Code/Code.exe', serverDir: '.vscode-server' },
  { id: 'code-insiders', label: 'VS Code Insiders', windowsExe: 'Microsoft VS Code Insiders/Code - Insiders.exe', serverDir: '.vscode-server-insiders' },
  { id: 'codium', label: 'VSCodium', windowsExe: 'VSCodium/VSCodium.exe', serverDir: '.vscodium-server' },
  { id: 'cursor', label: 'Cursor', windowsExe: 'cursor/Cursor.exe', serverDir: '.cursor-server' },
]

export const CMD_EXE = '/mnt/c/Windows/System32/cmd.exe'

// O que a detecção precisa perguntar à máquina; injetado para dar teste.
export type Probe = {
  // Caminho real (links resolvidos) do binário no PATH do Linux; '' quando não há.
  which: (id: string) => Promise<string>
  // Data de modificação do caminho; undefined quando não existe.
  mtime: (path: string) => Promise<number | undefined>
  // Pastas em /mnt/c/Users; vazio fora do WSL.
  users: readonly string[]
  // Se o PATH do Windows resolve o nome (via cmd.exe); false fora do WSL.
  where: (id: string) => Promise<boolean>
}

// Onde o Windows guarda programas: por usuário e os dois Program Files.
export const windowsRoots = (users: readonly string[]): string[] => [
  ...users.map(user => `/mnt/c/Users/${user}/AppData/Local/Programs`),
  '/mnt/c/Program Files',
  '/mnt/c/Program Files (x86)',
]

// O CLI remoto que um IDE deixa em ~/.<nome>-server só fala com uma janela
// que outro abriu: não conta como instalação.
export const isRealInstall = (binary: string): boolean => binary !== '' && !binary.includes('-server/')

// O binário mora num disco do Windows (/mnt/c/...): é o lançador que a
// instalação Windows põe no PATH do WSL, não uma instalação Linux.
export const isWindowsLauncher = (binary: string): boolean => /^\/mnt\/[a-z]\//i.test(binary)

// Como rodar o IDE do lado Windows: o caminho do .exe achado no disco (o
// mais recente), o próprio id quando só o PATH do Windows o resolve, ou ''.
export async function windowsTarget(candidate: Candidate, probe: Probe): Promise<string> {
  const found = await Promise.all(
    windowsRoots(probe.users).map(async root => {
      const path = `${root}/${candidate.windowsExe}`

      return { path, mtimeMs: await probe.mtime(path) }
    }),
  )
  const newest = found
    .filter(one => one.mtimeMs !== undefined)
    .sort((a, b) => (b.mtimeMs ?? 0) - (a.mtimeMs ?? 0))[0]
  if (newest !== undefined) return newest.path

  return (await probe.where(candidate.id)) ? candidate.id : ''
}

// Quais IDEs conhecidos são alcançáveis daqui, no WSL e no Windows.
export async function detectIdes(probe: Probe): Promise<Ide[]> {
  const found = await Promise.all(
    CANDIDATES.map(async candidate => {
      const [binary, target] = await Promise.all([probe.which(candidate.id), windowsTarget(candidate, probe)])
      const here: Ide[] = []
      // O lançador do Windows no PATH e o .exe achado no disco são a mesma
      // instalação: aparece uma vez, como Windows.
      const isSameInstall = isWindowsLauncher(binary) && target !== ''
      if (isRealInstall(binary) && !isSameInstall) here.push({ id: candidate.id, label: candidate.label, location: 'wsl' })
      if (target !== '') here.push({ id: candidate.id, label: candidate.label, location: 'windows' })

      return here
    }),
  )

  return found.flat().sort((a, b) => a.label.localeCompare(b.label) || a.location.localeCompare(b.location))
}

// O nome da distro como `wsl.exe -l -q` devolve: UTF-16, então cheio de NUL.
export function parseDistro(output: string): string {
  return (
    output
      .split('\n')
      .map(line => line.replace(/\u0000/g, '').trim())
      .find(line => line !== '') ?? ''
  )
}

// A linha (a partir de 1) onde `needle` começa em `text`; 1 se não achar.
export function lineOf(text: string, needle: string): number {
  const at = needle === '' ? -1 : text.indexOf(needle)

  return at < 0 ? 1 : text.slice(0, at).split('\n').length
}

export type Target = { project: string; file?: { path: string; line: number } }

export type Way =
  // Binário Linux no PATH.
  | { kind: 'wsl' }
  // Janela do IDE Windows já aberta: fala-se com ela pelo CLI remoto.
  | { kind: 'window'; cli: string; socket: string }
  // O .exe achado no disco, rodado direto pelo WSL.
  | { kind: 'exe'; exe: string; distro: string }
  // Só o PATH do Windows conhece o nome: cmd.exe resolve.
  | { kind: 'path'; distro: string }

export type Launch = { argv: string[]; env: Record<string, string> }

// O comando que abre o alvo no IDE, conforme o caminho disponível.
export function launchOf(id: string, way: Way, target: Target): Launch {
  const goto = target.file === undefined ? [] : ['--goto', `${target.file.path}:${target.file.line}`]
  // O lançador `code` no WSL pergunta no stdin se é para abrir a instalação
  // Linux; sem stdin ele sai sem abrir nada. Esta variável cala a pergunta.
  const env: Record<string, string> = { DONT_PROMPT_WSL_INSTALL: '1' }

  if (way.kind === 'wsl') return { argv: [id, target.project, ...goto], env }
  if (way.kind === 'window') {
    return { argv: [way.cli, target.project, ...goto], env: { ...env, VSCODE_IPC_HOOK_CLI: way.socket } }
  }

  const remote = `wsl+${way.distro}`
  const args =
    target.file === undefined
      ? ['--folder-uri', `vscode-remote://${remote}${target.project}`]
      : ['--remote', remote, target.project, ...goto]

  return way.kind === 'exe'
    ? { argv: [way.exe, ...args], env }
    : { argv: [CMD_EXE, '/c', 'start', '', id, ...args], env }
}

// Lança destacado e devolve logo: sessão própria, saída fora dos pipes da
// chamada. Quem abre janela fica vivo; quem não abriu morre na hora, e meio
// segundo separa os dois: é o que traz o erro à tela.
export const DETACH_SCRIPT = [
  'log=$(mktemp) || exit 1',
  'setsid "$@" >"$log" 2>&1 </dev/null &',
  'pid=$!',
  'sleep 0.5',
  'if kill -0 "$pid" 2>/dev/null; then rm -f "$log"; exit 0; fi',
  'wait "$pid"; code=$?',
  'head -n 5 "$log"; rm -f "$log"',
  'exit $code',
].join('\n')
