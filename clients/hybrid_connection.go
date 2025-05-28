package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Interface para escritores de conexão
type WebSocketWriter interface {
	WriteMessage(message string) error
}

var (
	connectionMutex sync.Mutex
)

func init() {
	// Mark functions as used to satisfy linter for U1000 errors,
	// assuming they are work-in-progress or conditionally used
	// in ways the linter might not fully detect in the current context.
	_ = isConnectionHealthy
	_ = disconnectFromRelay
	_ = disconnectFromLocal
}

// Gerenciador principal de conexões híbridas
func startHybridConnectionManager() {
	fmt.Println("🔗 Iniciando gerenciador de conexões híbridas...")

	for {
		connectionMutex.Lock()
		isConnected := connected
		connectionMutex.Unlock()

		if !isConnected {
			// Não está conectado, tenta estabelecer conexão
			attemptHybridConnection()
		}

		// Verifica conexão a cada minuto
		time.Sleep(60 * time.Second)
	}
}

// Tenta conectar usando estratégia híbrida
func attemptHybridConnection() {
	fmt.Println("🔄 Tentando estabelecer conexão híbrida...")

	// Use configuration's connection mode
	switch appConfig.ConnectionMode {
	case "local":
		if tryLocalConnection() {
			return
		}
		fmt.Println("⚠️ Conexão local falhou, deseja tentar configuração de relay?")
		var tryRelay string
		fmt.Print("Tentar relay? (s/n): ")
		fmt.Scanln(&tryRelay)

		if strings.ToLower(tryRelay) == "s" || strings.ToLower(tryRelay) == "sim" {
			// Force configuration prompt
			forceConfigPrompt = true
			PromptForConfiguration()
			tryPublicConnection()
		}

	case "public":
		// Make sure configuration is loaded first
		if appConfig.FirstRun {
			PromptForConfiguration()
		}

		if tryPublicConnection() {
			return
		}
		fmt.Println("⚠️ Conexão pública falhou, tentando local como backup...")
		tryLocalConnection()

	default: // "auto"
		// Try local first (faster and more secure)
		if tryLocalConnection() {
			return
		}

		// If local fails, prompt for API configuration
		fmt.Println("⚠️ Conexão local falhou, configurando conexão pública...")
		// Force configuration prompt
		forceConfigPrompt = true
		PromptForConfiguration()

		// Try public connection with new settings
		if tryPublicConnection() {
			return
		}

		fmt.Println("❌ Todas as tentativas de conexão falharam")
	}
}

// Tenta conexão P2P local
func tryLocalConnection() bool {
	if host == nil {
		fmt.Println("⚠️ Host P2P não inicializado")
		return false
	}

	fmt.Println("🏠 Tentando conexão P2P local...")
	success := connectWithRetry()

	if success {
		setConnectionStatus(LOCAL_P2P)
		fmt.Println("✅ Conectado via P2P local")
		return true
	}

	return false
}

// Tenta conexão via relay público
func tryPublicConnection() bool {
	fmt.Println("🌐 Tentando conexão via relay público...")
	success := connectViaRelay()

	if success {
		setConnectionStatus(PUBLIC_RELAY)
		fmt.Println("✅ Conectado via relay público")
		return true
	}

	return false
}

// Monitora qualidade da conexão e troca se necessário
func monitorConnectionQuality() {
	for {
		time.Sleep(5 * time.Minute)

		// Use GetConnectionStatus() instead of direct access
		currentConn := GetConnectionStatus()

		if currentConn == DISCONNECTED {
			continue
		}

		// Se está em conexão pública mas local ficou disponível, troca
		if currentConn == PUBLIC_RELAY && isLocalNetworkAvailable() {
			fmt.Println("🔄 Rede local disponível, migrando de público para local...")

			// Tenta conexão local
			if tryLocalConnection() {
				// Desconecta do relay público
				disconnectFromRelay()
			}
		}

		// Se está em local mas a qualidade está ruim, considera público
		if currentConn == LOCAL_P2P && !isConnectionHealthy() {
			fmt.Println("🔄 Conexão local instável, considerando migração para público...")

			if isPublicNetworkAvailable() {
				if tryPublicConnection() {
					// Desconecta do P2P local
					disconnectFromLocal()
				}
			}
		}
	}
}

// Verifica se a conexão atual está saudável
func isConnectionHealthy() bool {
	// Implementa verificação de saúde da conexão
	// Por exemplo: ping, latência, perda de pacotes
	return true // Simplificado por enquanto
}

// Desconecta do relay público
func disconnectFromRelay() {
	fmt.Println("📴 Desconectando do relay público...")
	// Implementar desconexão do WebSocket
}

// Desconecta do P2P local
func disconnectFromLocal() {
	fmt.Println("📴 Desconectando do P2P local...")
	// Implementar desconexão do libp2p
}

// Força troca de modo de conexão
func switchConnectionMode(newMode string) {
	connectionMutex.Lock()
	connectionMode = newMode
	connected = false
	currentConnection = DISCONNECTED
	connectionMutex.Unlock()

	fmt.Printf("🔄 Modo de conexão alterado para: %s\n", newMode)

	// Força nova tentativa de conexão
	go attemptHybridConnection()
}
