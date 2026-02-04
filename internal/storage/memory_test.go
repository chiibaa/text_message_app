package storage

import (
	"testing"

	"github.com/tasukuchiba/text_messaging_app/internal/models"
)

func TestMemoryStorage_Save(t *testing.T) {
	store := NewMemoryStorage()
	msg := models.Message{
		ID:      "test-id",
		Sender:  "alice",
		Content: "Hello",
	}

	err := store.Save(msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	messages, err := store.GetAll()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
	if messages[0].ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", messages[0].ID)
	}
}

func TestMemoryStorage_GetAll(t *testing.T) {
	store := NewMemoryStorage()

	// 空の状態
	messages, err := store.GetAll()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}

	// メッセージ追加後
	store.Save(models.Message{ID: "1", Sender: "alice", Content: "Hello"})
	store.Save(models.Message{ID: "2", Sender: "bob", Content: "Hi"})

	messages, err = store.GetAll()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestMemoryStorage_GetByID(t *testing.T) {
	store := NewMemoryStorage()
	store.Save(models.Message{ID: "test-id", Sender: "alice", Content: "Hello"})

	// 存在するID
	msg, err := store.GetByID("test-id")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if msg.Sender != "alice" {
		t.Errorf("expected sender 'alice', got '%s'", msg.Sender)
	}

	// 存在しないID
	_, err = store.GetByID("non-existent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStorage_Delete(t *testing.T) {
	store := NewMemoryStorage()
	store.Save(models.Message{ID: "test-id", Sender: "alice", Content: "Hello"})

	// 削除
	err := store.Delete("test-id")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// 削除後は取得できない
	_, err = store.GetByID("test-id")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	// 存在しないIDの削除
	err = store.Delete("non-existent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStorage_GetAllReturnsCopy(t *testing.T) {
	store := NewMemoryStorage()
	store.Save(models.Message{ID: "1", Sender: "alice", Content: "Hello"})

	messages, _ := store.GetAll()
	messages[0].Content = "Modified"

	original, _ := store.GetByID("1")
	if original.Content != "Hello" {
		t.Error("GetAll should return a copy, not original data")
	}
}

// TestMemoryStorage_ImplementsStorage はMemoryStorageがStorageインターフェースを実装していることを確認する
func TestMemoryStorage_ImplementsStorage(t *testing.T) {
	var _ Storage = (*MemoryStorage)(nil)
}
