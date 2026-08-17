#!/usr/bin/env bash
# scripts/vitrine.sh
#
# Regera as imagens de docs/img a partir da tela de verdade: o desenho sai do
# mesmo código que roda no terminal, e vira SVG. Rode depois de mexer no
# mosaico, na paleta ou nos marcadores — senão a vitrine passa a mostrar um
# produto que não existe mais.
set -euo pipefail

raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$raiz"

bruto="$(mktemp -d)"
trap 'rm -rf "$bruto"' EXIT

VITRINE="$bruto" go test ./internal/tela/ -run TestVitrine -count=1 >/dev/null

mkdir -p docs/img
for tela in mosaico digitar; do
	python3 scripts/ansi-para-svg.py "$bruto/$tela.ansi" "docs/img/$tela.svg" "Tesseract — $tela"
done
