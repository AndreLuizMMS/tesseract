// Package historico guarda tudo o que uma célula produziu, em arquivo próprio,
// com teto de tamanho e descarte do começo. É o que sustenta a rolagem e a
// busca depois de uma queda da WSL.
package historico

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TetoPadrao é o tamanho máximo de um histórico de célula. Passou disso, o
// começo é descartado e o fim é preservado.
const TetoPadrao int64 = 5 << 20

// sobra é a fatia do teto que continua no arquivo depois de um descarte.
// Descartar só o excedente faria o corte acontecer a cada escrita.
const sobra = 3.0 / 4.0

// Historico é o arquivo de uma célula.
type Historico struct {
	mu      sync.Mutex
	caminho string
	teto    int64
	arquivo *os.File
	tamanho int64
}

// Ocorrencia é uma linha do histórico que casou com a busca.
type Ocorrencia struct {
	Linha int    // numeração a partir de 1, no conteúdo atual do arquivo
	Texto string // já sem os códigos de terminal
}

// Abrir prepara o histórico da célula, continuando o arquivo que já existir.
func Abrir(caminho string, teto int64) (*Historico, error) {
	if teto <= 0 {
		teto = TetoPadrao
	}
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return nil, err
	}
	arquivo, err := os.OpenFile(caminho, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	info, err := arquivo.Stat()
	if err != nil {
		arquivo.Close()
		return nil, err
	}
	return &Historico{caminho: caminho, teto: teto, arquivo: arquivo, tamanho: info.Size()}, nil
}

// Write grava o que a célula produziu e corta o começo quando o teto estoura.
func (h *Historico) Write(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	n, err := h.arquivo.Write(p)
	h.tamanho += int64(n)
	if err != nil {
		return n, err
	}
	if h.tamanho > h.teto {
		if err := h.descartarComeco(); err != nil {
			return n, err
		}
	}
	return n, nil
}

// descartarComeco reescreve o arquivo mantendo apenas o fim. Chamado já com o
// cadeado na mão.
func (h *Historico) descartarComeco() error {
	manter := int64(float64(h.teto) * sobra)
	origem, err := os.Open(h.caminho)
	if err != nil {
		return err
	}
	defer origem.Close()
	if _, err := origem.Seek(h.tamanho-manter, io.SeekStart); err != nil {
		return err
	}

	temporario := h.caminho + ".cortando"
	destino, err := os.Create(temporario)
	if err != nil {
		return err
	}
	copiado, err := io.Copy(destino, origem)
	if cerr := destino.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(temporario)
		return err
	}
	if err := os.Rename(temporario, h.caminho); err != nil {
		os.Remove(temporario)
		return err
	}

	novo, err := os.OpenFile(h.caminho, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	h.arquivo.Close()
	h.arquivo = novo
	h.tamanho = copiado
	return nil
}

// Cauda devolve os últimos n bytes gravados, que é o que o motor reproduz na
// tela da célula quando ela renasce depois de uma queda.
func (h *Historico) Cauda(n int64) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if n > h.tamanho {
		n = h.tamanho
	}
	if n <= 0 {
		return nil, nil
	}
	buf := make([]byte, n)
	if _, err := h.arquivo.ReadAt(buf, h.tamanho-n); err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}

// Buscar varre o histórico e devolve as linhas que contêm o termo, com o
// número da linha. A comparação ignora maiúsculas.
func (h *Historico) Buscar(termo string) ([]Ocorrencia, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if termo == "" {
		return nil, nil
	}
	arquivo, err := os.Open(h.caminho)
	if err != nil {
		return nil, err
	}
	defer arquivo.Close()

	alvo := strings.ToLower(termo)
	var achados []Ocorrencia
	leitor := bufio.NewScanner(arquivo)
	leitor.Buffer(make([]byte, 0, 64<<10), 1<<20)
	numero := 0
	for leitor.Scan() {
		numero++
		texto := LimparCodigos(leitor.Text())
		if strings.Contains(strings.ToLower(texto), alvo) {
			achados = append(achados, Ocorrencia{Linha: numero, Texto: texto})
		}
	}
	if err := leitor.Err(); err != nil {
		return achados, fmt.Errorf("lendo %s: %w", h.caminho, err)
	}
	return achados, nil
}

// Linhas conta quantas linhas o histórico tem agora, que é o universo em que a
// busca numera o que encontra.
func (h *Historico) Linhas() (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	arquivo, err := os.Open(h.caminho)
	if err != nil {
		return 0, err
	}
	defer arquivo.Close()

	leitor := bufio.NewScanner(arquivo)
	leitor.Buffer(make([]byte, 0, 64<<10), 1<<20)
	total := 0
	for leitor.Scan() {
		total++
	}
	return total, leitor.Err()
}

// Tamanho é quanto o histórico ocupa agora.
func (h *Historico) Tamanho() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.tamanho
}

// Fechar solta o arquivo.
func (h *Historico) Fechar() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.arquivo.Close()
}

// LimparCodigos tira do texto os códigos de controle do terminal, deixando só
// o que uma pessoa leria na tela.
func LimparCodigos(bruto string) string {
	var limpo strings.Builder
	limpo.Grow(len(bruto))
	for i := 0; i < len(bruto); {
		c := bruto[i]
		switch {
		case c == 0x1b && i+1 < len(bruto) && bruto[i+1] == '[':
			// Sequência CSI: termina no primeiro byte de 0x40 a 0x7e.
			i += 2
			for i < len(bruto) && (bruto[i] < 0x40 || bruto[i] > 0x7e) {
				i++
			}
			i++
		case c == 0x1b && i+1 < len(bruto) && (bruto[i+1] == ']' || bruto[i+1] == 'P' || bruto[i+1] == '^' || bruto[i+1] == '_'):
			// Sequência de string: termina em BEL ou ESC \.
			i += 2
			for i < len(bruto) {
				if bruto[i] == 0x07 {
					i++
					break
				}
				if bruto[i] == 0x1b && i+1 < len(bruto) && bruto[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		case c == 0x1b:
			i += 2
		case c == '\r':
			i++
		case c < 0x20 && c != '\t':
			i++
		default:
			limpo.WriteByte(c)
			i++
		}
	}
	return limpo.String()
}
