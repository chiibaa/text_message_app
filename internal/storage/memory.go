package storage

import (
	"sync"

	"github.com/tasukuchiba/text_messaging_app/internal/models"
)

// MemoryStorage はメッセージをメモリ上に保存するストレージ
type MemoryStorage struct {
	mu       sync.RWMutex
	messages []models.Message
}

// NewMemoryStorage は新しいMemoryStorageを作成する
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		messages: make([]models.Message, 0),
	}
}

// Save はメッセージを保存する
func (s *MemoryStorage) Save(msg models.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
	return nil
}

// GetAll は全てのメッセージを取得する
func (s *MemoryStorage) GetAll() ([]models.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.Message, len(s.messages))
	copy(result, s.messages)
	return result, nil
}

// GetByID は指定されたIDのメッセージを取得する
func (s *MemoryStorage) GetByID(id string) (models.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, msg := range s.messages {
		if msg.ID == id {
			return msg, nil
		}
	}
	return models.Message{}, ErrNotFound
}

// Delete は指定されたIDのメッセージを削除する
func (s *MemoryStorage) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, msg := range s.messages {
		if msg.ID == id {
			s.messages = append(s.messages[:i], s.messages[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}
