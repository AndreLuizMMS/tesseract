#!/bin/sh
# themes/eza-ls-colors.sh
# LS_COLORS e EZA_COLORS do Tesseract Neon.  source este arquivo no seu rc.
#
# Decisão registrada: NENHUM verde aqui, em nenhum tom.
#   * Verde é posse do teclado, e uma listagem de diretório imprime dezenas de
#     linhas — o executável em verde repetiria a cor de posse à exaustão.
#     Executável fica em laranja de bloqueio (#FFB454): é o item que age.
#   * Ciano é estrutura, e diretório é a estrutura da árvore — esse é o único
#     uso de ciano na listagem (#128C86 para pasta, #22E0D0 para link).
# O resto sai das cores da ANSI 16 que não são estado: azul, roxo, amarelo.

# Truecolor: 38;2;R;G;B
_ts_fg() { printf '38;2;%d;%d;%d' "$1" "$2" "$3"; }

TS_FLUX_CORE=$(_ts_fg 18 140 134)    # #128C86  pasta
TS_FLUX=$(_ts_fg 34 224 208)         # #22E0D0  link
TS_BLOCK=$(_ts_fg 255 180 84)        # #FFB454  executável
TS_DEAD=$(_ts_fg 255 59 71)          # #FF3B47  link quebrado, órfão
TS_ORPHAN=$(_ts_fg 199 125 255)      # #C77DFF  arquivo comprimido
TS_READ=$(_ts_fg 125 183 232)        # #7DB7E8  imagem, vídeo
TS_YELLOW=$(_ts_fg 201 162 39)       # #C9A227  documento
TS_DEFAULT=$(_ts_fg 191 209 198)     # #BFD1C6  arquivo comum
TS_MUTED=$(_ts_fg 108 128 118)       # #6C8076  metadado secundário
TS_FAINT=$(_ts_fg 62 83 78)          # #3E534E  ignorado, oculto
TS_BRIGHT=$(_ts_fg 232 244 236)      # #E8F4EC  nome em destaque
TS_PURPLE=$(_ts_fg 139 79 196)       # #8B4FC4  socket, fifo, device

LS_COLORS="\
di=1;${TS_FLUX_CORE}:\
ln=${TS_FLUX}:\
or=${TS_DEAD};1:\
mh=${TS_FLUX}:\
ex=1;${TS_BLOCK}:\
fi=${TS_DEFAULT}:\
pi=${TS_PURPLE}:\
so=${TS_PURPLE}:\
do=${TS_PURPLE}:\
bd=${TS_PURPLE};1:\
cd=${TS_PURPLE};1:\
su=${TS_DEAD};1:\
sg=${TS_DEAD};1:\
tw=${TS_FLUX_CORE};1:\
ow=${TS_FLUX_CORE}:\
st=${TS_FLUX_CORE}:\
ca=${TS_BLOCK}:\
*.tar=${TS_ORPHAN}:*.tgz=${TS_ORPHAN}:*.zip=${TS_ORPHAN}:*.gz=${TS_ORPHAN}:\
*.bz2=${TS_ORPHAN}:*.xz=${TS_ORPHAN}:*.zst=${TS_ORPHAN}:*.7z=${TS_ORPHAN}:\
*.rar=${TS_ORPHAN}:*.deb=${TS_ORPHAN}:*.rpm=${TS_ORPHAN}:\
*.jpg=${TS_READ}:*.jpeg=${TS_READ}:*.png=${TS_READ}:*.gif=${TS_READ}:\
*.webp=${TS_READ}:*.svg=${TS_READ}:*.mp4=${TS_READ}:*.mkv=${TS_READ}:\
*.mp3=${TS_READ}:*.flac=${TS_READ}:*.wav=${TS_READ}:\
*.md=${TS_YELLOW}:*.txt=${TS_YELLOW}:*.pdf=${TS_YELLOW}:*.adoc=${TS_YELLOW}:\
*.json=${TS_MUTED}:*.yaml=${TS_MUTED}:*.yml=${TS_MUTED}:*.toml=${TS_MUTED}:\
*.ini=${TS_MUTED}:*.conf=${TS_MUTED}:*.env=${TS_MUTED}:\
*.log=${TS_FAINT}:*.bak=${TS_FAINT}:*.tmp=${TS_FAINT}:*.swp=${TS_FAINT}:\
*~=${TS_FAINT}:*.orig=${TS_FAINT}:\
*Makefile=${TS_BRIGHT}:*Dockerfile=${TS_BRIGHT}:*README.md=1;${TS_BRIGHT}:\
"
export LS_COLORS

# eza reusa o LS_COLORS acima e acrescenta o seu próprio dicionário de campos.
EZA_COLORS="\
ur=${TS_MUTED}:uw=${TS_MUTED}:ux=${TS_BLOCK}:ue=${TS_BLOCK}:\
gr=${TS_FAINT}:gw=${TS_FAINT}:gx=${TS_FAINT}:\
tr=${TS_FAINT}:tw=${TS_FAINT}:tx=${TS_FAINT}:\
su=${TS_DEAD}:sf=${TS_DEAD}:xa=${TS_FAINT}:\
sn=${TS_DEFAULT}:sb=${TS_MUTED}:\
uu=${TS_BRIGHT}:un=${TS_MUTED}:\
gu=${TS_MUTED}:gn=${TS_FAINT}:\
da=${TS_MUTED}:\
lc=${TS_FLUX}:lm=${TS_MUTED}:\
ga=${TS_BLOCK}:gm=${TS_BLOCK}:gd=${TS_DEAD}:gv=${TS_ORPHAN}:gt=${TS_READ}:\
hd=1;${TS_BRIGHT}:\
punctuation=${TS_FAINT}:\
"
export EZA_COLORS

# NO_COLOR=1 ou terminal burro: sem cor nenhuma. A listagem continua legível
# pelos indicadores de tipo do próprio ls (-F: / @ * | =).
if [ -n "${NO_COLOR:-}" ] || [ "${TERM:-dumb}" = "dumb" ]; then
  unset LS_COLORS EZA_COLORS
  export EZA_COLORS=""
  alias ls='ls -F --color=never'
  alias eza='eza -F --color=never'
fi

unset TS_FLUX_CORE TS_FLUX TS_BLOCK TS_DEAD TS_ORPHAN TS_READ TS_YELLOW \
      TS_DEFAULT TS_MUTED TS_FAINT TS_BRIGHT TS_PURPLE
unset -f _ts_fg 2>/dev/null || true
