# =============================================================================
# Text Messaging App - Multi-stage Dockerfile
# =============================================================================
# 学習ポイント:
# - マルチステージビルドで最終イメージを軽量化
# - ビルダーステージ: Go コンパイル用（重い）
# - 実行ステージ: Alpine Linux + バイナリのみ（軽量）
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Builder
# -----------------------------------------------------------------------------
# Go の公式イメージを使用してビルド
FROM golang:1.22-alpine AS builder

# ビルドに必要なツールをインストール
# - git: go mod download で必要な場合がある
# - ca-certificates: HTTPS通信用
RUN apk add --no-cache git ca-certificates

# 作業ディレクトリを設定
WORKDIR /app

# 依存関係ファイルを先にコピー（Docker レイヤーキャッシュを活用）
# go.mod/go.sum が変更されない限り、このレイヤーはキャッシュされる
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピー
COPY . .

# 静的リンクされたバイナリをビルド
# - CGO_ENABLED=0: Cライブラリに依存しない静的バイナリを生成
# - -ldflags="-w -s": デバッグ情報を削除してバイナリを小さく
# - -o: 出力ファイル名を指定
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/server \
    ./cmd/server

# -----------------------------------------------------------------------------
# Stage 2: Runtime
# -----------------------------------------------------------------------------
# 最小限の Alpine イメージを使用
FROM alpine:3.19

# セキュリティ: 非rootユーザーで実行
# - 本番環境では root ユーザーで実行しない
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

# CA証明書をインストール（HTTPS通信に必要）
RUN apk add --no-cache ca-certificates tzdata

# 作業ディレクトリを設定
WORKDIR /app

# ビルダーステージからバイナリをコピー
COPY --from=builder /app/server /app/server

# 実行ファイルの所有権を設定
RUN chown appuser:appgroup /app/server

# 非rootユーザーに切り替え
USER appuser

# アプリケーションが使用するポート
# 注: 実際のポートは環境変数 PORT で制御可能
EXPOSE 8080

# ヘルスチェック設定
# - ECSのヘルスチェックとは別に、コンテナ自体のヘルスチェック
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# アプリケーション起動
# - exec形式を使用（シグナル処理が正しく動作する）
CMD ["/app/server"]
