package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	libp2p "github.com/libp2p/go-libp2p"
	network "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

const protocolID = "/cmd/1.0.0"

type ClientInfo struct {
	Type   string `json:"type"`
	OS     string `json:"os"`
	Detail string `json:"detail"`
}

type ClientSession struct {
	Info   ClientInfo
	Stream network.Stream
}

var (
	clients   = make(map[peer.ID]ClientSession)
	clientsMu sync.Mutex
)

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
	fmt.Printf("Novo cliente: %s (%s, %s) conectado como %s\n", s.Conn().RemotePeer(), info.Type, info.OS, info.Detail)
}

func main() {
	host, err := libp2p.New()
	if err != nil {
		panic(err)
	}

	fmt.Println("Peer ID:", host.ID().String())
	for _, addr := range host.Addrs() {
		fmt.Printf("Endereço: %s/p2p/%s\n", addr, host.ID())
	}

	host.SetStreamHandler(protocolID, handleStream)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("\nClientes conectados:")
		clientsMu.Lock()
		for pid, sess := range clients {
			fmt.Printf("%s - %s (%s) - %s\n", pid, sess.Info.Type, sess.Info.OS, sess.Info.Detail)
		}
		clientsMu.Unlock()
		fmt.Print("Digite o PeerID do cliente e o comando (ex: <peerid> ls): ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()
		parts := strings.SplitN(input, " ", 2)
		if len(parts) != 2 {
			fmt.Println("Formato inválido.")
			continue
		}
		peerIDStr, cmd := parts[0], parts[1]
		pid, err := peer.Decode(peerIDStr)
		if err != nil {
			fmt.Println("PeerID inválido:", err)
			continue
		}
		clientsMu.Lock()
		sess, ok := clients[pid]
		clientsMu.Unlock()
		if !ok {
			fmt.Println("Cliente não encontrado.")
			continue
		}
		_, err = sess.Stream.Write([]byte(cmd + "\n"))
		if err != nil {
			fmt.Println("Erro ao enviar comando:", err)
			continue
		}
		resp := bufio.NewReader(sess.Stream)
		output, _ := resp.ReadString('\x00')
		fmt.Println("Saída do cliente:\n", strings.TrimSuffix(output, "\x00"))
	}
}
