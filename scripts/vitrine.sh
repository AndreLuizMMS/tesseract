#!/usr/bin/env bash
# scripts/vitrine.sh
#
# Regenerates the docs/img images from the real screen: the drawing comes
# from the same code that runs in the terminal, and turns into SVG. Run
# after touching the mosaic, the palette, or the markers — otherwise the
# showcase ends up displaying a product that no longer exists.
set -euo pipefail

raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$raiz"

bruto="$(mktemp -d)"
trap 'rm -rf "$bruto"' EXIT

VITRINE="$bruto" go test ./internal/tela/ -run TestVitrine -count=1 >/dev/null

mkdir -p docs/img
for tela in mosaico digitar; do
	python3 scripts/ansi-to-svg.py "$bruto/$tela.ansi" "docs/img/$tela.svg" "Tesseract — $tela"
done
