package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	network "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// Estrutura para armazenar dados persistentes de clientes
type PersistentClientInfo struct {
	PeerID    string    `json:"peer_id"`
	Type      string    `json:"type"`
	OS        string    `json:"os"`
	Detail    string    `json:"detail"`
	Modules   []string  `json:"modules"`
	LastSeen  time.Time `json:"last_seen"`
	FirstSeen time.Time `json:"first_seen"`
	IPs       []string  `json:"ips"`
}

// Carrega o banco de dados de clientes persistente
func loadClientDB() map[string]PersistentClientInfo {
	db := make(map[string]PersistentClientInfo)

	// Cria diretório se não existir
	os.MkdirAll(filepath.Dir(clientDBFile), 0755)

	data, err := ioutil.ReadFile(clientDBFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("❌ Erro ao ler banco de dados de clientes: %v\n", err)
		}
		return db
	}

	if err := json.Unmarshal(data, &db); err != nil {
		fmt.Printf("❌ Erro ao decodificar banco de dados: %v\n", err)
		return db
	}

	fmt.Printf("📊 Carregados %d clientes do banco de dados\n", len(db))
	return db
}

// Salva o banco de dados de clientes
func saveClientDB(db map[string]PersistentClientInfo) {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		fmt.Printf("❌ Erro ao codificar banco de dados: %v\n", err)
		return
	}

	if err := ioutil.WriteFile(clientDBFile, data, 0644); err != nil {
		fmt.Printf("❌ Erro ao salvar banco de dados: %v\n", err)
	}
}

// Registra um cliente novo ou atualiza existente
func registerClient(peerID peer.ID, info ClientInfo, ip string) {
	// Carrega DB atual
	db := loadClientDB()

	peerStr := peerID.String()
	now := time.Now()

	// Verifica se cliente já existe
	existing, exists := db[peerStr]

	if exists {
		// Atualiza cliente existente
		existing.Type = info.Type
		existing.OS = info.OS
		existing.Detail = info.Detail
		existing.Modules = info.Modules
		existing.LastSeen = now

		// Adiciona IP se não estiver na lista
		hasIP := false
		for _, existingIP := range existing.IPs {
			if existingIP == ip {
				hasIP = true
				break
			}
		}
		if !hasIP {
			existing.IPs = append(existing.IPs, ip)
		}

		db[peerStr] = existing
	} else {
		// Registra novo cliente
		db[peerStr] = PersistentClientInfo{
			PeerID:    peerStr,
			Type:      info.Type,
			OS:        info.OS,
			Detail:    info.Detail,
			Modules:   info.Modules,
			FirstSeen: now,
			LastSeen:  now,
			IPs:       []string{ip},
		}
	}

	// Salva DB atualizado
	saveClientDB(db)
}

// Obtém IP do cliente remotamente
func getClientIP(s network.Stream) string {
	// Extrai endereço remoto
	addr := s.Conn().RemoteMultiaddr().String()

	// Simplificação - uma implementação mais robusta extrairia o IP real
	return addr
}

// Recupera clientes inativos para tentar reconexão
func getInactiveClients(days int) []PersistentClientInfo {
	db := loadClientDB()
	var inactive []PersistentClientInfo

	cutoff := time.Now().AddDate(0, 0, -days)

	for _, client := range db {
		if client.LastSeen.Before(cutoff) {
			inactive = append(inactive, client)
		}
	}

	return inactive
}

// Salva estado atual para sobreviver a reboots
func persistServerState() {
	// Salva estado atual em arquivo
	// Pode ser expandido para salvar mais informações

	// Backup automático do DB a cada hora
	for {
		time.Sleep(1 * time.Hour)

		// Converte clientes atuais para formato persistente
		clientsMu.Lock()
		persistentClients := make(map[string]PersistentClientInfo)

		for pid, sess := range clients {
			persistentClients[pid.String()] = PersistentClientInfo{
				PeerID:   pid.String(),
				Type:     sess.Info.Type,
				OS:       sess.Info.OS,
				Detail:   sess.Info.Detail,
				Modules:  sess.Info.Modules,
				LastSeen: time.Now(),
				IPs:      []string{getClientIP(sess.Stream)},
			}
		}
		clientsMu.Unlock()

		// Salva
		saveClientDB(persistentClients)
	}
}
