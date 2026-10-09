// Guarda contra Docker destrutivo em comandos de shell.
//
// Mesma regra do painel do mosaico (internal/docker/docker.go): nada que
// apague volume entra. Aqui a regra vale para o que o Claude manda pelo Bash.
//
// O comando é quebrado em comandos simples (argv) e só o argv é julgado:
// `echo "docker compose down -v"` e `grep 'system prune'` passam, porque o
// programa ali é echo/grep, não docker.
//
// Fail-closed em dois pontos:
// - texto entregue a um shell (`bash -c`, `eval`, `| sh`, `<<<`, heredoc,
//   `watch`, `script -c`, `find -exec`, `xargs`) é lido como outro script;
// - `docker`/`compose` com subcomando perigoso e um argumento que vem de
//   expansão (`$VAR`, `$(...)`, crase, chaves) é negado: a guarda não sabe o
//   que a expansão vira, e `-v` ali apagaria volume.

export type Verdict = {
  // O comando simples que foi negado, como argv unido por espaço.
  command: string
  // Por que é destrutivo, em uma frase.
  reason: string
  // O que usar no lugar, em uma frase.
  instead: string
}

// Onde havia uma expansão que a guarda não resolve.
const HOLE = '\u0000'

type Word = {
  // O texto literal; cada expansão vira um HOLE.
  text: string
  // A expansão pode virar qualquer coisa, inclusive uma flag: está fora de
  // aspas, ou a palavra começa por ela, ou por `-`.
  open: boolean
}

type Command = {
  words: Word[]
  // O que chega pela entrada padrão: corpo de heredoc e here-string.
  feeds: string[]
  // Recebe a saída do comando anterior por pipe.
  afterPipe: boolean
  // Recebe um `<(...)`.
  substituted: boolean
}

type Parsed = {
  commands: Command[]
  // Trechos que são outro script: substituição de comando.
  nested: string[]
}

type Why = { reason: string; instead: string }

const SHELLS = new Set(['sh', 'bash', 'zsh', 'dash', 'ksh', 'ash', 'fish'])
// Programas cujo `-c <texto>` é um script.
const TAKES_SCRIPT = new Set([...SHELLS, 'su', 'sg'])
const DOCKERS = new Set(['docker', 'docker-compose', 'podman', 'podman-compose'])

// Programas que só mostram ou procuram texto: um `docker` no meio do argv
// deles é assunto, não comando.
const INERT = new Set([
  'echo', 'printf', 'man', 'tldr', 'info', 'help', 'which', 'type', 'whereis', 'whatis',
  'apropos', 'grep', 'egrep', 'fgrep', 'rg', 'ag', 'git', 'apt', 'apt-get', 'brew',
  'dnf', 'yum', 'pacman', 'apk', 'snap',
])

// Programas que só repassam o resto do argv a outro programa. O valor lista
// as flags que consomem o argumento seguinte.
const WRAPPERS: Record<string, readonly string[]> = {
  sudo: ['-u', '-g', '-h', '-p', '-C', '-D', '-R', '-T', '-U'],
  doas: ['-u', '-C'],
  env: ['-u', '-C'],
  xargs: ['-I', '-n', '-P', '-L', '-d', '-E', '-s', '-a'],
  timeout: ['-s', '-k'],
  nice: ['-n'],
  ionice: ['-c', '-n', '-p'],
  stdbuf: ['-i', '-o', '-e'],
  command: [],
  exec: ['-a'],
  builtin: [],
  time: [],
  nohup: [],
  setsid: [],
}

// Palavras reservadas que podem anteceder um comando.
const KEYWORDS = new Set(['then', 'do', 'else', 'elif', 'if', 'while', 'until', '!', '{', '}'])

const DOCKER_VALUE_FLAGS = new Set([
  '-H', '--host', '-c', '--context', '--config', '-l', '--log-level',
  '--tlscacert', '--tlscert', '--tlskey',
  // podman
  '--root', '--runroot', '--connection', '--url', '--identity', '--storage-driver',
  '--storage-opt', '--tmpdir', '--cgroup-manager', '--runtime', '--module',
])

const COMPOSE_VALUE_FLAGS = new Set([
  '-f', '--file', '-p', '--project-name', '--profile', '--env-file',
  '--project-directory', '--ansi', '--progress', '--parallel',
])

// Os valores que o Docker lê como "desligado" em `--flag=valor`.
const FALSY = new Set(['false', 'f', '0'])

const ASK_USER = 'If the user really wants it gone, ask them to run it themselves in a shell.'

const OPEN_ARGS: Why = {
  reason:
    'its arguments come from an expansion ($VAR, $(...), backticks, braces or xargs input) the guard cannot read, and a -v there would delete volumes',
  instead: 'Write the flags and names out literally and run it again.',
}
const OPEN_SUB: Why = {
  reason: 'its subcommand comes from an expansion ($VAR, $(...), backticks) the guard cannot read',
  instead: 'Write the subcommand out literally and run it again.',
}

const baseName = (path: string): string =>
  (path.split('/').pop() ?? path).replace(/\.exe$/i, '')

// O nome do programa de uma palavra; vazio quando ela tem expansão.
const nameOf = (word: Word | undefined): string =>
  word === undefined || word.text.includes(HOLE) ? '' : baseName(word.text)

// Como a palavra aparece numa mensagem e num script relido: a expansão volta
// a ser uma variável, para continuar sendo expansão na releitura.
const shown = (word: Word): string => word.text.replaceAll(HOLE, '$_')

const join = (words: readonly Word[]): string => words.map(shown).join(' ')

// O comando como a recusa o cita: a expansão aparece como `$…`.
const cited = (words: readonly Word[]): string =>
  words.map(word => word.text.replaceAll(HOLE, '$…')).join(' ')

// Lê até o fecho correspondente e devolve o miolo; `from` aponta o abre.
function balanced(script: string, from: number): [string, number] {
  let depth = 0
  for (let j = from; j < script.length; j++) {
    if (script[j] === '(') depth++
    if (script[j] === ')' && --depth === 0) return [script.slice(from + 1, j), j]
  }

  return [script.slice(from + 1), script.length]
}

// As substituições de comando de um texto que o shell expande (heredoc sem
// aspas no delimitador): `$(...)` e crases.
function substitutions(text: string): string[] {
  const found: string[] = []
  for (let i = 0; i < text.length; i++) {
    if (text[i] === '\\') i++
    else if (text[i] === '$' && text[i + 1] === '(') {
      const [inner, close] = balanced(text, i + 1)
      found.push(inner)
      i = close
    } else if (text[i] === '`') {
      const close = text.indexOf('`', i + 1)
      const end = close < 0 ? text.length : close
      found.push(text.slice(i + 1, end))
      i = end
    }
  }

  return found
}

const ANSI_C: Record<string, string> = {
  n: '\n', t: '\t', r: '\r', a: '\u0007', b: '\b', e: '\u001b', f: '\f', v: '\u000b',
}

// Lê `$'...'` a partir da aspa de abertura; devolve o texto e onde fecha.
function ansiC(script: string, from: number): [string, number] {
  let text = ''
  let i = from + 1
  for (; i < script.length && script[i] !== "'"; i++) {
    if (script[i] !== '\\') {
      text += script[i]
      continue
    }
    const rest = script.slice(i + 1)
    const coded = /^(x[0-9a-fA-F]{1,2}|u[0-9a-fA-F]{1,4}|[0-7]{1,3})/.exec(rest)?.[0]
    if (coded !== undefined) {
      const isOctal = /^[0-7]/.test(coded)
      text += String.fromCharCode(parseInt(isOctal ? coded : coded.slice(1), isOctal ? 8 : 16))
      i += coded.length
    } else {
      const char = rest[0] ?? ''
      text += ANSI_C[char] ?? char
      i++
    }
  }

  return [text, i]
}

// Quebra um script de shell em comandos simples. Não expande nada: cada
// expansão vira um HOLE na palavra, e quem julga decide o que fazer com ele.
function parse(script: string): Parsed {
  const commands: Command[] = []
  const nested: string[] = []
  const fresh = (): Command => ({ words: [], feeds: [], afterPipe: false, substituted: false })
  let current = fresh()
  let word = ''
  let hasWord = false
  // A palavra tem expansão fora de aspas.
  let hasBareHole = false
  // O que a próxima palavra é, quando não é argumento.
  let role: 'arg' | 'redirect' | 'herestring' = 'arg'
  // Heredocs pendentes na linha corrente, cada um com o comando que o recebe.
  let heredocs: { delimiter: string; isQuoted: boolean; owner: Command }[] = []

  const endWord = (): void => {
    if (!hasWord) return
    const hasHole = word.includes(HOLE)
    const open = hasHole && (hasBareHole || word.startsWith(HOLE) || word.startsWith('-'))
    if (role === 'herestring') current.feeds.push(word.replaceAll(HOLE, '$_'))
    else if (role === 'arg') current.words.push({ text: word, open })
    role = 'arg'
    word = ''
    hasWord = false
    hasBareHole = false
  }
  const endCommand = (): void => {
    endWord()
    if (current.words.length === 0 && current.feeds.length === 0) return
    commands.push(current)
    current = fresh()
  }
  // Consome uma expansão que começa em `$` ou crase, na posição `i`; devolve
  // onde ela acaba, ou -1 se ali não há expansão.
  const expansion = (i: number): number => {
    if (script[i] === '`') {
      const close = script.indexOf('`', i + 1)
      const end = close < 0 ? script.length : close
      nested.push(script.slice(i + 1, end))

      return end
    }
    const following = script[i + 1]
    if (following === '(') {
      const [inner, close] = balanced(script, i + 1)
      nested.push(inner)

      return close
    }
    if (following === '{') {
      const close = script.indexOf('}', i + 1)

      return close < 0 ? script.length : close
    }
    const named = /^[A-Za-z_]\w*|^[@*#?$!0-9-]/.exec(script.slice(i + 1))?.[0]

    return named === undefined ? -1 : i + named.length
  }

  for (let i = 0; i < script.length; i++) {
    const char = script[i] as string
    const following = script[i + 1]

    if (char === '\\') {
      if (following === '\n') i++ // continuação de linha
      else if (following !== undefined) {
        word += following
        hasWord = true
        i++
      }
      continue
    }
    if (char === "'") {
      const close = script.indexOf("'", i + 1)
      const end = close < 0 ? script.length : close
      word += script.slice(i + 1, end)
      hasWord = true
      i = end
      continue
    }
    if (char === '$' && following === "'") {
      const [text, close] = ansiC(script, i + 1)
      word += text
      hasWord = true
      i = close
      continue
    }
    // `$"..."` é só uma aspa dupla traduzível.
    if (char === '$' && following === '"') continue
    if (char === '"') {
      hasWord = true
      for (i++; i < script.length && script[i] !== '"'; i++) {
        if (script[i] === '\\' && i + 1 < script.length) {
          word += script[++i]
          continue
        }
        const end = script[i] === '$' || script[i] === '`' ? expansion(i) : -1
        if (end < 0) word += script[i]
        else {
          word += HOLE
          i = end
        }
      }
      continue
    }
    if (char === '$' || char === '`') {
      const end = expansion(i)
      hasWord = true
      if (end < 0) word += char
      else {
        word += HOLE
        hasBareHole = true
        i = end
      }
      continue
    }
    // Expansão de chaves (`-{v,x}`, `{1..3}`); `{}` sozinho é literal.
    if (char === '{') {
      const braces = /^\{[^{}\s]*(,|\.\.)[^{}\s]*\}/.exec(script.slice(i))?.[0]
      if (braces !== undefined) {
        word += HOLE
        hasWord = true
        hasBareHole = true
        i += braces.length - 1
        continue
      }
    }
    if (char === '#' && !hasWord) {
      const close = script.indexOf('\n', i)
      i = close < 0 ? script.length : close - 1
      continue
    }
    if (char === '<' || char === '>') {
      // Um número colado antes do operador é o descritor (`2>`), não argumento.
      if (hasWord && /^\d+$/.test(word)) {
        word = ''
        hasWord = false
      }
      endWord()
      if (char === '<' && following === '<' && script[i + 2] === '<') {
        role = 'herestring'
        i += 2
        continue
      }
      if (char === '<' && following === '<') {
        // Heredoc: guarda o delimitador, sem aspas e sem o `-` de recuo.
        const found = /^<<-?\s*(['"\\]?)([A-Za-z_][\w.-]*)['"]?/.exec(script.slice(i))
        if (found !== null) {
          heredocs.push({ delimiter: found[2] as string, isQuoted: found[1] !== '', owner: current })
          i += found[0].length - 1
          continue
        }
      }
      while (script[i + 1] === '<' || script[i + 1] === '>' || script[i + 1] === '&') i++
      if (char === '>' && script[i + 1] === '|') i++ // `>|`
      // `<(...)` e `>(...)` são comandos: o parêntese separa adiante.
      if (script[i + 1] !== '(') role = 'redirect'
      else if (char === '<') current.substituted = true
      continue
    }
    if (char === '\n') {
      endCommand()
      for (const heredoc of heredocs) {
        const rest = script.slice(i + 1)
        const close = new RegExp(`^[ \\t]*${heredoc.delimiter}[ \\t]*$`, 'm').exec(rest)
        const body = close === null ? rest : rest.slice(0, close.index)
        heredoc.owner.feeds.push(body)
        // Sem aspas no delimitador, o shell expande o corpo antes de entregar.
        if (!heredoc.isQuoted) nested.push(...substitutions(body))
        // Um heredoc sem comando (`<<EOF` solto) ainda conta como comando.
        if (!commands.includes(heredoc.owner) && heredoc.owner !== current) commands.push(heredoc.owner)
        i += close === null ? rest.length : close.index + close[0].length
      }
      heredocs = []
      continue
    }
    if (char === '|' && following !== '|') {
      endCommand()
      if (following === '&') i++
      current.afterPipe = true
      continue
    }
    if (char === ';' || char === '&' || char === '|') {
      endCommand()
      if (following === char) i++
      current.afterPipe = false
      continue
    }
    if (char === '(' || char === ')') {
      endCommand()
      continue
    }
    if (char === ' ' || char === '\t') {
      endWord()
      continue
    }
    word += char
    hasWord = true
  }
  endCommand()

  return { commands, nested }
}

type Unwrapped = {
  rest: Word[]
  // O argv ganha argumentos da entrada padrão (xargs): ninguém sabe quais.
  isFedByStdin: boolean
  // Texto que o repassador lê como comando (`env -S`).
  scripts: string[]
}

// Tira do começo do argv atribuições, palavras reservadas e repassadores.
function unwrap(words: readonly Word[]): Unwrapped {
  let rest = [...words]
  let isFedByStdin = false
  const scripts: string[] = []
  for (;;) {
    const head = rest[0]
    if (head === undefined) break
    if (/^[A-Za-z_]\w*=/.test(head.text) || KEYWORDS.has(head.text)) {
      rest = rest.slice(1)
      continue
    }
    const wrapper = nameOf(head)
    const valueFlags = WRAPPERS[wrapper]
    if (valueFlags === undefined) break
    if (wrapper === 'xargs') isFedByStdin = true
    rest = rest.slice(1)
    while (rest[0] !== undefined) {
      const flag = rest[0].text
      if (wrapper === 'env' && (flag === '-S' || flag === '--split-string')) {
        if (rest[1] !== undefined) scripts.push(shown(rest[1]))
        rest = rest.slice(2)
      } else if (wrapper === 'env' && flag.startsWith('--split-string=')) {
        scripts.push(shown({ ...rest[0], text: flag.slice('--split-string='.length) }))
        rest = rest.slice(1)
      } else if (valueFlags.includes(flag)) rest = rest.slice(2)
      else if (flag.startsWith('-') || /^[A-Za-z_]\w*=/.test(flag) || /^\d+(\.\d+)?[smhd]?$/.test(flag)) {
        rest = rest.slice(1)
      } else break
    }
  }

  return { rest, isFedByStdin, scripts }
}

// Tira as opções globais e devolve [subcomando, argumentos dele].
function splitSubcommand(
  args: readonly Word[],
  valueFlags: ReadonlySet<string>,
): [Word | undefined, Word[]] {
  let i = 0
  while (i < args.length && (args[i] as Word).text.startsWith('-')) {
    i += valueFlags.has((args[i] as Word).text) ? 2 : 1
  }

  return [args[i], args.slice(i + 1)]
}

// Os argumentos até o `--`: depois dele nada é flag.
const beforeDashes = (args: readonly Word[]): Word[] => {
  const end = args.findIndex(arg => arg.text === '--')

  return end < 0 ? [...args] : args.slice(0, end)
}

// Diz se a flag curta (em grupo, `-fsv`, ou `-v=true`) ou a longa aparece.
function hasFlag(args: readonly Word[], short: string, long: string): boolean {
  for (const arg of beforeDashes(args)) {
    const equals = arg.text.indexOf('=')
    const name = equals < 0 ? arg.text : arg.text.slice(0, equals)
    if (equals >= 0 && FALSY.has(arg.text.slice(equals + 1).toLowerCase())) continue
    if (name === `--${long}`) return true
    if (/^-[A-Za-z]+$/.test(name) && name.includes(short)) return true
  }

  return false
}

const hasOpen = (args: readonly Word[]): boolean => beforeDashes(args).some(arg => arg.open)

const PRUNES: Record<string, Why> = {
  container: {
    reason: 'it deletes every stopped container on this machine, other projects included',
    instead: 'Use `docker rm <name>` for one container you name.',
  },
  image: {
    reason: 'it deletes unused images of every project on this machine',
    instead: 'Use `docker image ls`, then `docker rmi <image>` for one you name.',
  },
  builder: {
    reason: 'it deletes the build cache of every project on this machine',
    instead: 'Use `docker builder du` to see what the cache holds, and leave the pruning to the user.',
  },
  buildx: {
    reason: 'it deletes the build cache of every project on this machine',
    instead: 'Use `docker buildx du` to see what the cache holds, and leave the pruning to the user.',
  },
  network: {
    reason: 'it deletes unused networks of every project on this machine',
    instead: 'Use `docker network rm <name>` for one network you name.',
  },
}

const VOLUME: Why = {
  reason: 'it deletes volumes (data loss)',
  instead:
    'Nothing replaces deleting a volume: `docker volume ls` and `docker volume inspect <name>` show what is there.',
}
const ANONYMOUS: Why = {
  reason: 'it deletes the anonymous volumes of the removed containers',
  instead: 'Use the same command without -v.',
}

// `isStrict`: o programa está na cabeça do argv, então um subcomando que vem
// de expansão também é negado. No meio do argv (`usermod -aG docker $USER`)
// só o que é perigoso com certeza conta.
function composeWhy(args: readonly Word[], isStrict: boolean): Why | undefined {
  const [sub, rest] = splitSubcommand(args, COMPOSE_VALUE_FLAGS)
  if (sub === undefined) return undefined
  if (sub.text.includes(HOLE)) return isStrict ? OPEN_SUB : undefined

  if (sub.text === 'down') {
    if (hasFlag(rest, 'v', 'volumes')) {
      return {
        reason: 'it deletes the stack volumes (data loss)',
        instead: 'Use `docker compose stop`, or `docker compose down` without -v, which keeps the volumes.',
      }
    }
    if (rest.some(arg => arg.text === '--rmi' || arg.text.startsWith('--rmi='))) {
      return { reason: 'it deletes the stack images', instead: 'Use `docker compose down` without --rmi.' }
    }
  }
  if (sub.text === 'rm' && hasFlag(rest, 'v', 'volumes')) {
    return { ...ANONYMOUS, instead: 'Use `docker compose rm` without -v.' }
  }
  if (sub.text === 'up' && hasFlag(rest, 'V', 'renew-anon-volumes')) {
    return {
      reason: 'it recreates the anonymous volumes, dropping the data in them',
      instead: 'Use `docker compose up -d`; `--force-recreate` recreates the containers and keeps the volumes.',
    }
  }
  if ((sub.text === 'down' || sub.text === 'rm') && hasOpen(rest)) return OPEN_ARGS

  return undefined
}

function dockerWhy(args: readonly Word[], isPodman: boolean, isStrict: boolean): Why | undefined {
  const [sub, rest] = splitSubcommand(args, DOCKER_VALUE_FLAGS)
  if (sub === undefined) return undefined
  if (sub.text.includes(HOLE)) return isStrict ? OPEN_SUB : undefined
  if (sub.text === 'compose') return composeWhy(rest, isStrict)

  const [verb, verbArgs] = splitSubcommand(rest, new Set())
  const isVerbOpen = verb?.text.includes(HOLE) === true
  if (sub.text === 'system') {
    if (verb?.text === 'prune') {
      return {
        reason: 'it deletes every stopped container, unused network and image on this machine',
        instead:
          'Use `docker system df` to see what takes the space; `docker compose down` removes only this project\'s containers and networks.',
      }
    }
    if (isPodman && verb?.text === 'reset') {
      return { reason: 'it wipes all podman storage: containers, images and volumes', instead: 'Use `podman system df` to see what takes the space.' }
    }
    if (isVerbOpen) return OPEN_SUB
  }
  if (sub.text === 'volume') {
    if (verb?.text === 'rm' || verb?.text === 'remove' || verb?.text === 'prune') return VOLUME
    if (isVerbOpen) return OPEN_SUB
  }
  const prune = PRUNES[sub.text]
  if (prune !== undefined) {
    if (verb?.text === 'prune') return prune
    if (isVerbOpen) return OPEN_SUB
  }
  const removed = sub.text === 'rm' ? rest : sub.text === 'container' && verb?.text === 'rm' ? verbArgs : undefined
  if (removed !== undefined) {
    if (hasFlag(removed, 'v', 'volumes')) return ANONYMOUS
    if (hasOpen(removed)) return OPEN_ARGS
  }

  return undefined
}

// O argumento de `-c` / `--command` de quem roda um texto como script.
function scriptOf(args: readonly Word[]): string | undefined {
  for (let i = 0; i < args.length; i++) {
    const text = (args[i] as Word).text
    if (text.startsWith('--command=')) return shown({ ...(args[i] as Word), text: text.slice(10) })
    if (text === '--command' || /^-[A-Za-z]*c[A-Za-z]*$/.test(text)) {
      return args[i + 1] === undefined ? undefined : shown(args[i + 1] as Word)
    }
  }

  return undefined
}

// Os argumentos de `watch` depois das flags dele: o comando que ele repete.
function watched(args: readonly Word[]): string {
  let i = 0
  while (i < args.length && (args[i] as Word).text.startsWith('-')) {
    i += ['-n', '--interval', '-q', '--equexit'].includes((args[i] as Word).text) ? 2 : 1
  }

  return join(args.slice(i))
}

// Julga um comando simples; o que nele é outro script entra em `nested`.
function judge(command: Command, nested: string[]): Verdict | undefined {
  const { rest, isFedByStdin, scripts } = unwrap(command.words)
  nested.push(...scripts)
  const head = rest[0]
  if (head === undefined) return undefined

  const program = nameOf(head)
  if (INERT.has(program)) return undefined

  for (let i = 0; i < rest.length; i++) {
    const name = nameOf(rest[i])
    if (DOCKERS.has(name)) {
      const args = rest.slice(i + 1)
      // O que o xargs acrescenta é um argumento que ninguém leu.
      if (isFedByStdin) args.push({ text: HOLE, open: true })
      const why = name.endsWith('-compose')
        ? composeWhy(args, i === 0)
        : dockerWhy(args, name === 'podman', i === 0)
      if (why !== undefined) return { command: cited(rest.slice(i)), ...why }
      break
    }
    if (TAKES_SCRIPT.has(name)) {
      const inner = scriptOf(rest.slice(i + 1))
      if (inner !== undefined) nested.push(inner)
    }
  }

  const args = rest.slice(1)
  // `$DOCKER compose down -v`, `$COMPOSE down -v`: o programa é a expansão.
  if (program === '') {
    const why = dockerWhy(args, false, false) ?? composeWhy(args, false)
    if (why !== undefined) return { command: cited(rest), ...why }
  }
  if (program === 'eval') nested.push(join(args))
  if (program === 'watch') nested.push(watched(args))
  if (program === 'script') {
    const inner = scriptOf(args)
    if (inner !== undefined) nested.push(inner)
  }

  return undefined
}

// Devolve o primeiro comando Docker destrutivo do script, ou undefined.
// `depth` limita a recursão de script dentro de script.
export function findDestructive(script: string, depth = 0): Verdict | undefined {
  if (depth > 6) {
    return {
      command: script.slice(0, 80),
      reason: 'it nests too deep to be read',
      instead: 'Run the inner command directly.',
    }
  }

  const { commands, nested } = parse(script)
  // Algum shell deste script lê o que outro comando escreve.
  let isShellFed = false
  for (const command of commands) {
    const found = judge(command, nested)
    if (found !== undefined) return found

    const program = nameOf(unwrap(command.words).rest[0])
    const isFed = command.afterPipe || command.substituted
    if (SHELLS.has(program) || program === 'source' || program === '.') {
      nested.push(...command.feeds)
      if (isFed) isShellFed = true
    }
    if (command.afterPipe && command.words.some(word => nameOf(word) === 'xargs')) isShellFed = true
  }
  // ponytail: sem seguir qual comando alimenta qual, todo texto do script é
  // candidato. Sobra falso positivo só em `echo "<docker destrutivo>" ... | sh`
  // onde o echo não é o que chega ao shell; seguir o pipeline se incomodar.
  if (isShellFed) {
    for (const command of commands) {
      nested.push(join(command.words.slice(1)), ...command.words.map(shown), ...command.feeds)
    }
  }
  for (const inner of nested) {
    const found = findDestructive(inner, depth + 1)
    if (found !== undefined) return found
  }

  return undefined
}

// A recusa que o modelo lê: o que foi barrado, por quê e o que fazer.
export const denial = (verdict: Verdict): string =>
  `Blocked \`${verdict.command}\`: ${verdict.reason}. ${verdict.instead} ${ASK_USER}`
