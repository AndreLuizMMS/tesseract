package motor

import (
	"os"
	"path/filepath"
)

// DiretorioEstado é onde o motor guarda o estado desejado e os históricos.
func DiretorioEstado() string {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "tesseract")
	}
	casa, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "tesseract")
	}
	return filepath.Join(casa, ".local", "state", "tesseract")
}

// CaminhoSocket é o ponto de encontro entre a tela e o motor.
func CaminhoSocket() string {
	if base := os.Getenv("XDG_RUNTIME_DIR"); base != "" {
		return filepath.Join(base, "tesseract", "motor.sock")
	}
	return filepath.Join(DiretorioEstado(), "motor.sock")
}

// CaminhoEstado é o arquivo do estado desejado.
func CaminhoEstado(dirEstado string) string {
	return filepath.Join(dirEstado, "estado.json")
}

// CaminhoHistorico é o arquivo de histórico de uma célula.
func CaminhoHistorico(dirEstado, idCelula string) string {
	return filepath.Join(dirEstado, "historico", idCelula+".log")
}
