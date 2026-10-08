#!/usr/bin/env bash
# scripts/check-theme.sh
#
# Guards the three rules that don't get negotiated:
#
#   1. no green or cyan from the palette may appear in the state map
#      — green belongs to keyboard ownership, cyan belongs to structure;
#   2. no hand-written color outside the theme file — neither a hex nor a
#      color escape code typed by hand;
#   3. the whole palette gets printed as ANSI blocks, for a visual check.
#
# The palette printed here is the brand sheet's, token by token:
# style/tesseract-brand-identity.html, sections // 04 and // 10.
#
# Exits 1 if any rule breaks.

set -u

raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$raiz" || exit 1

ARQ_TEMA="internal/theme/theme.go"
MARCA_INICIO='>>> state-map'
MARCA_FIM='<<< state-map'

falhas=0

erro() {
	printf '\033[1;31mFAIL\033[0m  %s\n' "$1" >&2
	falhas=$((falhas + 1))
}

ok() {
	printf '\033[1;32mok\033[0m     %s\n' "$1"
}

# ---------------------------------------------------------------------------
# Rule 1 — state is never green nor cyan.
# ---------------------------------------------------------------------------

VERDES_E_CIANOS=(
	'#0B3322' '#1F7A4C' '#35C27A' '#55FFA6'
	'#082F31' '#128C86' '#22E0D0'
)

# Names of the forbidden tokens, because the state map uses a constant, not hex.
TOKENS_PROIBIDOS=(
	'BrandDeep' 'BrandCore' 'BrandLive' 'BrandPhosphor'
	'FluxDeep' 'FluxCore' 'Flux'
)

if [ ! -f "$ARQ_TEMA" ]; then
	erro "theme file not found: $ARQ_TEMA"
else
	mapa="$(awk "/$MARCA_INICIO/{dentro=1; next} /$MARCA_FIM/{dentro=0} dentro" "$ARQ_TEMA")"

	if [ -z "$mapa" ]; then
		erro "couldn't find the block between '$MARCA_INICIO' and '$MARCA_FIM' in $ARQ_TEMA"
	else
		for cor in "${VERDES_E_CIANOS[@]}"; do
			if grep -qiF "$cor" <<<"$mapa"; then
				erro "the state map uses $cor — green and cyan have an owner and aren't state"
			fi
		done

		for token in "${TOKENS_PROIBIDOS[@]}"; do
			# \b so 'Flux' doesn't match twice inside 'FluxCore'.
			if grep -qE "\b${token}\b" <<<"$mapa"; then
				erro "the state map uses the token ${token} — green and cyan aren't state"
			fi
		done

		if [ "$falhas" -eq 0 ]; then
			ok "state map clean of green and cyan"
		fi
	fi
fi

# ---------------------------------------------------------------------------
# Rule 2 — hex only lives inside the theme file.
# ---------------------------------------------------------------------------
#
# The scope is the application's Go code. The themes/ folder is made of
# third-party theme files (kitty, tmux, nvim...) and, by definition, that's
# where hex belongs.

soltos="$(
	find . -name '*.go' \
		-not -path "./$ARQ_TEMA" \
		-not -path './internal/theme/*' \
		-not -path './vendor/*' \
		-not -path './.claude/*' \
		-print0 |
		xargs -0 grep -nE '"#[0-9A-Fa-f]{6}"' 2>/dev/null
)"

if [ -n "$soltos" ]; then
	erro "hex written outside $ARQ_TEMA:"
	printf '%s\n' "$soltos" | sed 's/^/         /' >&2
else
	ok "no hex outside $ARQ_TEMA"
fi

# A color escape typed by hand is the same leak with another spelling: an
# indexed or a 24-bit color that the theme never heard of. Tests are left
# out — they write escapes to feed the screen what an agent would.
escapes="$(
	find . -name '*.go' -not -name '*_test.go' \
		-not -path './internal/theme/*' \
		-not -path './vendor/*' \
		-not -path './.claude/*' \
		-print0 |
		xargs -0 grep -nE '\\x1b\[[0-9;]*[34]8;[25];' 2>/dev/null
)"

if [ -n "$escapes" ]; then
	erro "color escape written by hand outside $ARQ_TEMA:"
	printf '%s\n' "$escapes" | sed 's/^/         /' >&2
else
	ok "no hand-written color escape outside $ARQ_TEMA"
fi

# ---------------------------------------------------------------------------
# Rule 3 — print the palette.
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
bloco '#030507' 'BgVoid' 'TYPE mode background'
bloco '#070B0C' 'BgBase' 'default background'
bloco '#0C1315' 'BgSurface' 'cell body'
bloco '#121C1F' 'BgRaised' 'header, selection'
bloco '#16282A' 'LineDim' 'unfocused grid, everything in TYPE'
bloco '#205047' 'LineActive' 'cell lines'
bloco '#3E534E' 'FgFaint' 'off, shortcuts'
bloco '#6C8076' 'FgMuted' 'secondary text'
bloco '#BFD1C6' 'FgDefault' 'primary text'
bloco '#E8F4EC' 'FgBright' 'titles'

titulo 'NEON GREEN — keyboard ownership'
bloco '#0B3322' 'BrandDeep' 'background green'
bloco '#1F7A4C' 'BrandCore' 'active project grid, front square'
bloco '#35C27A' 'BrandLive' 'focused cell'
bloco '#55FFA6' 'BrandPhosphor' 'keyboard owner — 1 per screen'

titulo 'NEON CYAN — structure'
bloco '#082F31' 'FluxDeep' 'background cyan'
bloco '#128C86' 'FluxCore' 'grid, corners, labels'
bloco '#22E0D0' 'Flux' 'glyph, numbering'

titulo 'STATES — no green, no cyan'
bloco '#6C8076' 'StateWorking' '▸ working'
bloco '#7DB7E8' 'StateRead' '⬤ responded'
bloco '#FFB454' 'StateBlock' '⏵ approve'
bloco '#835F30' 'StateBlockDim' '⏵ approve, dim frame of the blink'
bloco '#FF3B47' 'StateDead' '✖ crashed'
bloco '#3E534E' 'StateOff' '○ stopped'
bloco '#C77DFF' 'StateOrphan' '⚠ orphaned'

titulo 'ANSI 16'
bloco '#070B0C' '0  black' ''
bloco '#C22F38' '1  red' ''
bloco '#1F7A4C' '2  green' ''
bloco '#C9A227' '3  yellow' ''
bloco '#3E7FA8' '4  blue' ''
bloco '#8B4FC4' '5  magenta' ''
bloco '#128C86' '6  cyan' ''
bloco '#BFD1C6' '7  white' ''
bloco '#3E534E' '8  black+' ''
bloco '#FF3B47' '9  red+' ''
bloco '#55FFA6' '10 green+' ''
bloco '#FFB454' '11 yellow+' ''
bloco '#7DB7E8' '12 blue+' ''
bloco '#C77DFF' '13 magenta+' ''
bloco '#22E0D0' '14 cyan+' ''
bloco '#E8F4EC' '15 white+' ''

titulo 'TERMINAL'
bloco '#070B0C' 'background' ''
bloco '#BFD1C6' 'foreground' ''
bloco '#55FFA6' 'cursor' ''
bloco '#030507' 'cursor_text' ''
bloco '#121C1F' 'selection' ''

titulo 'STATE ALPHABET — the no-color test'
printf '  Cover the color with your hand: the glyph and shape have to be enough.\n\n'
printf '    ▸ WORKING   ⬤ RESPONDED   ✖ CRASHED   ○ STOPPED   ⚠ ORPHANED\n'
if [ -n "${NO_COLOR:-}" ]; then
	printf '    %s\n' "$(printf '\033[7m⏵ APPROVE — solid bar, whole line\033[0m')"
else
	printf '    \033[48;2;255;180;84m\033[38;2;7;11;12m\033[1m ⏵ APPROVE — solid bar, occupies the whole line            \033[0m\n'
fi

printf '\n'
if [ "$falhas" -gt 0 ]; then
	printf '\033[1;31m%d rule(s) broken.\033[0m\n' "$falhas" >&2
	exit 1
fi
printf '\033[1;32mtheme intact.\033[0m\n'
