package storage

import (
	"errors"

	"github.com/tasukuchiba/text_messaging_app/internal/models"
)

// ErrNotFound はメッセージが見つからない場合のエラー
var ErrNotFound = errors.New("message not found")

// Storage はメッセージストレージのインターフェース
type Storage interface {
	// Save はメッセージを保存する
	Save(msg models.Message) error

	// GetAll は全てのメッセージを取得する
	GetAll() ([]models.Message, error)

	// GetByID は指定されたIDのメッセージを取得する
	GetByID(id string) (models.Message, error)

	// Delete は指定されたIDのメッセージを削除する
	Delete(id string) error
}
