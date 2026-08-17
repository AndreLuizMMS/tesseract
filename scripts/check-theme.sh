#!/usr/bin/env bash
# scripts/check-theme.sh
#
# Guarda das três regras que não se negociam:
#
#   1. nenhuma cor verde ou ciano da paleta pode aparecer no mapa de estados
#      — verde é posse do teclado, ciano é estrutura;
#   2. nenhum hex escrito à mão fora do arquivo de tema;
#   3. a paleta inteira sai impressa em blocos ANSI, para conferência a olho.
#
# Sai com 1 se qualquer regra quebrar.

set -u

raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$raiz" || exit 1

ARQ_TEMA="internal/tema/tema.go"
MARCA_INICIO='>>> mapa-de-estados'
MARCA_FIM='<<< mapa-de-estados'

falhas=0

erro() {
	printf '\033[1;31mFALHA\033[0m  %s\n' "$1" >&2
	falhas=$((falhas + 1))
}

ok() {
	printf '\033[1;32mok\033[0m     %s\n' "$1"
}

# ---------------------------------------------------------------------------
# Regra 1 — estado nunca é verde nem ciano.
# ---------------------------------------------------------------------------

VERDES_E_CIANOS=(
	'#0B3322' '#1F7A4C' '#35C27A' '#55FFA6'
	'#082F31' '#128C86' '#22E0D0'
)

# Nomes dos tokens proibidos, porque o mapa de estados usa constante, não hex.
TOKENS_PROIBIDOS=(
	'BrandDeep' 'BrandCore' 'BrandLive' 'BrandPhosphor'
	'FluxDeep' 'FluxCore' 'Flux'
)

if [ ! -f "$ARQ_TEMA" ]; then
	erro "arquivo de tema não encontrado: $ARQ_TEMA"
else
	mapa="$(awk "/$MARCA_INICIO/{dentro=1; next} /$MARCA_FIM/{dentro=0} dentro" "$ARQ_TEMA")"

	if [ -z "$mapa" ]; then
		erro "não achei o bloco entre '$MARCA_INICIO' e '$MARCA_FIM' em $ARQ_TEMA"
	else
		for cor in "${VERDES_E_CIANOS[@]}"; do
			if grep -qiF "$cor" <<<"$mapa"; then
				erro "o mapa de estados usa $cor — verde e ciano têm dono e não são estado"
			fi
		done

		for token in "${TOKENS_PROIBIDOS[@]}"; do
			# \b para 'Flux' não casar dentro de 'FluxCore' duas vezes.
			if grep -qE "\b${token}\b" <<<"$mapa"; then
				erro "o mapa de estados usa o token ${token} — verde e ciano não são estado"
			fi
		done

		if [ "$falhas" -eq 0 ]; then
			ok "mapa de estados sem verde e sem ciano"
		fi
	fi
fi

# ---------------------------------------------------------------------------
# Regra 2 — hex só dentro do arquivo de tema.
# ---------------------------------------------------------------------------
#
# O escopo é o código Go da aplicação. A pasta themes/ é feita de arquivos de
# tema de terceiros (kitty, tmux, nvim...) e, por definição, é onde o hex mora.

soltos="$(
	find . -name '*.go' \
		-not -path "./$ARQ_TEMA" \
		-not -path './internal/tema/*' \
		-not -path './vendor/*' \
		-print0 |
		xargs -0 grep -nE '"#[0-9A-Fa-f]{6}"' 2>/dev/null
)"

if [ -n "$soltos" ]; then
	erro "hex escrito fora de $ARQ_TEMA:"
	printf '%s\n' "$soltos" | sed 's/^/         /' >&2
else
	ok "nenhum hex fora de $ARQ_TEMA"
fi

# ---------------------------------------------------------------------------
# Regra 3 — imprimir a paleta.
# ---------------------------------------------------------------------------

bloco() { # bloco <hex> <token> <papel>
	local hex="$1" token="$2" papel="$3"
	local r g b
	r=$((16#${hex:1:2}))
	g=$((16#${hex:3:2}))
	b=$((16#${hex:5:2}))
	if [ -n "${NO_COLOR:-}" ]; then
		printf '        %-7s  %-14s  %s\n' "$hex" "$token" "$papel"
	else
		printf '  \033[48;2;%d;%d;%dm      \033[0m  \033[38;2;%d;%d;%dm%-7s\033[0m  %-14s  %s\n' \
			"$r" "$g" "$b" "$r" "$g" "$b" "$hex" "$token" "$papel"
	fi
}

titulo() {
	if [ -n "${NO_COLOR:-}" ]; then
		printf '\n  %s\n' "$1"
	else
		printf '\n  \033[1m%s\033[0m\n' "$1"
	fi
}

printf '\n'
titulo 'BASE'
bloco '#030507' 'BgVoid' 'fundo do modo DIGITAR'
bloco '#070B0C' 'BgBase' 'fundo padrão'
bloco '#0C1315' 'BgSurface' 'corpo da célula'
bloco '#121C1F' 'BgRaised' 'cabeçalho, seleção'
bloco '#16282A' 'LineDim' 'grade não focada'
bloco '#205047' 'LineActive' 'grade do projeto focado'
bloco '#3E534E' 'FgFaint' 'desligado, atalhos'
bloco '#6C8076' 'FgMuted' 'texto secundário'
bloco '#BFD1C6' 'FgDefault' 'texto principal'
bloco '#E8F4EC' 'FgBright' 'títulos'

titulo 'NEON VERDE — posse do teclado'
bloco '#0B3322' 'BrandDeep' 'verde de fundo'
bloco '#1F7A4C' 'BrandCore' 'grade do projeto ativo, logo'
bloco '#35C27A' 'BrandLive' 'célula focada'
bloco '#55FFA6' 'BrandPhosphor' 'dono do teclado — 1 por tela'

titulo 'NEON CIANO — estrutura'
bloco '#082F31' 'FluxDeep' 'ciano de fundo'
bloco '#128C86' 'FluxCore' 'grade, cantos, rótulos'
bloco '#22E0D0' 'Flux' 'glifo, numeração'

titulo 'ESTADOS — nenhum verde, nenhum ciano'
bloco '#6C8076' 'StateWorking' '▸ trabalhando'
bloco '#7DB7E8' 'StateRead' '⬤ respondeu'
bloco '#FFB454' 'StateBlock' '⏵ aprovar'
bloco '#FF3B47' 'StateDead' '✖ caiu'
bloco '#3E534E' 'StateOff' '○ parada'
bloco '#C77DFF' 'StateOrphan' '⚠ órfã'

titulo 'ANSI 16'
bloco '#070B0C' '0  preto' ''
bloco '#C22F38' '1  vermelho' ''
bloco '#1F7A4C' '2  verde' ''
bloco '#C9A227' '3  amarelo' ''
bloco '#3E7FA8' '4  azul' ''
bloco '#8B4FC4' '5  magenta' ''
bloco '#128C86' '6  ciano' ''
bloco '#BFD1C6' '7  branco' ''
bloco '#3E534E' '8  preto+' ''
bloco '#FF3B47' '9  vermelho+' ''
bloco '#55FFA6' '10 verde+' ''
bloco '#FFB454' '11 amarelo+' ''
bloco '#7DB7E8' '12 azul+' ''
bloco '#C77DFF' '13 magenta+' ''
bloco '#22E0D0' '14 ciano+' ''
bloco '#E8F4EC' '15 branco+' ''

titulo 'ALFABETO DE ESTADOS — o teste sem cor'
printf '  Tape a cor com a mão: o glifo e a forma têm que bastar.\n\n'
printf '    ▸ TRABALHANDO   ⬤ RESPONDEU   ✖ CAIU   ○ PARADA   ⚠ ÓRFÃ\n'
if [ -n "${NO_COLOR:-}" ]; then
	printf '    %s\n' "$(printf '\033[7m⏵ APROVAR — barra sólida, linha inteira\033[0m')"
else
	printf '    \033[48;2;255;180;84m\033[38;2;3;5;7m\033[1m ⏵ APROVAR — barra sólida, ocupa a linha inteira            \033[0m\n'
fi

printf '\n'
if [ "$falhas" -gt 0 ]; then
	printf '\033[1;31m%d regra(s) quebrada(s).\033[0m\n' "$falhas" >&2
	exit 1
fi
printf '\033[1;32mtema íntegro.\033[0m\n'
