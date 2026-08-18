package engine

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/andreluiz/tesseract/internal/protocol"
)

func loadedEngine(b *testing.B, projects, cells int) *Engine {
	b.Helper()
	m := New(b.TempDir(), benchTestConfig())
	if err := m.Start(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(m.Shutdown)
	for p := range projects {
		dir := b.TempDir()
		_ = p
		for range cells {
			id, err := m.Create(protocol.Create{Path: dir, Type: "bash"})
			if err != nil {
				b.Fatal(err)
			}
			_ = m.Resize(id, 100, 24)
		}
	}
	return m
}

func benchTestConfig() Config {
	config := DefaultConfig()
	config.Sound, config.Notify = false, false
	config.HistoryLimit = 64 << 10
	return config
}

func BenchmarkSnapshot(b *testing.B) {
	m := loadedEngine(b, 3, 4)
	b.ReportAllocs()
	for b.Loop() {
		_ = m.Snapshot()
	}
}

func BenchmarkSnapshotPlusJSON(b *testing.B) {
	m := loadedEngine(b, 3, 4)
	encoder := json.NewEncoder(io.Discard)
	b.ReportAllocs()
	for b.Loop() {
		envelope, _ := protocol.Pack(protocol.TypeState, m.Snapshot())
		_ = encoder.Encode(envelope)
	}
}

func BenchmarkReadQuota(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = ReadQuota()
	}
}
