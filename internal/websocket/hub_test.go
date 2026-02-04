package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tasukuchiba/text_messaging_app/internal/storage"
)

func TestNewHub(t *testing.T) {
	store := storage.NewMemoryStorage()
	hub := NewHub(store)

	if hub == nil {
		t.Fatal("NewHub returned nil")
	}

	if hub.clients == nil {
		t.Error("clients map is nil")
	}

	if hub.broadcast == nil {
		t.Error("broadcast channel is nil")
	}

	if hub.register == nil {
		t.Error("register channel is nil")
	}

	if hub.unregister == nil {
		t.Error("unregister channel is nil")
	}

	if hub.storage != store {
		t.Error("storage not properly set")
	}
}

func TestHub_BroadcastMessage(t *testing.T) {
	store := storage.NewMemoryStorage()
	hub := NewHub(store)

	// Hubを別goroutineで起動
	go hub.Run()

	// テスト用のクライアントを作成（sendチャネルのみ）
	client := &Client{
		hub:    hub,
		send:   make(chan []byte, 256),
		sender: "test-receiver",
	}

	// クライアントを登録
	hub.register <- client

	// 少し待ってから登録を確認
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", hub.ClientCount())
	}

	// メッセージをブロードキャスト
	err := hub.BroadcastMessage("alice", "Hello, World!")
	if err != nil {
		t.Fatalf("BroadcastMessage failed: %v", err)
	}

	// メッセージを受信
	select {
	case msg := <-client.send:
		var outMsg OutgoingMessage
		if err := json.Unmarshal(msg, &outMsg); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}

		if outMsg.Type != "message" {
			t.Errorf("Expected type 'message', got '%s'", outMsg.Type)
		}

		if outMsg.Sender != "alice" {
			t.Errorf("Expected sender 'alice', got '%s'", outMsg.Sender)
		}

		if outMsg.Content != "Hello, World!" {
			t.Errorf("Expected content 'Hello, World!', got '%s'", outMsg.Content)
		}

		if outMsg.ID == "" {
			t.Error("Expected non-empty ID")
		}

	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// ストレージにも保存されていることを確認
	messages, err := store.GetAll()
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message in storage, got %d", len(messages))
	}

	if messages[0].Sender != "alice" {
		t.Errorf("Expected sender 'alice' in storage, got '%s'", messages[0].Sender)
	}

	if messages[0].Content != "Hello, World!" {
		t.Errorf("Expected content 'Hello, World!' in storage, got '%s'", messages[0].Content)
	}
}

func TestHub_RegisterUnregister(t *testing.T) {
	store := storage.NewMemoryStorage()
	hub := NewHub(store)

	go hub.Run()

	client1 := &Client{
		hub:    hub,
		send:   make(chan []byte, 256),
		sender: "client1",
	}

	client2 := &Client{
		hub:    hub,
		send:   make(chan []byte, 256),
		sender: "client2",
	}

	// クライアント1を登録
	hub.register <- client1
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("Expected 1 client after first register, got %d", hub.ClientCount())
	}

	// クライアント2を登録
	hub.register <- client2
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 2 {
		t.Errorf("Expected 2 clients after second register, got %d", hub.ClientCount())
	}

	// クライアント1を登録解除
	hub.unregister <- client1
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("Expected 1 client after unregister, got %d", hub.ClientCount())
	}

	// クライアント2を登録解除
	hub.unregister <- client2
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("Expected 0 clients after all unregister, got %d", hub.ClientCount())
	}
}

func TestOutgoingMessage_JSON(t *testing.T) {
	now := time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC)
	msg := OutgoingMessage{
		Type:      "message",
		ID:        "test-id",
		Sender:    "alice",
		Content:   "Hello!",
		CreatedAt: now,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var parsed OutgoingMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if parsed.Type != msg.Type {
		t.Errorf("Type mismatch: expected %s, got %s", msg.Type, parsed.Type)
	}

	if parsed.ID != msg.ID {
		t.Errorf("ID mismatch: expected %s, got %s", msg.ID, parsed.ID)
	}

	if parsed.Sender != msg.Sender {
		t.Errorf("Sender mismatch: expected %s, got %s", msg.Sender, parsed.Sender)
	}

	if parsed.Content != msg.Content {
		t.Errorf("Content mismatch: expected %s, got %s", msg.Content, parsed.Content)
	}
}

func TestIncomingMessage_JSON(t *testing.T) {
	jsonStr := `{"type":"message","content":"Hello!"}`

	var msg IncomingMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if msg.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", msg.Type)
	}

	if msg.Content != "Hello!" {
		t.Errorf("Expected content 'Hello!', got '%s'", msg.Content)
	}
}
