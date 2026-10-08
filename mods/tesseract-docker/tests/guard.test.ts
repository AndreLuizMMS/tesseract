import { describe, expect, test } from 'claude-code/testing'

import { denial, findDestructive } from '../hooks/guard'

const DESTRUCTIVE = [
  'docker compose down -v',
  'docker compose -f docker/compose.yml -p api down --volumes --remove-orphans',
  'docker-compose down -v',
  'cd infra && docker compose down --rmi all',
  'sudo docker system prune -af',
  'sudo -u root docker volume prune -f',
  'docker volume rm api_pgdata',
  'docker volume prune -f',
  'bash -c "docker compose down -v"',
  'bash -lc \'docker system prune --volumes\'',
  'echo ok; FOO=1 docker compose rm -fsv db',
  'docker --context dev compose down -v',
  'echo "$(docker volume prune -f)"',
  'echo `docker volume rm x`',
  'docker compose down \\\n  -v',
  "bash <<'EOF'\ndocker compose down -v\nEOF",
  'echo a | xargs docker volume rm',
  'timeout 30 docker compose down -v',
  'if true; then docker compose down -v; fi',
  '/usr/bin/docker rm -fv old',
  'docker.exe compose down --volumes',
  'eval "docker compose down -v"',
  'docker compose stop && docker compose down -v 2>&1 | tee out.log',
  // O que o validador fez passar: texto entregue a um shell.
  'echo "docker compose down -v" | sh',
  'echo "docker compose down -v" | tee /tmp/x | bash -s',
  'printf \'%s\\n\' \'docker volume rm data\' | sudo bash',
  'bash <<< "docker compose down -v"',
  "cat <<'EOF' | sh\ndocker compose down -v\nEOF",
  'bash <(echo "docker compose down -v")',
  'echo "docker compose down -v" | xargs -I{} sh -c "{}"',
  'find . -maxdepth 0 -exec docker compose down -v \\;',
  "find . -name compose.yml -execdir sh -c 'docker compose down -v' \\;",
  'script -qc "docker compose down -v" /dev/null',
  'script -q --command="docker compose down -v" /dev/null',
  'watch -n 5 "docker compose down -v"',
  'watch docker compose down -v',
  'env -S "docker compose down -v"',
  'su -c "docker compose down -v" root',
  'nohup docker compose down -v &',
  'time docker compose down -v',
  'doas docker volume prune -f',
  'flock /tmp/lock docker compose down -v',
  // Flag escrita de outro jeito.
  'docker compose down -v=true',
  'docker compose down --volumes=true',
  "$'docker' compose down -v",
  "$'\\x64ocker' compose down $'-v'",
  '"docker" compose "down" "-v"',
  '\\docker compose down -v',
  // Expansão que a guarda não resolve, com subcomando perigoso: nega.
  'docker compose down $(echo -v)',
  'docker compose down `echo -v`',
  'docker compose down $FLAGS',
  'docker compose down "$FLAGS"',
  'docker compose down ${FLAGS:-}',
  'V=-v; docker compose down $V',
  'docker compose down -{v,}',
  'docker compose rm -f $SERVICES',
  'docker rm $(docker ps -aq)',
  'docker ps -aq | xargs docker rm',
  'docker compose $ACTION',
  'docker $SUB prune',
  'docker volume $VERB data',
  '$DOCKER compose down -v',
  '$COMPOSE down --volumes',
  '$SUDO docker system prune -af',
  "bash -c 'docker compose down \"$1\"' _ -v",
  // Heredoc sem aspas: o shell roda o $(...) do corpo antes do cat.
  'cat <<EOF\n$(docker compose down -v)\nEOF',
  'cat <<EOF > out.txt\nstate: `docker volume prune -f`\nEOF',
  // Outras formas de perder dado.
  'docker compose up -d -V',
  'docker compose up --renew-anon-volumes',
  'docker container prune -f',
  'docker image prune -a',
  'docker builder prune -af',
  'docker network prune',
  'podman compose down -v',
  'podman-compose down --volumes',
  'podman volume rm data',
  'podman system prune -a',
  'podman system reset',
]

const HARMLESS = [
  'docker compose down',
  'docker compose down --remove-orphans',
  'docker compose down --volumes=false',
  'docker compose stop db',
  'docker compose up -d',
  'docker compose restart api',
  'docker compose -f down.yml up -d',
  'docker run --rm -v ./data:/data alpine ls',
  'docker compose run --rm -v ./x:/x app sh',
  'docker compose logs -f --tail 50 api',
  'docker compose exec db psql -c "drop table volume"',
  'docker volume ls',
  'docker system df',
  'docker rm old-container',
  'docker ps -a',
  'echo "docker compose down -v"',
  "grep -rn 'docker system prune' docs/",
  'git commit -m "docs: explica docker volume rm"',
  "cat <<'EOF' > README.md\ndocker compose down -v\nEOF",
  '# docker system prune\nls',
  'ls -v && echo down',
  'npm run docker:down -- -v',
  'rg "volume prune" > /tmp/out.txt 2>&1',
  '',
  // Falsos positivos que a guarda nova não pode criar.
  'docker compose logs -v',
  "grep 'system prune' compose.yml",
  "git commit -m 'texto com docker compose down -v em aspas simples'",
  'git commit -m "explica $(basename compose.yml): docker compose down -v"',
  'echo "docker compose down -v" | grep down',
  'echo docker compose down -v',
  'echo "docker compose down -v" > notes.txt',
  "cat <<'EOF'\n$(docker compose down -v)\nEOF",
  'cat <<EOF > README.md\ndocker compose down -v\nEOF',
  'man docker volume rm',
  'sudo usermod -aG docker $USER',
  'docker compose -f "$PWD/compose.yml" up -d',
  'docker compose up -d $SERVICE',
  'docker compose up -d --force-recreate',
  'docker compose down --volumes=0',
  'docker compose down -v=false',
  'docker compose down -t 5',
  'docker compose down -- $SERVICE',
  'docker compose stop $SERVICE',
  'docker logs $(docker ps -q)',
  'docker exec $CID ls',
  'docker rm "api-$ID"',
  'docker volume inspect $VOLUME',
  'docker volume ls -q | xargs docker volume inspect',
  'docker ps -q | xargs docker stop',
  'docker image ls',
  'docker network ls',
  'podman ps -a',
  'command -v docker',
  'find . -name "*.yml" -exec grep -l "down -v" {} \\;',
  'curl -fsSL https://example.com/install.sh | sh',
  'ls | sh -c "cat"',
  'sed -i "s/docker compose down -v/docker compose down/" README.md',
  // Mensagem de commit por heredoc com aspas: é texto, não comando.
  "git commit -m \"$(cat <<'EOF'\nfix: explica docker compose down -v\nEOF\n)\"",
  "gh pr create --title x --body \"$(cat <<'EOF'\nRun `docker volume prune` by hand.\nEOF\n)\"",
]

describe('guarda Docker', () => {
  for (const command of DESTRUCTIVE) {
    test(`nega: ${command.replace(/\n/g, '⏎')}`, () => {
      expect(findDestructive(command)).toBeDefined()
    })
  }
  for (const command of HARMLESS) {
    test(`deixa passar: ${command.replace(/\n/g, '⏎')}`, () => {
      expect(findDestructive(command)).toBeUndefined()
    })
  }

  test('a recusa cita só o comando simples culpado', () => {
    expect(findDestructive('cd infra && docker compose down -v')?.command).toBe('docker compose down -v')
    expect(findDestructive('find . -exec docker compose down -v \\;')?.command).toBe('docker compose down -v ;')
    expect(findDestructive('docker compose down $(echo -v)')?.command).toBe('docker compose down $…')
  })

  test('a recusa diz o que usar em cada caso, sem receita genérica', () => {
    const said = (command: string): string => {
      const verdict = findDestructive(command)

      return verdict === undefined ? '' : denial(verdict)
    }
    expect(said('docker compose down -v')).toMatch(/`docker compose down` without -v/)
    expect(said('docker volume rm data')).toMatch(/docker volume inspect/)
    expect(said('docker volume rm data')).not.toMatch(/without -v/)
    expect(said('docker system prune -af')).toMatch(/docker system df/)
    expect(said('docker system prune -af')).not.toMatch(/without -v/)
    expect(said('docker compose up -V')).toMatch(/--force-recreate/)
    expect(said('docker compose down $(echo -v)')).toMatch(/expansion .* the guard cannot read/)
    expect(said('docker compose $ACTION')).toMatch(/subcommand comes from an expansion/)
    for (const command of ['docker compose down -v', 'docker volume rm x', 'docker rm $X']) {
      expect(said(command)).toMatch(/ask them to run it themselves/)
    }
  })
})

describe('guarda Docker no tool.call', () => {
  test('Bash destrutivo é negado e nada roda', async ($, on) => {
    let ran = 0
    on('tool.call', { tool: 'Bash' }, () => {
      ran += 1

      return { result: 'ran' }
    })

    const denied = await $.tool.call({ tool: 'Bash', command: 'docker compose down -v' })
    expect(denied.deny).toMatch(/Blocked `docker compose down -v`/)
    expect(ran).toBe(0)

    const allowed = await $.tool.call({ tool: 'Bash', command: 'docker compose down' })
    expect(allowed.deny).toBeUndefined()
    expect(ran).toBe(1)
  })
})
