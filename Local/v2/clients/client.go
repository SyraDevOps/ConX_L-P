package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	network "github.com/libp2p/go-libp2p/core/network"
	peerstore "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

const protocolID = "/cmd/1.0.0"

type ClientInfo struct {
	Type   string `json:"type"`   // camera, sensor, anotador, etc
	OS     string `json:"os"`     // linux, windows, etc
	Detail string `json:"detail"` // info extra
}

func handleStream(s network.Stream, info ClientInfo) {
	// Envia identificação ao servidor
	idBytes, _ := json.Marshal(info)
	s.Write(append(idBytes, '\n'))

	reader := bufio.NewReader(s)
	for {
		cmdLine, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		cmdLine = strings.TrimSpace(cmdLine)
		if cmdLine == "" {
			continue
		}
		// Exemplo: comandos especiais por tipo
		if cmdLine == "getinfo" {
			s.Write([]byte(fmt.Sprintf("Tipo: %s, OS: %s, Detalhe: %s\x00", info.Type, info.OS, info.Detail)))
			continue
		}
		// Executa comando no SO
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
		s.Write(append(out.Bytes(), '\x00'))
	}
}

func main() {
	host, err := libp2p.New()
	if err != nil {
		panic(err)
	}

	// Troque pelo endereço do servidor
	addrStr := "/ip6/::1/tcp/31104/p2p/12D3KooWSUjGru7YgNT1d3saU3xf8NwCGviV46q7ygRZttQJ1iSd"
	addr, err := ma.NewMultiaddr(addrStr)
	if err != nil {
		panic(err)
	}
	info, err := peerstore.AddrInfoFromP2pAddr(addr)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := host.Connect(ctx, *info); err != nil {
		panic(err)
	}
	fmt.Println("✅ Conectado ao servidor:", info.ID)

	// Defina o tipo e detalhes do cliente aqui
	clientInfo := ClientInfo{
		Type:   "camera", // ou "sensor", "anotador", etc
		OS:     runtime.GOOS,
		Detail: "Câmera principal observatório",
	}

	stream, err := host.NewStream(context.Background(), info.ID, protocolID)
	if err != nil {
		panic(err)
	}
	handleStream(stream, clientInfo)
}
