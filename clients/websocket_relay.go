package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Cliente WebSocket para relay público
type RelayClient struct {
	conn   *websocket.Conn
	url    string
	active bool
}

var (
	relayClient *RelayClient
	// Default relay endpoint (will be fully configured during runtime)
	relayURLs = []string{}
)

// Prompt user to configure API relay settings
func promptAPIConfiguration() {
	var configureAPI string
	fmt.Print("⚙️ Deseja configurar a API relay? (s/n): ")
	fmt.Scanln(&configureAPI)

	if strings.ToLower(configureAPI) == "s" || strings.ToLower(configureAPI) == "sim" {
		// Get custom client ID
		var clientID string
		fmt.Print("🆔 Digite um ID para este cliente: ")
		fmt.Scanln(&clientID)

		if clientID == "" {
			clientID = fmt.Sprintf("client-%d", time.Now().UnixNano())
			fmt.Printf("🆔 Usando ID padrão gerado: %s\n", clientID)
		}

		// Optionally get custom API URL
		var customAPI string
		fmt.Print("🌐 Digite o endereço da API relay (deixe em branco para usar localhost): ")
		fmt.Scanln(&customAPI)

		baseURL := "ws://localhost:8000/ws/client"
		if customAPI != "" {
			baseURL = customAPI
			if !strings.HasSuffix(baseURL, "/") {
				baseURL += "/"
			}
			if !strings.HasSuffix(baseURL, "client/") {
				baseURL += "client/"
			}
		}

		// Construct final URL
		finalURL := baseURL + clientID
		relayURLs = []string{finalURL}
		fmt.Printf("✅ API configurada: %s\n", finalURL)
	} else {
		// Use default with generated ID
		clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
		relayURLs = []string{fmt.Sprintf("ws://localhost:8000/ws/client/%s", clientID)}
		fmt.Printf("🔧 Usando endereço padrão com ID gerado: %s\n", relayURLs[0])
	}
}

// Initialize relay configuration from flags or environment
func initRelayConfig() {
	// Check for command-line flag first
	relayFlag := flag.String("relay", "", "API relay server URL")
	flag.Parse()

	// Command-line flag takes precedence
	if *relayFlag != "" {
		relayURLs = []string{*relayFlag}
		fmt.Printf("🔧 Using relay URL from command-line: %s\n", *relayFlag)
		return
	}

	// Next check environment variable
	if envRelay := os.Getenv("API_RELAY_URL"); envRelay != "" {
		relayURLs = []string{envRelay}
		fmt.Printf("🔧 Using relay URL from environment: %s\n", envRelay)
		return
	}

	// If no relayURLs set yet, wait for user prompt
}

// Conecta via relay WebSocket
func connectViaRelay() bool {
	// Make sure configuration is loaded
	if appConfig.APIEndpoint == "" {
		LoadConfig()
	}

	// Get full endpoint from configuration
	fullEndpoint := GetFullAPIEndpoint()
	fmt.Printf("🌐 Conectando ao relay: %s\n", fullEndpoint)

	// Try connect with the configured endpoint
	if tryConnectToRelay(fullEndpoint) {
		// Remember this was successful
		appConfig.LastConnected = fullEndpoint
		SaveConfig()
		return true
	}

	return false
}

// Tenta conectar a um relay específico
func tryConnectToRelay(relayURL string) bool {
	fmt.Printf("🌐 Tentando conectar ao relay: %s\n", relayURL)

	u, err := url.Parse(relayURL)
	if err != nil {
		fmt.Printf("❌ URL inválida: %v\n", err)
		return false
	}

	// Client ID is now part of the URL, so no need to generate one here
	// Extract the client ID from the URL path for identification message
	urlPath := u.Path
	pathParts := strings.Split(urlPath, "/")
	clientId := pathParts[len(pathParts)-1]

	// Conecta ao WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Printf("❌ Falha ao conectar: %v\n", err)
		return false
	}

	relayClient = &RelayClient{
		conn:   conn,
		url:    relayURL,
		active: true,
	}

	// Envia identificação inicial com ID único
	identification := map[string]interface{}{
		"type":    "client",
		"id":      clientId,
		"info":    clientInfo,
		"command": "identify",
	}

	if err := relayClient.conn.WriteJSON(identification); err != nil {
		fmt.Printf("❌ Erro ao enviar identificação: %v\n", err)
		conn.Close()
		return false
	}

	// Inicia handlers
	go relayMessageHandler()
	go relayPingHandler()

	fmt.Printf("✅ Conectado ao relay: %s com ID: %s\n", relayURL, clientId)
	return true
}

// Handler principal de mensagens do relay
func relayMessageHandler() {
	defer func() {
		if relayClient != nil {
			relayClient.active = false
			relayClient.conn.Close()
		}
		setConnectionStatus(DISCONNECTED)
	}()

	for relayClient.active {
		var message map[string]interface{}
		err := relayClient.conn.ReadJSON(&message)
		if err != nil {
			fmt.Printf("❌ Erro ao ler mensagem do relay: %v\n", err)
			return
		}

		// Processa diferentes tipos de mensagem
		handleRelayMessage(message)
	}
}

// Processa mensagens recebidas do relay
func handleRelayMessage(message map[string]interface{}) {
	msgType, ok := message["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "command":
		// Comando para executar
		if cmd, ok := message["command"].(string); ok {
			fmt.Printf("🔧 Comando via relay: %s\n", cmd)
			executeCommandViaRelay(cmd)
		}

	case "module_download":
		// Download de módulo
		if moduleData, ok := message["module"].(map[string]interface{}); ok {
			handleModuleViaRelay(moduleData)
		}

	case "ping":
		// Responde ping
		response := map[string]interface{}{
			"type":      "pong",
			"timestamp": time.Now().Unix(),
		}
		relayClient.conn.WriteJSON(response)

	case "info_request":
		// Solicitação de informações
		response := map[string]interface{}{
			"type":            "info_response",
			"client_info":     clientInfo,
			"modules":         installedModules,
			"connection_type": "relay",
		}
		relayClient.conn.WriteJSON(response)
	}
}

// Executa comando recebido via relay
func executeCommandViaRelay(cmdLine string) {

	// Comandos especiais
	switch cmdLine {
	case "getinfo":
		response := fmt.Sprintf("Tipo: %s, OS: %s, Detalhe: %s, Módulos: %v",
			clientInfo.Type, clientInfo.OS, clientInfo.Detail, installedModules)
		sendRelayResponse("command_result", response)
		return

	case "listmodules":
		response := fmt.Sprintf("Módulos instalados: %v", installedModules)
		sendRelayResponse("command_result", response)
		return
	}

	// Executa comando no sistema
	executeCommand(cmdLine, &RelayWriter{})
}

// Implementa WebSocketWriter para relay
type RelayWriter struct{}

func (rw *RelayWriter) WriteMessage(message string) error {
	return sendRelayResponse("command_result", message)
}

// Envia resposta via relay
func sendRelayResponse(responseType, content string) error {
	if relayClient == nil || !relayClient.active {
		return fmt.Errorf("cliente relay não ativo")
	}

	response := map[string]interface{}{
		"type":      responseType,
		"content":   content,
		"timestamp": time.Now().Unix(),
	}

	return relayClient.conn.WriteJSON(response)
}

// Handler de módulo via relay
func handleModuleViaRelay(moduleData map[string]interface{}) {
	// Converte map para struct ModuleInfo
	jsonData, err := json.Marshal(moduleData)
	if err != nil {
		fmt.Printf("❌ Erro ao processar módulo: %v\n", err)
		return
	}

	var module ModuleInfo
	if err := json.Unmarshal(jsonData, &module); err != nil {
		fmt.Printf("❌ Erro ao decodificar módulo: %v\n", err)
		return
	}

	fmt.Printf("📥 Recebendo módulo via relay: %s (%s)\n", module.Name, module.Type)

	if err := installModule(module); err != nil {
		fmt.Printf("❌ Erro ao instalar %s: %v\n", module.Name, err)
		sendRelayResponse("module_error", fmt.Sprintf("Erro ao instalar %s: %v", module.Name, err))
	} else {
		fmt.Printf("✅ Módulo %s instalado com sucesso via relay\n", module.Name)
		sendRelayResponse("module_success", fmt.Sprintf("Módulo %s instalado", module.Name))
	}
}

// Mantém conexão ativa com pings
func relayPingHandler() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if relayClient != nil && relayClient.active {
			ping := map[string]interface{}{
				"type":      "ping",
				"timestamp": time.Now().Unix(),
			}

			if err := relayClient.conn.WriteJSON(ping); err != nil {
				fmt.Printf("❌ Erro ao enviar ping: %v\n", err)
				relayClient.active = false
				return
			}
		} else {
			// If client is not active or nil, stop the handler
			return
		}
	}
}

// Desconecta do relay
func disconnectRelay() {
	if relayClient != nil && relayClient.active {
		relayClient.active = false
		relayClient.conn.Close()
		relayClient = nil
		fmt.Println("📴 Desconectado do relay")
	}
}
