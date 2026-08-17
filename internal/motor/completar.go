package motor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/andreluiz/tesseract/internal/protocolo"
)

// Completar resolve o que o usuário digitou até agora num caminho e diz quantos
// candidatos casam — é o que o `tab` do formulário usa. Um candidato só é
// completado inteiro; vários param no prefixo comum.
func Completar(pedido protocolo.Completar) protocolo.Completado {
	bruto := expandirCasa(pedido.Caminho)
	pasta, comeco := filepath.Split(bruto)
	if pasta == "" {
		pasta = "."
	}

	entradas, err := os.ReadDir(pasta)
	if err != nil {
		return protocolo.Completado{Caminho: pedido.Caminho}
	}

	var candidatos []string
	for _, entrada := range entradas {
		nome := entrada.Name()
		if !strings.HasPrefix(nome, comeco) {
			continue
		}
		if comeco == "" && strings.HasPrefix(nome, ".") {
			continue // pasta escondida só aparece quando o usuário pede
		}
		if pedido.SoDiretorio && !ehDiretorio(filepath.Join(pasta, nome)) {
			continue
		}
		candidatos = append(candidatos, nome)
	}
	if len(candidatos) == 0 {
		return protocolo.Completado{Caminho: pedido.Caminho}
	}

	escolhido := prefixoComum(candidatos)
	completo := filepath.Join(pasta, escolhido)
	if len(candidatos) == 1 && ehDiretorio(completo) {
		completo += string(os.PathSeparator)
	}
	return protocolo.Completado{Caminho: completo, Quantidade: len(candidatos)}
}

// prefixoComum é até onde dá para completar sem escolher por conta própria.
func prefixoComum(nomes []string) string {
	comum := nomes[0]
	for _, nome := range nomes[1:] {
		for !strings.HasPrefix(nome, comum) {
			comum = comum[:len(comum)-1]
			if comum == "" {
				return ""
			}
		}
	}
	return comum
}

func ehDiretorio(caminho string) bool {
	info, err := os.Stat(caminho)
	return err == nil && info.IsDir()
}

// expandirCasa troca o ~ pelo diretório da conta.
func expandirCasa(caminho string) string {
	if !strings.HasPrefix(caminho, "~") {
		return caminho
	}
	casa, err := os.UserHomeDir()
	if err != nil {
		return caminho
	}
	return filepath.Join(casa, strings.TrimPrefix(caminho, "~"))
}
