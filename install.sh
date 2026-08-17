#!/usr/bin/env bash
# Instala o Tesseract inteiro, numa linha só:
#
#   curl -fsSL https://raw.githubusercontent.com/AndreLuizMMS/tesseract/main/install.sh | bash
#
# Tudo se resolve aqui dentro: baixa o código se ele não estiver por perto,
# baixa o Go se a máquina não tiver um que sirva, compila o comando `ts`,
# instala o serviço de usuário, põe o diretório no PATH e reinicia o motor com
# o código novo. Rodar de novo por cima é seguro — é assim que se atualiza.
set -euo pipefail
# Sem isso, erro dentro de $( ) não derruba o script: um download que falha
# seguiria adiante e quebraria mais na frente, com a mensagem errada.
shopt -s inherit_errexit

repositorio="AndreLuizMMS/tesseract"
ramo="${TESSERACT_REF:-main}"
destino="${HOME}/.local/bin"
servicos="${HOME}/.config/systemd/user"
suporte="${HOME}/.local/share/tesseract"

# O que o script diz vai para o erro padrão: a saída padrão é o valor que as
# funções devolvem umas para as outras.
aviso() { printf '→ %s\n' "$*" >&2; }
erro() {
	printf '! %s\n' "$*" >&2
	exit 1
}

tem() { command -v "$1" >/dev/null 2>&1; }
precisa() { tem "$1" || erro "$1 não está instalado, e o instalador precisa dele"; }

# fonte devolve o diretório com o código. Rodando de dentro de um clone, é o
# próprio clone; vindo do curl, o código é baixado.
fonte() {
	local aqui
	aqui="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd)" || aqui=""
	if [ -n "$aqui" ] && [ -f "$aqui/go.mod" ] && [ -d "$aqui/cmd/tess" ]; then
		printf '%s' "$aqui"
		return
	fi
	aviso "baixando o código ($ramo)"
	rm -rf "${suporte:?}/fonte"
	mkdir -p "$suporte/fonte"
	# Há proxy que barra o tarball do GitHub e deixa o git passar, então o
	# segundo caminho existe para quando o primeiro não passa.
	if ! tarball 2>/dev/null && tem git; then
		aviso "o tarball do GitHub não passou — indo pelo git"
		rm -rf "${suporte:?}/fonte"
		git clone --quiet --depth 1 --branch "$ramo" \
			"https://github.com/${repositorio}.git" "$suporte/fonte" || true
	fi
	[ -f "$suporte/fonte/go.mod" ] || erro "não deu para baixar o código de ${repositorio} (${ramo})"
	printf '%s' "$suporte/fonte"
}

tarball() {
	tem curl && tem tar &&
		curl -fsSL "https://codeload.github.com/${repositorio}/tar.gz/refs/heads/${ramo}" |
		tar xz -C "$suporte/fonte" --strip-components=1
}

# serve diz se o Go da máquina é novo o bastante. De 1.21 em diante o próprio Go
# baixa a versão exata que o go.mod pede — abaixo disso, não adianta.
serve() {
	local versao="${1#go}" maior menor
	maior="${versao%%.*}"
	versao="${versao#*.}"
	menor="${versao%%.*}"
	case "$maior$menor" in *[!0-9]* | "") return 1 ;; esac
	[ "$maior" -gt 1 ] || { [ "$maior" -eq 1 ] && [ "$menor" -ge 21 ]; }
}

# comandoGo devolve o Go que vai compilar, baixando um se for preciso. O Go
# baixado fica num canto do Tesseract e não mexe no PATH de ninguém.
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
	*) erro "não há Go pronto para $(uname -m) — instale o Go 1.21+ na mão e rode de novo" ;;
	esac
	versao="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"
	[ -n "$versao" ] || erro "não deu para descobrir a versão do Go"
	aviso "baixando o $versao — a máquina não tem um Go que sirva"
	rm -rf "${suporte:?}/go"
	mkdir -p "$suporte"
	curl -fsSL "https://go.dev/dl/${versao}.linux-${arquitetura}.tar.gz" | tar xz -C "$suporte"
	printf '%s' "$suporte/go/bin/go"
}

# garantirPath só mexe no seu shell quando o diretório não está no caminho.
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
	aviso "$destino entrou no PATH pelo $arquivo — abra um terminal novo, ou rode: source $arquivo"
}

[ "$(uname -s)" = "Linux" ] || erro "o Tesseract roda em Linux (WSL com systemd); esta máquina é $(uname -s)"

raiz="$(fonte)"
go="$(comandoGo)"

aviso "compilando o ts"
mkdir -p "$destino"
provisorio="$(mktemp "${destino}/.ts.XXXXXX")"
trap 'rm -f "$provisorio"' EXIT
(cd "$raiz" && "$go" build -trimpath -o "$provisorio" ./cmd/tess)
chmod 755 "$provisorio"
# Troca por rename: o motor que está de pé continua no binário antigo até o
# restart, em vez de o build esbarrar num arquivo em uso.
mv -f "$provisorio" "$destino/ts"
trap - EXIT

aviso "instalando o serviço"
mkdir -p "$servicos"
cp "$raiz/systemd/tesseract.service" "$servicos/tesseract.service"

if tem systemctl && systemctl --user show-environment >/dev/null 2>&1; then
	systemctl --user daemon-reload
	systemctl --user enable tesseract.service >/dev/null
	# restart, não start: numa atualização o motor já está de pé com o código
	# velho, e é o restart que o faz ler o binário novo.
	systemctl --user restart tesseract.service
	aviso "motor no ar com o código novo"
else
	aviso "systemd de usuário indisponível: o motor sobe sozinho quando você rodar 'ts'"
fi

garantirPath

printf '\npronto. rode `ts` dentro de um projeto.\n' >&2
