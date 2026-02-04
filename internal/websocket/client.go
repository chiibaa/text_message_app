package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// 書き込み待機時間
	writeWait = 10 * time.Second

	// pongメッセージの待機時間
	pongWait = 60 * time.Second

	// ping送信間隔（pongWaitより短くする必要がある）
	pingPeriod = (pongWait * 9) / 10

	// 最大メッセージサイズ
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 開発環境用: 全てのオリジンを許可
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client は単一のWebSocket接続を表す
type Client struct {
	hub *Hub

	// WebSocket接続
	conn *websocket.Conn

	// 送信用バッファチャネル
	send chan []byte

	// ユーザー識別子
	sender string
}

// NewClient は新しいClientを作成する
func NewClient(hub *Hub, conn *websocket.Conn, sender string) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		sender: sender,
	}
}

// ReadPump はWebSocket接続からメッセージを読み取る
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// 受信メッセージをパース
		var inMsg IncomingMessage
		if err := json.Unmarshal(message, &inMsg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// メッセージタイプが"message"の場合のみ処理
		if inMsg.Type == "message" {
			if err := c.hub.BroadcastMessage(c.sender, inMsg.Content); err != nil {
				log.Printf("Failed to broadcast message: %v", err)
			}
		}
	}
}

// WritePump はWebSocket接続にメッセージを書き込む
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hubがチャネルをクローズした
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// キューに溜まったメッセージも送信
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWs はWebSocket接続をアップグレードしてクライアントを登録する
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	sender := r.URL.Query().Get("sender")
	if sender == "" {
		http.Error(w, "sender parameter is required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := NewClient(hub, conn, sender)
	client.hub.register <- client

	// goroutineで読み書きを並行実行
	go client.WritePump()
	go client.ReadPump()
}
