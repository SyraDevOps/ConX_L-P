package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"nhooyr.io/websocket"
)

func runServerWS() {
	peerID := "server-001"
	role := "server"
	ctx := context.Background()
	url := fmt.Sprintf("ws://localhost:8000/ws/%s/%s", peerID, role)
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		panic(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	fmt.Println("Conectado ao relay como servidor.")

	// Exemplo: enviar comandos manualmente
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Digite o comando para o cliente: ")
		if !scanner.Scan() {
			break
		}
		cmd := scanner.Text()
		if cmd == "" {
			continue
		}
		// Envia comando para o relay (que repassará ao cliente parceiro)
		conn.Write(ctx, websocket.MessageText, []byte(cmd))

		// Aguarda resposta do cliente
		ctxResp, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, resp, err := conn.Read(ctxResp)
		cancel()
		if err != nil {
			fmt.Println("Erro ao receber resposta:", err)
			continue
		}
		fmt.Println("Resposta do cliente:\n", string(resp))
	}
}
