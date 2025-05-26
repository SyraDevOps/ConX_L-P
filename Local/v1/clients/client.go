package main

import (
	"context"
	"fmt"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	peerstore "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

func main() {
	// Criar novo host
	host, err := libp2p.New()
	if err != nil {
		panic(err)
	}

	// Endereço do peer remoto, pode ser qualquer um dos endereços
	addrStr := "/ip4/192.168.0.121/tcp/41579/p2p/12D3KooWNJ69F2fbAJjm84urJ9aK9Ja6beMBFYwT3EM9NvvB3dAQ"
	addr, err := ma.NewMultiaddr(addrStr)
	if err != nil {
		panic(err)
	}

	// Extraindo ID do peer e endereço
	info, err := peerstore.AddrInfoFromP2pAddr(addr)
	if err != nil {
		panic(err)
	}

	// Conectar ao peer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := host.Connect(ctx, *info); err != nil {
		panic(err)
	}

	fmt.Println("✅ Conectado com sucesso ao peer:", info.ID)
}
