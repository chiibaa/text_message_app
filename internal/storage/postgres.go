package storage

import (
	"database/sql"
	"time"

	_ "github.com/lib/pq"
	"github.com/tasukuchiba/text_messaging_app/internal/models"
)

// PostgresStorage はメッセージをPostgreSQLに保存するストレージ
type PostgresStorage struct {
	db *sql.DB
}

// NewPostgresStorage は新しいPostgresStorageを作成する
func NewPostgresStorage(databaseURL string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	// 接続プール設定
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// 接続確認
	if err := db.Ping(); err != nil {
		return nil, err
	}

	storage := &PostgresStorage{db: db}

	// マイグレーション実行
	if err := storage.migrate(); err != nil {
		return nil, err
	}

	return storage, nil
}

// migrate はデータベーススキーマを作成する
func (s *PostgresStorage) migrate() error {
	query := `
		CREATE TABLE IF NOT EXISTS messages (
			id VARCHAR(36) PRIMARY KEY,
			sender VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
	`
	_, err := s.db.Exec(query)
	return err
}

// Save はメッセージを保存する
func (s *PostgresStorage) Save(msg models.Message) error {
	query := `
		INSERT INTO messages (id, sender, content, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := s.db.Exec(query, msg.ID, msg.Sender, msg.Content, msg.CreatedAt)
	return err
}

// GetAll は全てのメッセージを取得する
func (s *PostgresStorage) GetAll() ([]models.Message, error) {
	query := `
		SELECT id, sender, content, created_at
		FROM messages
		ORDER BY created_at ASC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.Sender, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// nilではなく空のスライスを返す
	if messages == nil {
		messages = []models.Message{}
	}

	return messages, nil
}

// GetByID は指定されたIDのメッセージを取得する
func (s *PostgresStorage) GetByID(id string) (models.Message, error) {
	query := `
		SELECT id, sender, content, created_at
		FROM messages
		WHERE id = $1
	`
	var msg models.Message
	err := s.db.QueryRow(query, id).Scan(&msg.ID, &msg.Sender, &msg.Content, &msg.CreatedAt)
	if err == sql.ErrNoRows {
		return models.Message{}, ErrNotFound
	}
	if err != nil {
		return models.Message{}, err
	}
	return msg, nil
}

// Delete は指定されたIDのメッセージを削除する
func (s *PostgresStorage) Delete(id string) error {
	query := `DELETE FROM messages WHERE id = $1`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// Close はデータベース接続を閉じる
func (s *PostgresStorage) Close() error {
	return s.db.Close()
}
