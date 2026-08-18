package motor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/andreluiz/tesseract/internal/protocolo"
)

// quotaVelhaDepoisDe limita a idade do arquivo de consumo. O Claude Code só
// entrega esse número enquanto uma sessão está desenhando o statusline, então
// arquivo velho quer dizer que faz tempo que nada roda — e o número não vale
// mais nada.
const quotaVelhaDepoisDe = 15 * time.Minute

// arquivosDeQuota são os lugares onde o statusline do usuário deixa o consumo
// da janela de 5 horas, na ordem de preferência.
var arquivosDeQuota = []string{"tesseract-quota.json", "squad-quota.json"}

// validadeDaQuota é por quanto tempo o número lido serve. O retrato do motor
// sai até 25 vezes por segundo, e o consumo muda de minuto em minuto: ir ao
// disco a cada quadro é trabalho jogado fora.
const validadeDaQuota = 2 * time.Second

var quotaGuardada struct {
	sync.Mutex
	valor *protocolo.Quota
	lida  time.Time
}

// LerQuota devolve o consumo da janela de 5 horas, ou nil quando o dado não
// existe ou está velho. Sem o arquivo, tudo funciona igual — só não aparece o
// badge.
func LerQuota() *protocolo.Quota {
	agora := time.Now()
	quotaGuardada.Lock()
	defer quotaGuardada.Unlock()
	if agora.Sub(quotaGuardada.lida) < validadeDaQuota {
		return quotaGuardada.valor
	}
	quotaGuardada.valor, quotaGuardada.lida = lerQuotaDoDisco(), agora
	return quotaGuardada.valor
}

func lerQuotaDoDisco() *protocolo.Quota {
	casa, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	for _, nome := range arquivosDeQuota {
		if quota := lerArquivoDeQuota(filepath.Join(casa, ".claude", nome)); quota != nil {
			return quota
		}
	}
	return nil
}

func lerArquivoDeQuota(caminho string) *protocolo.Quota {
	info, err := os.Stat(caminho)
	if err != nil || time.Since(info.ModTime()) > quotaVelhaDepoisDe {
		return nil
	}
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		return nil
	}

	var bruto struct {
		UsedPercentage float64 `json:"used_percentage"`
		ResetsAt       int64   `json:"resets_at"`
	}
	if err := json.Unmarshal(conteudo, &bruto); err != nil {
		return nil
	}

	quota := &protocolo.Quota{Percentual: min(max(int(bruto.UsedPercentage+0.5), 0), 100)}
	if bruto.ResetsAt > 0 {
		quota.Vira = quantoFalta(time.Until(time.Unix(bruto.ResetsAt, 0)))
	}
	return quota
}

// quantoFalta escreve o tempo até a janela virar como h:mm.
func quantoFalta(falta time.Duration) string {
	if falta <= 0 {
		return "0:00"
	}
	horas := int(falta.Hours())
	minutos := int(falta.Minutes()) % 60
	texto := strconv.Itoa(horas) + ":"
	if minutos < 10 {
		texto += "0"
	}
	return texto + strconv.Itoa(minutos)
}
