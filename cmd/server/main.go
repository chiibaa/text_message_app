package main

import (
	"log"
	"net/http"
	"os"

	"github.com/tasukuchiba/text_messaging_app/internal/handlers"
	"github.com/tasukuchiba/text_messaging_app/internal/storage"
	"github.com/tasukuchiba/text_messaging_app/internal/websocket"
)

func main() {
	// ストレージの初期化
	store, cleanup := initStorage()
	if cleanup != nil {
		defer cleanup()
	}

	// ハンドラーの初期化
	messageHandler := handlers.NewMessageHandler(store)

	// WebSocket Hubの初期化と起動
	hub := websocket.NewHub(store)
	go hub.Run()

	// ルーティング設定
	http.HandleFunc("/messages", messageHandler.HandleMessages)
	http.HandleFunc("/messages/", messageHandler.HandleMessageByID)

	// WebSocketエンドポイント
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(hub, w, r)
	})

	// ヘルスチェック用エンドポイント
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// サーバー起動（環境変数PORTがあればそれを使用）
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("Server starting on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// initStorage は環境変数に基づいてストレージを初期化する
func initStorage() (storage.Storage, func()) {
	storageType := os.Getenv("STORAGE_TYPE")

	switch storageType {
	case "postgres":
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			log.Fatal("DATABASE_URL is required when STORAGE_TYPE=postgres")
		}

		store, err := storage.NewPostgresStorage(databaseURL)
		if err != nil {
			log.Fatalf("Failed to connect to PostgreSQL: %v", err)
		}

		log.Println("Using PostgreSQL storage")
		return store, func() {
			if err := store.Close(); err != nil {
				log.Printf("Error closing database connection: %v", err)
			}
		}

	default:
		log.Println("Using in-memory storage")
		return storage.NewMemoryStorage(), nil
	}
}
