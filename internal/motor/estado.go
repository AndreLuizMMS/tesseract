package motor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// CelulaSalva é a intenção guardada de uma célula: o que ela é, não o processo
// que ela estava rodando. O processo morre; isto fica.
type CelulaSalva struct {
	ID       string `json:"id"`
	Tipo     string `json:"tipo"`
	Nome     string `json:"nome"`
	Alvo     string `json:"alvo,omitempty"`
	Conversa string `json:"conversa,omitempty"`
}

// ProjetoSalvo é uma coluna do mosaico guardada em disco.
type ProjetoSalvo struct {
	ID      string        `json:"id"`
	Caminho string        `json:"caminho"`
	Cor     int           `json:"cor"`
	Celulas []CelulaSalva `json:"celulas"`
}

// EstadoDesejado é a grade inteira como o usuário a deixou.
type EstadoDesejado struct {
	Projetos []ProjetoSalvo `json:"projetos"`
	Tela     string         `json:"tela,omitempty"` // mosaico ou lista
}

// Salvar grava o estado desejado atomicamente: escreve ao lado e renomeia por
// cima, para que uma queda no meio da escrita nunca deixe um arquivo pela
// metade.
func Salvar(caminho string, estado EstadoDesejado) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return err
	}
	conteudo, err := json.MarshalIndent(estado, "", "  ")
	if err != nil {
		return err
	}
	conteudo = append(conteudo, '\n')

	temporario, err := os.CreateTemp(filepath.Dir(caminho), filepath.Base(caminho)+".escrevendo-*")
	if err != nil {
		return err
	}
	nomeTemporario := temporario.Name()
	if _, err := temporario.Write(conteudo); err != nil {
		temporario.Close()
		os.Remove(nomeTemporario)
		return err
	}
	if err := temporario.Sync(); err != nil {
		temporario.Close()
		os.Remove(nomeTemporario)
		return err
	}
	if err := temporario.Close(); err != nil {
		os.Remove(nomeTemporario)
		return err
	}
	if err := os.Chmod(nomeTemporario, 0o644); err != nil {
		os.Remove(nomeTemporario)
		return err
	}
	if err := os.Rename(nomeTemporario, caminho); err != nil {
		os.Remove(nomeTemporario)
		return err
	}
	return nil
}

// Carregar lê o estado desejado. Quando o arquivo está corrompido, carrega o
// que der, preserva o original com sufixo e devolve o erro para a tela avisar.
func Carregar(caminho string) (EstadoDesejado, error) {
	conteudo, err := os.ReadFile(caminho)
	if errors.Is(err, os.ErrNotExist) {
		return EstadoDesejado{}, nil
	}
	if err != nil {
		return EstadoDesejado{}, err
	}

	var bruto struct {
		Projetos []json.RawMessage `json:"projetos"`
		Tela     string            `json:"tela"`
	}
	if err := json.Unmarshal(conteudo, &bruto); err != nil {
		preservado, erroPreservar := preservar(caminho)
		if erroPreservar != nil {
			return EstadoDesejado{}, fmt.Errorf("estado ilegível e não foi possível preservá-lo: %w", erroPreservar)
		}
		return EstadoDesejado{}, fmt.Errorf("estado ilegível, o arquivo anterior está em %s: %w", preservado, err)
	}

	estado := EstadoDesejado{Tela: bruto.Tela}
	perdidos := 0
	for _, cru := range bruto.Projetos {
		var projeto ProjetoSalvo
		if err := json.Unmarshal(cru, &projeto); err != nil || projeto.Caminho == "" {
			perdidos++
			continue
		}
		estado.Projetos = append(estado.Projetos, projeto)
	}
	if perdidos > 0 {
		preservado, erroPreservar := preservar(caminho)
		if erroPreservar != nil {
			return estado, fmt.Errorf("%d projeto(s) ilegível(is) e não foi possível preservar o arquivo: %w", perdidos, erroPreservar)
		}
		return estado, fmt.Errorf("%d projeto(s) ilegível(is) foram descartados, o arquivo anterior está em %s", perdidos, preservado)
	}
	return estado, nil
}

// preservar guarda uma cópia do arquivo problemático com sufixo, sem
// sobrescrever preservações anteriores.
func preservar(caminho string) (string, error) {
	for i := 0; ; i++ {
		destino := caminho + ".corrompido"
		if i > 0 {
			destino += "-" + strconv.Itoa(i)
		}
		if _, err := os.Stat(destino); err == nil {
			continue
		}
		conteudo, err := os.ReadFile(caminho)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(destino, conteudo, 0o644); err != nil {
			return "", err
		}
		return destino, nil
	}
}
