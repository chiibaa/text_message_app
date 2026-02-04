package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tasukuchiba/text_messaging_app/internal/storage"
)

func TestServeWs_MissingSender(t *testing.T) {
	store := storage.NewMemoryStorage()
	hub := NewHub(store)
	go hub.Run()

	req := httptest.NewRequest("GET", "/ws", nil)
	w := httptest.NewRecorder()

	ServeWs(hub, w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestServeWs_Connection(t *testing.T) {
	store := storage.NewMemoryStorage()
	hub := NewHub(store)
	go hub.Run()

	// テスト用HTTPサーバーを作成
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer server.Close()

	// HTTP URLをWebSocket URLに変換
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?sender=alice"

	// WebSocket接続
	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("Expected status %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}

	// 接続後、Hubにクライアントが登録されていることを確認
	time.Sleep(100 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", hub.ClientCount())
	}
}

func TestClient_MessageFlow(t *testing.T) {
	store := storage.NewMemoryStorage()
	hub := NewHub(store)
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// クライアント1 (alice) を接続
	dialer := websocket.Dialer{}
	conn1, _, err := dialer.Dial(wsURL+"?sender=alice", nil)
	if err != nil {
		t.Fatalf("Failed to connect alice: %v", err)
	}
	defer conn1.Close()

	// クライアント2 (bob) を接続
	conn2, _, err := dialer.Dial(wsURL+"?sender=bob", nil)
	if err != nil {
		t.Fatalf("Failed to connect bob: %v", err)
	}
	defer conn2.Close()

	// 両方のクライアントが登録されるのを待つ
	time.Sleep(100 * time.Millisecond)

	if hub.ClientCount() != 2 {
		t.Errorf("Expected 2 clients, got %d", hub.ClientCount())
	}

	// aliceからメッセージを送信
	message := `{"type":"message","content":"Hello from Alice!"}`
	if err := conn1.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// bobがメッセージを受信することを確認
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	if !strings.Contains(string(msg), "Hello from Alice!") {
		t.Errorf("Expected message to contain 'Hello from Alice!', got '%s'", string(msg))
	}

	if !strings.Contains(string(msg), `"sender":"alice"`) {
		t.Errorf("Expected message to contain sender 'alice', got '%s'", string(msg))
	}

	// aliceも自分のメッセージを受信することを確認
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err = conn1.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read own message: %v", err)
	}

	if !strings.Contains(string(msg), "Hello from Alice!") {
		t.Errorf("Expected alice to receive own message")
	}

	// ストレージにメッセージが保存されていることを確認
	messages, err := store.GetAll()
	if err != nil {
		t.Fatalf("Failed to get messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message in storage, got %d", len(messages))
	}

	if messages[0].Content != "Hello from Alice!" {
		t.Errorf("Expected content 'Hello from Alice!' in storage, got '%s'", messages[0].Content)
	}
}

func TestClient_Disconnect(t *testing.T) {
	store := storage.NewMemoryStorage()
	hub := NewHub(store)
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?sender=alice"

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// 接続を確認
	time.Sleep(100 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", hub.ClientCount())
	}

	// 接続を閉じる
	conn.Close()

	// 切断が処理されるのを待つ
	time.Sleep(200 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("Expected 0 clients after disconnect, got %d", hub.ClientCount())
	}
}

func TestNewClient(t *testing.T) {
	store := storage.NewMemoryStorage()
	hub := NewHub(store)

	client := NewClient(hub, nil, "test-sender")

	if client.hub != hub {
		t.Error("hub not properly set")
	}

	if client.sender != "test-sender" {
		t.Errorf("Expected sender 'test-sender', got '%s'", client.sender)
	}

	if client.send == nil {
		t.Error("send channel is nil")
	}
}
