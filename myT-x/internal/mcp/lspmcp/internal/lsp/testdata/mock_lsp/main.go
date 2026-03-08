// mock_lsp はテスト用の最小限の LSP サーバー。initialize と shutdown に応答する。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var contentLen int
		if _, err := fmt.Sscanf(line, "Content-Length: %d", &contentLen); err == nil {
			// 空行まで読み飛ばす
			for {
				l, _ := reader.ReadString('\n')
				if strings.TrimSpace(l) == "" {
					break
				}
			}
			payload := make([]byte, contentLen)
			for i := 0; i < contentLen; i++ {
				b, err := reader.ReadByte()
				if err != nil {
					return
				}
				payload[i] = b
			}
			var req map[string]any
			if err := json.Unmarshal(payload, &req); err != nil {
				continue
			}
			method, _ := req["method"].(string)
			id := req["id"]
			var resp []byte
			switch method {
			case "initialize":
				res := map[string]any{
					"capabilities": map[string]any{
						"hoverProvider": true,
						"diagnosticProvider": map[string]any{
							"workspaceDiagnostics": true,
						},
					},
				}
				resp, _ = json.Marshal(map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"result":  res,
				})
			case "shutdown":
				resp, _ = json.Marshal(map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"result":  nil,
				})
			default:
				continue
			}
			fmt.Printf("Content-Length: %d\r\n\r\n%s", len(resp), resp)
		}
	}
}
