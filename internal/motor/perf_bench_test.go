package motor

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/andreluiz/tesseract/internal/protocolo"
)

func motorCarregado(b *testing.B, projetos, celulas int) *Motor {
	b.Helper()
	m := Novo(b.TempDir(), configuracaoDeTesteBench())
	if err := m.Iniciar(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(m.Encerrar)
	for p := range projetos {
		dir := b.TempDir()
		_ = p
		for range celulas {
			id, err := m.Criar(protocolo.Criar{Caminho: dir, Tipo: "bash"})
			if err != nil {
				b.Fatal(err)
			}
			_ = m.Tamanho(id, 100, 24)
		}
	}
	return m
}

func configuracaoDeTesteBench() Configuracao {
	config := ConfiguracaoPadrao()
	config.Som, config.Notificar = false, false
	config.TetoHistorico = 64 << 10
	return config
}

func BenchmarkRetrato(b *testing.B) {
	m := motorCarregado(b, 3, 4)
	b.ReportAllocs()
	for b.Loop() {
		_ = m.Retrato()
	}
}

func BenchmarkRetratoMaisJSON(b *testing.B) {
	m := motorCarregado(b, 3, 4)
	escritor := json.NewEncoder(io.Discard)
	b.ReportAllocs()
	for b.Loop() {
		envelope, _ := protocolo.Empacotar(protocolo.TipoEstado, m.Retrato())
		_ = escritor.Encode(envelope)
	}
}

func BenchmarkLerQuota(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = LerQuota()
	}
}
