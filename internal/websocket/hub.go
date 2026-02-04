package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tasukuchiba/text_messaging_app/internal/models"
	"github.com/tasukuchiba/text_messaging_app/internal/storage"
)

// Hub は全WebSocketクライアントの接続を管理する
type Hub struct {
	// 接続中のクライアント
	clients map[*Client]bool

	// ブロードキャスト用チャネル
	broadcast chan []byte

	// クライアント登録用チャネル
	register chan *Client

	// クライアント登録解除用チャネル
	unregister chan *Client

	// メッセージ永続化用ストレージ
	storage storage.Storage
}

// IncomingMessage はクライアントから受信するメッセージの形式
type IncomingMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// OutgoingMessage はクライアントへ送信するメッセージの形式
type OutgoingMessage struct {
	Type      string    `json:"type"`
	ID        string    `json:"id"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// NewHub は新しいHubを作成する
func NewHub(store storage.Storage) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		storage:    store,
	}
}

// Run はHubのメインループを開始する
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Printf("Client registered: %s (total: %d)", client.sender, len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("Client unregistered: %s (total: %d)", client.sender, len(h.clients))
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// BroadcastMessage はメッセージを全クライアントにブロードキャストする
func (h *Hub) BroadcastMessage(sender, content string) error {
	msg := models.Message{
		ID:        uuid.New().String(),
		Sender:    sender,
		Content:   content,
		CreatedAt: time.Now(),
	}

	// ストレージに保存
	if err := h.storage.Save(msg); err != nil {
		log.Printf("Failed to save message: %v", err)
		return err
	}

	// 送信用メッセージを作成
	outMsg := OutgoingMessage{
		Type:      "message",
		ID:        msg.ID,
		Sender:    msg.Sender,
		Content:   msg.Content,
		CreatedAt: msg.CreatedAt,
	}

	data, err := json.Marshal(outMsg)
	if err != nil {
		return err
	}

	h.broadcast <- data
	return nil
}

// ClientCount は接続中のクライアント数を返す
func (h *Hub) ClientCount() int {
	return len(h.clients)
}
