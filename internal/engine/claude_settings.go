package engine

// dockerDenyRules are Claude Code permission rules for the Docker commands
// that delete data — the ones the tesseract-docker mod's guard refuses,
// written in the form the agent usually produces.
//
// A rule matches the command's text, each part of a compound command on its
// own, with * standing for any text. That can't read a shell the way the
// guard does: the same command behind `sh -c`, a full path or a global flag
// (`docker --context x compose down -v`) gets past these and is the guard's
// to stop. What's here is the subset a pattern expresses with no false
// positive on the commands that keep the data — `docker compose down` alone
// matches none of them.
//
// Left out on purpose: `docker compose rm -v`, whose pattern would also deny
// `docker compose run --rm -v ./src:/app …`, and `docker compose up -V`.
var dockerDenyRules = []string{
	"Bash(docker compose* down* -v*)",
	"Bash(docker compose* down* --volumes*)",
	"Bash(docker compose* down* --rmi*)",
	"Bash(docker-compose* down* -v*)",
	"Bash(docker-compose* down* --volumes*)",
	"Bash(docker-compose* down* --rmi*)",
	"Bash(docker volume rm*)",
	"Bash(docker volume remove*)",
	"Bash(docker volume prune*)",
	"Bash(docker system prune*)",
	"Bash(docker container prune*)",
	"Bash(docker image prune*)",
	"Bash(docker network prune*)",
	"Bash(docker builder prune*)",
	"Bash(docker buildx prune*)",
	"Bash(docker rm* -v*)",
}

// claudeSettings is what rides on --settings for a cell's session: it holds
// for that session and touches nothing the user has outside Tesseract.
func (e *Engine) claudeSettings() map[string]any {
	settings := map[string]any{}
	if theme := e.config.ClaudeTheme; theme != "" {
		settings["theme"] = theme
	}
	if e.config.DockerDeny {
		settings["permissions"] = map[string]any{"deny": dockerDenyRules}
	}
	return settings
}
