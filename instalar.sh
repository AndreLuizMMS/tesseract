#!/usr/bin/env bash
# Instala o Tesseract: compila o comando `ts`, põe em ~/.local/bin e liga o
# serviço de usuário que mantém o motor de pé.
set -euo pipefail

raiz="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
destino="${HOME}/.local/bin"
servicos="${HOME}/.config/systemd/user"

echo "→ compilando"
mkdir -p "$destino"
(cd "$raiz" && go build -trimpath -o "$destino/ts" ./cmd/tess)

echo "→ instalando o serviço"
mkdir -p "$servicos"
cp "$raiz/systemd/tesseract.service" "$servicos/tesseract.service"

if command -v systemctl >/dev/null && systemctl --user show-environment >/dev/null 2>&1; then
  systemctl --user daemon-reload
  systemctl --user enable --now tesseract.service
  echo "→ serviço ligado"
else
  echo "! systemd de usuário indisponível: o motor sobe sozinho quando você rodar 'ts'"
fi

case ":$PATH:" in
  *":$destino:"*) ;;
  *) echo "! acrescente $destino ao PATH para o comando 'ts' funcionar de qualquer lugar" ;;
esac

echo
echo "pronto. rode 'ts' dentro de um projeto."
