package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// Lista de servidores de backup para conexão
var (
	serverAddresses = []string{
		// Lista de endereços de servidor primário e fallbacks
		"/ip6/::1/udp/54192/quic-v1/webtransport/certhash/uEiDaJbtDnxcnFrd2Iv9N_7spPad04MRK9AwA2AoROSf28A/certhash/uEiCAtFWZcpwI4oIP_ZIvayr79oJ4p3hFq0F7ktmpXeI-ng/p2p/12D3KooWDhUF5WduWM3FyUkj1jWVJRPoC5hzUqdoLjDXoEGrkKTt",
		// Adicionar outros servidores de backup
		"/dns4/backup1.example.com/tcp/443/wss/p2p/12D3KooWXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
		"/dns4/backup2.example.com/tcp/443/wss/p2p/12D3KooWYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY",
	}

	// Lista de endpoints de API relay
	relayEndpoints = []string{} // Empty this list to avoid trying non-existent servers
)

// Tenta conectar a qualquer servidor disponível
func connectWithRetry() bool {
	// Randomiza a ordem dos servidores para distribuir conexões
	rand.Seed(time.Now().UnixNano())
	shuffleAddresses()

	// Loop contínuo até conseguir conexão
	for {
		// Tenta cada servidor na lista
		for _, addrStr := range serverAddresses {
			success := tryConnect(addrStr)
			if success {
				return true
			}
		}

		// Se nenhum funcionou, tenta conectar via relay
		if connectViaRelay() {
			return true
		}

		// Espera com jitter aleatório entre tentativas (5-30 segundos)
		sleepTime := 5000 + rand.Intn(25000)
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
	}
	// This line is unreachable because the loop above is infinite
	// and only exits by returning true.
	// return false
}

// Randomiza a ordem dos servidores
func shuffleAddresses() {
	rand.Shuffle(len(serverAddresses), func(i, j int) {
		serverAddresses[i], serverAddresses[j] = serverAddresses[j], serverAddresses[i]
	})
	rand.Shuffle(len(relayEndpoints), func(i, j int) {
		relayEndpoints[i], relayEndpoints[j] = relayEndpoints[j], relayEndpoints[i]
	})
}

// Tenta conexão com um servidor p2p
func tryConnect(addrStr string) bool {
	if host == nil {
		fmt.Println("⚠️ Host P2P não inicializado")
		return false
	}
	// Ignora erros de parsing com servidor inválido
	addr, err := ma.NewMultiaddr(addrStr)
	if err != nil {
		return false
	}

	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return false
	}

	// Timeout curto para não travar muito tempo
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Tenta conectar
	if err := host.Connect(ctx, *info); err != nil {
		return false
	}

	// Conectado com sucesso
	fmt.Println("✅ Conectado ao servidor:", info.ID)

	// Configura stream e inicia handler
	stream, err := host.NewStream(context.Background(), info.ID, protocolID)
	if err != nil {
		return false
	}

	go handleStreamWithInfo(stream)
	return true
}

// Tenta conexão via relay WebSocket
func tryRelayConnect(relayURL string) bool {
	// Instead of a single URL, try the connectViaRelay function
	// which uses the properly configured relayURLs from websocket_relay.go
	return connectViaRelay()
}

// Monitora conexão e reconecta se perdida
func monitorConnection() {
	for {
		// Verifica a cada minuto com jitter
		jitter := rand.Intn(30000)
		time.Sleep(time.Duration(60000+jitter) * time.Millisecond)
	}
}

// Wrapper para handleStream que usa clientInfo global
func handleStreamWithInfo(s network.Stream) {
	handleStream(s, clientInfo)
}
