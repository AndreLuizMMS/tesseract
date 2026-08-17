package motor

import (
	"os/exec"
	"strings"
)

// O aviso sai do motor, não da tela. É por isso que o usuário é avisado mesmo
// com a tela fechada — o que o fork não fazia.
const tituloDoAviso = "Tesseract"

// anunciar toca o aviso e sobe a notificação do sistema. Os dois são
// desligáveis, separadamente, e nenhum dos dois espera resposta: notificador é
// programa de fora e pode demorar a subir.
func (m *Motor) anunciar(corpo string) {
	m.mu.Lock()
	som, notificar, comando := m.config.Som, m.config.Notificar, m.config.ComandoNotificacao
	m.mu.Unlock()

	if som {
		soltar(comandoDeSom())
	}
	if notificar {
		soltar(comandoDeNotificacao(comando, tituloDoAviso, corpo))
	}
}

// comandoDeSom é o aviso sonoro. Na WSL não há daemon de som, então quem apita
// é o Windows do outro lado.
func comandoDeSom() *exec.Cmd {
	if caminho, err := exec.LookPath("powershell.exe"); err == nil {
		return exec.Command(caminho, "-NoProfile", "-Command", "[console]::beep(880,180)")
	}
	if caminho, err := exec.LookPath("paplay"); err == nil {
		return exec.Command(caminho, "/usr/share/sounds/freedesktop/stereo/message.oga")
	}
	return nil
}

// comandoDeNotificacao monta o comando que sobe a notificação do sistema.
// Devolve nil quando a máquina não tem nenhum notificador — falta de
// notificador não é erro que valha mostrar.
//
// custom, quando preenchido, é uma linha de comando cujo programa recebe o
// título e o corpo como os dois últimos argumentos.
func comandoDeNotificacao(custom, titulo, corpo string) *exec.Cmd {
	if campos := strings.Fields(custom); len(campos) > 0 {
		return exec.Command(campos[0], append(campos[1:], titulo, corpo)...)
	}
	// wsl-notify-send faz a ponte para o toast do Windows; notify-send é o
	// padrão do Linux quando há daemon.
	for _, nome := range []string{"wsl-notify-send.exe", "notify-send"} {
		if caminho, err := exec.LookPath(nome); err == nil {
			return exec.Command(caminho, titulo, corpo)
		}
	}
	if caminho, err := exec.LookPath("powershell.exe"); err == nil {
		return exec.Command(caminho, "-NoProfile", "-Command", scriptDeToast(titulo, corpo))
	}
	return nil
}

// scriptDeToast é o caminho que sempre existe na WSL: o PowerShell do Windows
// mostrando um balão com o nome da célula e do projeto.
func scriptDeToast(titulo, corpo string) string {
	return strings.Join([]string{
		"[reflection.assembly]::LoadWithPartialName('System.Windows.Forms') | Out-Null;",
		"$aviso = New-Object System.Windows.Forms.NotifyIcon;",
		"$aviso.Icon = [System.Drawing.SystemIcons]::Information;",
		"$aviso.BalloonTipTitle = " + comoLiteral(titulo) + ";",
		"$aviso.BalloonTipText = " + comoLiteral(corpo) + ";",
		"$aviso.Visible = $true;",
		"$aviso.ShowBalloonTip(6000);",
		"Start-Sleep -Seconds 7;",
		"$aviso.Dispose()",
	}, " ")
}

// comoLiteral escreve o texto como literal do PowerShell, escapando o que
// fecharia a aspa.
func comoLiteral(texto string) string {
	return "'" + strings.ReplaceAll(texto, "'", "''") + "'"
}

// soltar dispara o comando sem esperar: o aviso nunca segura o motor.
func soltar(comando *exec.Cmd) {
	if comando == nil {
		return
	}
	if err := comando.Start(); err != nil {
		return
	}
	go func() { _ = comando.Wait() }()
}
