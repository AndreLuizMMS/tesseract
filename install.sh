#!/usr/bin/env bash
# Installs Tesseract entirely, in one line:
#
#   curl -fsSL https://raw.githubusercontent.com/AndreLuizMMS/tesseract/main/install.sh | bash
#
# Everything is handled right here: downloads the code if it isn't around,
# downloads Go if the machine doesn't have a version that works, builds the
# `ts` command, installs the user service, puts the directory on PATH and
# restarts the engine with the new code. Running it again on top is safe —
# that's how you update.
set -euo pipefail
# Without this, an error inside $( ) wouldn't bring the script down: a failed
# download would carry on and break further ahead, with the wrong message.
shopt -s inherit_errexit

repositorio="AndreLuizMMS/tesseract"
ramo="${TESSERACT_REF:-main}"
destino="${HOME}/.local/bin"
servicos="${HOME}/.config/systemd/user"
suporte="${HOME}/.local/share/tesseract"

# What the script says goes to stderr: stdout is the value functions hand
# each other.
aviso() { printf '→ %s\n' "$*" >&2; }
erro() {
	printf '! %s\n' "$*" >&2
	exit 1
}

tem() { command -v "$1" >/dev/null 2>&1; }
precisa() { tem "$1" || erro "$1 is not installed, and the installer needs it"; }

# fonte returns the directory holding the code. Running from inside a clone,
# it's the clone itself; coming from curl, the code gets downloaded.
fonte() {
	local aqui
	aqui="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd)" || aqui=""
	if [ -n "$aqui" ] && [ -f "$aqui/go.mod" ] && [ -d "$aqui/cmd/tess" ]; then
		printf '%s' "$aqui"
		return
	fi
	aviso "downloading the code ($ramo)"
	rm -rf "${suporte:?}/fonte"
	mkdir -p "$suporte/fonte"
	# Some proxies block the GitHub tarball but let git through, so the
	# second path exists for when the first one doesn't.
	if ! tarball 2>/dev/null && tem git; then
		aviso "the GitHub tarball didn't go through — falling back to git"
		rm -rf "${suporte:?}/fonte"
		git clone --quiet --depth 1 --branch "$ramo" \
			"https://github.com/${repositorio}.git" "$suporte/fonte" || true
	fi
	[ -f "$suporte/fonte/go.mod" ] || erro "couldn't download the code from ${repositorio} (${ramo})"
	printf '%s' "$suporte/fonte"
}

tarball() {
	tem curl && tem tar &&
		curl -fsSL "https://codeload.github.com/${repositorio}/tar.gz/refs/heads/${ramo}" |
		tar xz -C "$suporte/fonte" --strip-components=1
}

# serve says whether the machine's Go is new enough. From 1.21 onward Go
# itself downloads the exact version go.mod asks for — below that, it's no use.
serve() {
	local versao="${1#go}" maior menor
	maior="${versao%%.*}"
	versao="${versao#*.}"
	menor="${versao%%.*}"
	case "$maior$menor" in *[!0-9]* | "") return 1 ;; esac
	[ "$maior" -gt 1 ] || { [ "$maior" -eq 1 ] && [ "$menor" -ge 21 ]; }
}

# comandoGo returns the Go that will do the build, downloading one if needed.
# A downloaded Go stays tucked inside Tesseract's own space and doesn't touch
# anyone's PATH.
comandoGo() {
	if tem go && serve "$(go env GOVERSION 2>/dev/null || echo go0)"; then
		command -v go
		return
	fi
	if [ -x "$suporte/go/bin/go" ] && serve "$("$suporte/go/bin/go" env GOVERSION)"; then
		printf '%s' "$suporte/go/bin/go"
		return
	fi
	precisa curl
	precisa tar
	local arquitetura versao
	case "$(uname -m)" in
	x86_64 | amd64) arquitetura=amd64 ;;
	aarch64 | arm64) arquitetura=arm64 ;;
	*) erro "no ready-made Go for $(uname -m) — install Go 1.21+ by hand and run again" ;;
	esac
	versao="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"
	[ -n "$versao" ] || erro "couldn't figure out the Go version"
	aviso "downloading $versao — this machine has no Go that works"
	rm -rf "${suporte:?}/go"
	mkdir -p "$suporte"
	curl -fsSL "https://go.dev/dl/${versao}.linux-${arquitetura}.tar.gz" | tar xz -C "$suporte"
	printf '%s' "$suporte/go/bin/go"
}

# garantirPath only touches your shell when the directory isn't on the path.
garantirPath() {
	case ":$PATH:" in *":$destino:"*) return ;; esac
	local arquivo linha='export PATH="$HOME/.local/bin:$PATH"'
	case "${SHELL##*/}" in
	zsh) arquivo="$HOME/.zshrc" ;;
	bash) arquivo="$HOME/.bashrc" ;;
	*) arquivo="$HOME/.profile" ;;
	esac
	if ! grep -qsF "$linha" "$arquivo"; then
		printf '\n# tesseract\n%s\n' "$linha" >>"$arquivo"
	fi
	aviso "$destino was added to PATH via $arquivo — open a new terminal, or run: source $arquivo"
}

[ "$(uname -s)" = "Linux" ] || erro "Tesseract runs on Linux (WSL with systemd); this machine is $(uname -s)"

raiz="$(fonte)"
go="$(comandoGo)"

aviso "building ts"
mkdir -p "$destino"
provisorio="$(mktemp "${destino}/.ts.XXXXXX")"
trap 'rm -f "$provisorio"' EXIT
(cd "$raiz" && "$go" build -trimpath -ldflags="-s -w" -o "$provisorio" ./cmd/tess)
chmod 755 "$provisorio"
# Swap by rename: the engine that's already running stays on the old binary
# until the restart, instead of the build tripping over a file in use.
mv -f "$provisorio" "$destino/ts"
trap - EXIT

aviso "installing the service"
mkdir -p "$servicos"
cp "$raiz/systemd/tesseract.service" "$servicos/tesseract.service"

# An engine brought up by hand — which is what `ts` does when it finds none —
# doesn't belong to the service, so restarting the unit would leave it there
# serving the old binary and the change wouldn't show up. Whoever is holding
# the socket goes down first, no matter who started it.
if [ -S "${XDG_RUNTIME_DIR:-$HOME/.local/state}/tesseract/engine.sock" ] ||
	pgrep -f "^$destino/ts engine$" >/dev/null 2>&1; then
	aviso "shutting down the engine that's already running"
	# Anchored: without it the pattern also matches any shell whose command
	# line merely mentions the engine — this very script included.
	"$destino/ts" stop >/dev/null 2>&1 || pkill -f "^$destino/ts engine$" || true
fi

if tem systemctl && systemctl --user show-environment >/dev/null 2>&1; then
	systemctl --user daemon-reload
	systemctl --user enable tesseract.service >/dev/null
	# restart, not start: on an update the engine is already up with the old
	# code, and it's the restart that makes it pick up the new binary.
	systemctl --user restart tesseract.service
	aviso "engine up with the new code"
else
	aviso "user systemd unavailable: the engine will come up on its own when you run 'ts'"
fi

garantirPath

printf '\ndone. run `ts` inside a project.\n' >&2
