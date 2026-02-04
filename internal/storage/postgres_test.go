package storage

import (
	"os"
	"testing"
	"time"

	"github.com/tasukuchiba/text_messaging_app/internal/models"
)

// getTestDatabaseURL はテスト用のデータベースURLを取得する
func getTestDatabaseURL() string {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://app:password@localhost:5432/messaging?sslmode=disable"
	}
	return url
}

// skipIfNoPostgres はPostgreSQLが利用できない場合にテストをスキップする
func skipIfNoPostgres(t *testing.T) *PostgresStorage {
	t.Helper()
	storage, err := NewPostgresStorage(getTestDatabaseURL())
	if err != nil {
		t.Skipf("PostgreSQL not available: %v", err)
	}
	return storage
}

// cleanupMessages はテスト後にメッセージを削除する
func cleanupMessages(t *testing.T, storage *PostgresStorage) {
	t.Helper()
	_, err := storage.db.Exec("DELETE FROM messages")
	if err != nil {
		t.Fatalf("failed to cleanup messages: %v", err)
	}
}

func TestPostgresStorage_Save(t *testing.T) {
	storage := skipIfNoPostgres(t)
	defer storage.Close()
	defer cleanupMessages(t, storage)

	msg := models.Message{
		ID:        "test-pg-id",
		Sender:    "alice",
		Content:   "Hello from PostgreSQL",
		CreatedAt: time.Now(),
	}

	err := storage.Save(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 保存されたか確認
	retrieved, err := storage.GetByID("test-pg-id")
	if err != nil {
		t.Fatalf("failed to retrieve message: %v", err)
	}

	if retrieved.Sender != "alice" {
		t.Errorf("expected sender 'alice', got '%s'", retrieved.Sender)
	}

	if retrieved.Content != "Hello from PostgreSQL" {
		t.Errorf("expected content 'Hello from PostgreSQL', got '%s'", retrieved.Content)
	}
}

func TestPostgresStorage_GetAll(t *testing.T) {
	storage := skipIfNoPostgres(t)
	defer storage.Close()
	defer cleanupMessages(t, storage)

	// 空の状態
	messages, err := storage.GetAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}

	// メッセージ追加
	now := time.Now()
	storage.Save(models.Message{ID: "pg-1", Sender: "alice", Content: "Hello", CreatedAt: now})
	storage.Save(models.Message{ID: "pg-2", Sender: "bob", Content: "Hi", CreatedAt: now.Add(time.Second)})

	messages, err = storage.GetAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}

	// 作成日時順に並んでいることを確認
	if messages[0].ID != "pg-1" {
		t.Errorf("expected first message ID 'pg-1', got '%s'", messages[0].ID)
	}
	if messages[1].ID != "pg-2" {
		t.Errorf("expected second message ID 'pg-2', got '%s'", messages[1].ID)
	}
}

func TestPostgresStorage_GetByID(t *testing.T) {
	storage := skipIfNoPostgres(t)
	defer storage.Close()
	defer cleanupMessages(t, storage)

	storage.Save(models.Message{ID: "pg-test-id", Sender: "alice", Content: "Hello", CreatedAt: time.Now()})

	// 存在するID
	msg, err := storage.GetByID("pg-test-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Sender != "alice" {
		t.Errorf("expected sender 'alice', got '%s'", msg.Sender)
	}

	// 存在しないID
	_, err = storage.GetByID("non-existent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresStorage_Delete(t *testing.T) {
	storage := skipIfNoPostgres(t)
	defer storage.Close()
	defer cleanupMessages(t, storage)

	storage.Save(models.Message{ID: "pg-delete-id", Sender: "alice", Content: "Hello", CreatedAt: time.Now()})

	// 削除
	err := storage.Delete("pg-delete-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 削除後は取得できない
	_, err = storage.GetByID("pg-delete-id")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// 存在しないIDの削除
	err = storage.Delete("non-existent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestPostgresStorage_ImplementsStorage はPostgresStorageがStorageインターフェースを実装していることを確認する
func TestPostgresStorage_ImplementsStorage(t *testing.T) {
	var _ Storage = (*PostgresStorage)(nil)
}
