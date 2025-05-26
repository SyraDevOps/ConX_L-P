package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"nhooyr.io/websocket"
)

type ClientInfo struct {
	Type   string `json:"type"`
	OS     string `json:"os"`
	Detail string `json:"detail"`
}

func main() {
	peerID := "client-001" // personalize para cada cliente
	role := "client"
	ctx := context.Background()
	url := fmt.Sprintf("ws://localhost:8000/ws/%s/%s", peerID, role)
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		panic(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Envia identificação (opcional, se quiser)
	info := ClientInfo{
		Type:   "camera",
		OS:     runtime.GOOS,
		Detail: "Câmera principal observatório",
	}
	infoBytes, _ := json.Marshal(info)
	conn.Write(ctx, websocket.MessageText, infoBytes)

	fmt.Println("Conectado ao relay, aguardando comandos...")

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			fmt.Println("Desconectado:", err)
			return
		}
		cmdLine := strings.TrimSpace(string(data))
		if cmdLine == "" {
			continue
		}
		fmt.Println("Comando recebido:", cmdLine)
		var output []byte
		if cmdLine == "getinfo" {
			output = []byte(fmt.Sprintf("Tipo: %s, OS: %s, Detalhe: %s", info.Type, info.OS, info.Detail))
		} else {
			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.Command("cmd", "/C", cmdLine)
			} else {
				cmd = exec.Command("sh", "-c", cmdLine)
			}
			out, _ := cmd.CombinedOutput()
			output = out
		}
		conn.Write(ctx, websocket.MessageText, output)
	}
}
