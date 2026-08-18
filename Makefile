# Atalhos do dia a dia. O portão é sempre o mesmo: build, vet e testes.
.PHONY: gate build test instalar desinstalar limpar

gate:
	go build ./...
	go vet ./...
	go test ./...

build:
	go build -trimpath -ldflags='-s -w' -o ts ./cmd/tess

test:
	go test ./...

instalar:
	./install.sh

desinstalar:
	-systemctl --user disable --now tesseract.service
	rm -f $(HOME)/.config/systemd/user/tesseract.service
	rm -f $(HOME)/.local/bin/ts
	-systemctl --user daemon-reload

limpar:
	rm -f ts
