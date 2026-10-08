import { expect, test } from 'claude-code/testing'

import type { Probe } from '../hooks/ides'
import { CANDIDATES, CMD_EXE, detectIdes, isRealInstall, isWindowsLauncher, launchOf, lineOf, parseDistro, windowsTarget } from '../hooks/ides'

const NOTHING: Probe = {
  which: async () => '',
  mtime: async () => undefined,
  users: [],
  where: async () => false,
}

const CODE = CANDIDATES[0]!

test('detecção: nada instalado, lista vazia', async () => {
  expect(await detectIdes(NOTHING)).toEqual([])
})

test('detecção: IDE do Windows fora do PATH é achado no disco', async () => {
  const exe = '/mnt/c/Users/ana/AppData/Local/Programs/Microsoft VS Code/Code.exe'
  const probe: Probe = { ...NOTHING, users: ['ana', 'Public'], mtime: async path => (path === exe ? 10 : undefined) }
  expect(await detectIdes(probe)).toEqual([{ id: 'code', label: 'VS Code', location: 'windows' }])
  expect(await windowsTarget(CODE, probe)).toBe(exe)
})

test('detecção: entre duas instalações, a mais recente', async () => {
  const user = '/mnt/c/Users/ana/AppData/Local/Programs/Microsoft VS Code/Code.exe'
  const machine = '/mnt/c/Program Files/Microsoft VS Code/Code.exe'
  const probe: Probe = { ...NOTHING, users: ['ana'], mtime: async path => ({ [user]: 5, [machine]: 9 })[path] }
  expect(await windowsTarget(CODE, probe)).toBe(machine)
})

test('detecção: só no PATH do Windows vale o nome; CLI remoto não é instalação', async () => {
  const probe: Probe = {
    ...NOTHING,
    which: async id => (id === 'code' ? '/home/ana/.vscode-server/bin/abc/bin/remote-cli/code' : id === 'cursor' ? '/usr/bin/cursor' : ''),
    where: async id => id === 'codium',
  }
  expect(await detectIdes(probe)).toEqual([
    { id: 'cursor', label: 'Cursor', location: 'wsl' },
    { id: 'codium', label: 'VSCodium', location: 'windows' },
  ])
  expect(await windowsTarget(CANDIDATES[2]!, probe)).toBe('codium')
  expect(isRealInstall('')).toBe(false)
})

test('detecção: mesmo IDE nos dois lados aparece duas vezes, ordenado', async () => {
  const probe: Probe = { ...NOTHING, which: async id => (id === 'code' ? '/usr/bin/code' : ''), where: async id => id === 'code' }
  expect((await detectIdes(probe)).map(ide => `${ide.id}@${ide.location}`)).toEqual(['code@windows', 'code@wsl'])
})

test('detecção: o lançador do Windows no PATH do WSL não duplica a instalação', async () => {
  const exe = '/mnt/c/Users/ana/AppData/Local/Programs/Microsoft VS Code/Code.exe'
  const launcher = '/mnt/c/Users/ana/AppData/Local/Programs/Microsoft VS Code/bin/code'
  const probe: Probe = {
    ...NOTHING,
    users: ['ana'],
    which: async id => (id === 'code' ? launcher : ''),
    mtime: async path => (path === exe ? 10 : undefined),
  }
  expect(await detectIdes(probe)).toEqual([{ id: 'code', label: 'VS Code', location: 'windows' }])
  expect(isWindowsLauncher(launcher)).toBe(true)
  expect(isWindowsLauncher('/usr/bin/code')).toBe(false)

  // Sem o .exe achado do lado Windows, o lançador é o único caminho: fica.
  const alone: Probe = { ...NOTHING, which: async id => (id === 'code' ? launcher : '') }
  expect(await detectIdes(alone)).toEqual([{ id: 'code', label: 'VS Code', location: 'wsl' }])
})

test('distro: wsl.exe responde em UTF-16', () => {
  expect(parseDistro('U\u0000b\u0000u\u0000n\u0000t\u0000u\u0000\r\u0000\n\u0000D\u0000e\u0000b\u0000')).toBe('Ubuntu')
  expect(parseDistro('\n\n')).toBe('')
})

test('linha do trecho editado', () => {
  expect(lineOf('a\nb\nc\n', 'c')).toBe(3)
  expect(lineOf('a\nb\nc\n', 'a')).toBe(1)
  expect(lineOf('a\nb', 'zzz')).toBe(1)
  expect(lineOf('a\nb', '')).toBe(1)
})

test('comando: projeto e arquivo na linha, por cada caminho', () => {
  const project = { project: '/home/ana/api' }
  const file = { project: '/home/ana/api', file: { path: '/home/ana/api/main.go', line: 42 } }

  expect(launchOf('code', { kind: 'wsl' }, project).argv).toEqual(['code', '/home/ana/api'])
  expect(launchOf('code', { kind: 'wsl' }, file).argv).toEqual(['code', '/home/ana/api', '--goto', '/home/ana/api/main.go:42'])
  expect(launchOf('code', { kind: 'wsl' }, project).env.DONT_PROMPT_WSL_INSTALL).toBe('1')

  const window = launchOf('code', { kind: 'window', cli: '/cli/code', socket: '/run/x.sock' }, file)
  expect(window.argv[0]).toBe('/cli/code')
  expect(window.env.VSCODE_IPC_HOOK_CLI).toBe('/run/x.sock')

  expect(launchOf('code', { kind: 'exe', exe: '/mnt/c/Code.exe', distro: 'Ubuntu' }, project).argv).toEqual([
    '/mnt/c/Code.exe', '--folder-uri', 'vscode-remote://wsl+Ubuntu/home/ana/api',
  ])
  expect(launchOf('code', { kind: 'exe', exe: '/mnt/c/Code.exe', distro: 'Ubuntu' }, file).argv).toEqual([
    '/mnt/c/Code.exe', '--remote', 'wsl+Ubuntu', '/home/ana/api', '--goto', '/home/ana/api/main.go:42',
  ])
  expect(launchOf('codium', { kind: 'path', distro: 'Ubuntu' }, project).argv.slice(0, 5)).toEqual([CMD_EXE, '/c', 'start', '', 'codium'])
})
