package main

import (
    "fmt"

    libp2p "github.com/libp2p/go-libp2p"
)

func main() {
    // Criar o host libp2p
    host, err := libp2p.New()
    if err != nil {
        panic(err)
    }

    fmt.Println("Peer ID:", host.ID().String())
    for _, addr := range host.Addrs() {
        fmt.Printf("Endereço: %s/p2p/%s\n", addr, host.ID())
    }

    // Mantém o programa rodando
    select {}
}
