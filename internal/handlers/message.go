package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tasukuchiba/text_messaging_app/internal/models"
	"github.com/tasukuchiba/text_messaging_app/internal/storage"
)

// MessageHandler はメッセージ関連のHTTPリクエストを処理する
type MessageHandler struct {
	storage storage.Storage
}

// NewMessageHandler は新しいMessageHandlerを作成する
func NewMessageHandler(s storage.Storage) *MessageHandler {
	return &MessageHandler{storage: s}
}

// CreateMessageRequest はメッセージ作成リクエストのボディ
type CreateMessageRequest struct {
	Sender  string `json:"sender"`
	Content string `json:"content"`
}

// HandleMessages は /messages エンドポイントのハンドラー
func (h *MessageHandler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getMessages(w, r)
	case http.MethodPost:
		h.createMessage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleMessageByID は /messages/{id} エンドポイントのハンドラー
func (h *MessageHandler) HandleMessageByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/messages/")
	if id == "" {
		http.Error(w, "Message ID is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getMessageByID(w, r, id)
	case http.MethodDelete:
		h.deleteMessage(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getMessages は全てのメッセージを取得する
func (h *MessageHandler) getMessages(w http.ResponseWriter, r *http.Request) {
	messages, err := h.storage.GetAll()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// getMessageByID は指定されたIDのメッセージを取得する
func (h *MessageHandler) getMessageByID(w http.ResponseWriter, r *http.Request, id string) {
	msg, err := h.storage.GetByID(id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "Message not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

// createMessage は新しいメッセージを作成する
func (h *MessageHandler) createMessage(w http.ResponseWriter, r *http.Request) {
	var req CreateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Sender == "" || req.Content == "" {
		http.Error(w, "Sender and content are required", http.StatusBadRequest)
		return
	}

	msg := models.Message{
		ID:        uuid.New().String(),
		Sender:    req.Sender,
		Content:   req.Content,
		CreatedAt: time.Now(),
	}

	if err := h.storage.Save(msg); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(msg)
}

// deleteMessage は指定されたIDのメッセージを削除する
func (h *MessageHandler) deleteMessage(w http.ResponseWriter, r *http.Request, id string) {
	err := h.storage.Delete(id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "Message not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
