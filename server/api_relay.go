package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/libp2p/go-libp2p/core/host"
)

var (
	apiRelayClient *RelayClient
	// Default API URL that can be overridden
	apiURL = "ws://localhost:8000/ws/server/main-server"
)

// RelayClient for the server to connect to the API
type RelayClient struct {
	conn   *websocket.Conn
	url    string
	active bool
}

// Initialize API configuration from flags or environment
func initAPIConfig() {
	// Check for command-line flag first
	apiFlag := flag.String("api", "", "API relay server URL")
	flag.Parse()

	// Command-line flag takes precedence
	if *apiFlag != "" {
		apiURL = *apiFlag
		fmt.Printf("🔧 Using API URL from command-line: %s\n", apiURL)
		return
	}

	// Next check environment variable
	if envAPI := os.Getenv("API_RELAY_URL"); envAPI != "" {
		apiURL = envAPI
		fmt.Printf("🔧 Using API URL from environment: %s\n", apiURL)
		return
	}

	fmt.Printf("🔧 Using default API URL: %s\n", apiURL)
}

// Start relay connection in the background
func startAPIRelayConnection(h host.Host) {
	// Load configuration first
	LoadServerConfig()

	// If this is first run, prompt for configuration
	if serverConfig.FirstRun {
		PromptForServerConfiguration()
	}

	fmt.Println("🌐 Iniciando conexão com o servidor relay API...")

	// Use the endpoint from configuration
	apiURL = GetFullAPIEndpoint()

	// Try to connect initially
	connectToAPIRelay(h)

	// Keep trying to reconnect if disconnected
	go maintainAPIRelayConnection(h)
}

func connectToAPIRelay(h host.Host) bool {
	fmt.Printf("🌐 Conectando ao API relay: %s\n", apiURL)

	u, err := url.Parse(apiURL)
	if err != nil {
		fmt.Printf("❌ URL inválida: %v\n", err)
		return false
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Printf("❌ Falha ao conectar ao API relay: %v\n", err)
		return false
	}

	apiRelayClient = &RelayClient{
		conn:   conn,
		url:    apiURL,
		active: true,
	}

	// Send identification to the API
	identification := map[string]interface{}{
		"type":    "server",
		"command": "identify",
		"info": map[string]interface{}{
			"id":      h.ID().String(), // Pass host explicitly
			"type":    "p2p-server",
			"modules": len(modules),
		},
	}

	if err := apiRelayClient.conn.WriteJSON(identification); err != nil {
		fmt.Printf("❌ Erro ao enviar identificação: %v\n", err)
		conn.Close()
		return false
	}

	// Start message handler
	go handleAPIRelayMessages()
	go relayPingHandler()

	fmt.Printf("✅ Conectado ao API relay: %s\n", apiURL)
	return true
}

func maintainAPIRelayConnection(h host.Host) {
	for {
		time.Sleep(30 * time.Second)

		if apiRelayClient == nil || !apiRelayClient.active {
			connectToAPIRelay(h)
		}
	}
}

func handleAPIRelayMessages() {
	if apiRelayClient == nil || !apiRelayClient.active {
		return
	}

	for apiRelayClient.active {
		var message map[string]interface{}
		err := apiRelayClient.conn.ReadJSON(&message)
		if err != nil {
			fmt.Printf("❌ Erro na conexão com API relay: %v\n", err)
			apiRelayClient.active = false
			return
		}

		// Process relay messages
		processAPIRelayMessage(message)
	}
}

func processAPIRelayMessage(message map[string]interface{}) {
	msgType, ok := message["type"].(string)
	if !ok {
		return
	}

	fmt.Printf("📩 Mensagem recebida do relay: %s\n", msgType)

	switch msgType {
	case "client_connected":
		// A client was connected to this server via the relay
		fmt.Printf("🔗 Cliente conectado via relay\n")

		// Get client info if available
		if clientInfo, exists := message["client_info"].(map[string]interface{}); exists {
			clientType, _ := clientInfo["type"].(string)
			clientOS, _ := clientInfo["os"].(string)
			fmt.Printf("📱 Info do cliente: %s (%s)\n", clientType, clientOS)
		}

	case "command":
		// Process command from client via relay
		if cmd, exists := message["command"].(string); exists {
			// Handle remote commands here
			fmt.Printf("🔧 Comando via relay: %s\n", cmd)

			// Send response back
			response := map[string]interface{}{
				"type":    "command_result",
				"content": "Comando executado com sucesso", // Replace with actual result
			}
			apiRelayClient.conn.WriteJSON(response)
		}

	case "ping":
		// Respond to ping
		apiRelayClient.conn.WriteJSON(map[string]interface{}{
			"type":      "pong",
			"timestamp": time.Now().Unix(),
		})
	}
}

func relayPingHandler() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if apiRelayClient != nil && apiRelayClient.active {
			ping := map[string]interface{}{
				"type":      "ping",
				"timestamp": time.Now().Unix(),
			}

			if err := apiRelayClient.conn.WriteJSON(ping); err != nil {
				fmt.Printf("❌ Erro ao enviar ping para API: %v\n", err)
				apiRelayClient.active = false
				return
			}
		} else {
			return
		}
	}
}

// SendToClient sends a message to a specific client via the relay
func sendToClientViaRelay(clientID string, message map[string]interface{}) error {
	if apiRelayClient == nil || !apiRelayClient.active {
		return fmt.Errorf("sem conexão com o relay")
	}

	// Add routing information to the message
	message["target"] = clientID

	return apiRelayClient.conn.WriteJSON(message)
}
