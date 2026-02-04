# Text Messaging App

Go言語で実装するスケーラブルなチャットアプリケーション

## プロジェクト目標

- Go言語の学習
- AWSの学習
- スケーラブルなメッセージングシステムの理解

## Tech Stack

- **言語**: Go 1.21+
- **Webフレームワーク**: net/http (標準ライブラリ)
- **WebSocket**: gorilla/websocket (フェーズ3で導入)
- **データベース**: PostgreSQL または DynamoDB (フェーズ4で導入)
- **デプロイ**: AWS ECS/EC2 (フェーズ5で導入)

## ディレクトリ構造

```
.
├── cmd/
│   └── server/          # アプリケーションエントリーポイント
├── internal/
│   ├── handlers/        # HTTPハンドラー
│   ├── models/          # データモデル
│   └── storage/         # ストレージレイヤー
├── pkg/
│   └── websocket/       # WebSocket関連のパッケージ
├── docs/                # ドキュメント
├── go.mod               # Go モジュール定義
└── README.md
```

## セットアップ

### 前提条件

- Go 1.21以上がインストールされていること
- Git

### インストール

```bash
# リポジトリのクローン
git clone https://github.com/tasukuchiba/text_messaging_app.git
cd text_messaging_app

# 依存関係のダウンロード
go mod download
```

## 開発コマンド

```bash
# サーバーの起動 (今後実装)
go run cmd/server/main.go

# テストの実行 (今後実装)
go test ./...

# ビルド (今後実装)
go build -o bin/server cmd/server/main.go
```

## 実装フェーズ

### フェーズ1: 基礎設定 ✅ (完了)
- Goプロジェクト初期化
- プロジェクトディレクトリ構造の作成
- 基本的な設定ファイルの作成

### フェーズ2: シンプルなメッセージングAPI (予定)
- HTTPサーバーの実装
- RESTful APIエンドポイント
- インメモリストレージ

### フェーズ3: WebSocketによるリアルタイム通信 (予定)
- WebSocketサーバー実装
- クライアント接続管理
- リアルタイムメッセージ配信

### フェーズ4: データベース統合 (予定)
- データベーススキーマ設計
- メッセージ永続化
- ユーザー管理

### フェーズ5: AWSデプロイ (予定)
- Dockerコンテナ化
- AWS環境構築
- CI/CD設定

## ドキュメント

各フェーズの詳細なドキュメントは `docs/` ディレクトリにあります：

- [フェーズ1: 基礎設定](docs/phase1.md)

## ライセンス

MIT License
