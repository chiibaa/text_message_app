package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tasukuchiba/text_messaging_app/internal/models"
	"github.com/tasukuchiba/text_messaging_app/internal/storage"
)

func TestHandleMessages_GET(t *testing.T) {
	store := storage.NewMemoryStorage()
	store.Save(models.Message{ID: "1", Sender: "alice", Content: "Hello"})

	handler := NewMessageHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	rec := httptest.NewRecorder()

	handler.HandleMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var messages []models.Message
	if err := json.NewDecoder(rec.Body).Decode(&messages); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
}

func TestHandleMessages_POST(t *testing.T) {
	store := storage.NewMemoryStorage()
	handler := NewMessageHandler(store)

	body := `{"sender":"alice","content":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleMessages(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var msg models.Message
	if err := json.NewDecoder(rec.Body).Decode(&msg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if msg.Sender != "alice" {
		t.Errorf("expected sender 'alice', got '%s'", msg.Sender)
	}
	if msg.ID == "" {
		t.Error("expected ID to be generated")
	}
}

func TestHandleMessages_POST_InvalidBody(t *testing.T) {
	store := storage.NewMemoryStorage()
	handler := NewMessageHandler(store)

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPost, "/messages", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.HandleMessages(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleMessages_POST_MissingFields(t *testing.T) {
	store := storage.NewMemoryStorage()
	handler := NewMessageHandler(store)

	tests := []struct {
		name string
		body string
	}{
		{"missing sender", `{"content":"Hello"}`},
		{"missing content", `{"sender":"alice"}`},
		{"empty sender", `{"sender":"","content":"Hello"}`},
		{"empty content", `{"sender":"alice","content":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/messages", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()

			handler.HandleMessages(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
}

func TestHandleMessages_MethodNotAllowed(t *testing.T) {
	store := storage.NewMemoryStorage()
	handler := NewMessageHandler(store)

	req := httptest.NewRequest(http.MethodPut, "/messages", nil)
	rec := httptest.NewRecorder()

	handler.HandleMessages(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleMessageByID_GET(t *testing.T) {
	store := storage.NewMemoryStorage()
	store.Save(models.Message{ID: "test-id", Sender: "alice", Content: "Hello"})
	handler := NewMessageHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/messages/test-id", nil)
	rec := httptest.NewRecorder()

	handler.HandleMessageByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var msg models.Message
	if err := json.NewDecoder(rec.Body).Decode(&msg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if msg.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", msg.ID)
	}
}

func TestHandleMessageByID_GET_NotFound(t *testing.T) {
	store := storage.NewMemoryStorage()
	handler := NewMessageHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/messages/non-existent", nil)
	rec := httptest.NewRecorder()

	handler.HandleMessageByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleMessageByID_DELETE(t *testing.T) {
	store := storage.NewMemoryStorage()
	store.Save(models.Message{ID: "test-id", Sender: "alice", Content: "Hello"})
	handler := NewMessageHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/messages/test-id", nil)
	rec := httptest.NewRecorder()

	handler.HandleMessageByID(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	// 削除後は取得できない
	req = httptest.NewRequest(http.MethodGet, "/messages/test-id", nil)
	rec = httptest.NewRecorder()
	handler.HandleMessageByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d after delete, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleMessageByID_DELETE_NotFound(t *testing.T) {
	store := storage.NewMemoryStorage()
	handler := NewMessageHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/messages/non-existent", nil)
	rec := httptest.NewRecorder()

	handler.HandleMessageByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}
