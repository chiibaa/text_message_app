# Text Messaging App - インフラストラクチャガイド

このドキュメントでは、Text Messaging App の AWS インフラストラクチャについて詳しく説明します。学習目的で各コンポーネントの役割と関係性を理解できるよう構成しています。

## 目次

1. [アーキテクチャ概要](#アーキテクチャ概要)
2. [ネットワーク構成](#ネットワーク構成)
3. [コンピュート (ECS Fargate)](#コンピュート-ecs-fargate)
4. [データベース (RDS PostgreSQL)](#データベース-rds-postgresql)
5. [ロードバランサー (ALB)](#ロードバランサー-alb)
6. [デプロイ (Blue/Green with CodeDeploy)](#デプロイ-bluegreen-with-codedeploy)
7. [Auto Scaling](#auto-scaling)
8. [CI/CD パイプライン](#cicd-パイプライン)
9. [セキュリティ](#セキュリティ)
10. [コスト最適化](#コスト最適化)
11. [運用ガイド](#運用ガイド)

---

## アーキテクチャ概要

### 全体図

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              GitHub                                      │
│   ┌──────────┐     ┌──────────┐     ┌──────────────┐                    │
│   │ Developer│────▶│   PR     │────▶│ main branch  │                    │
│   └──────────┘     └────┬─────┘     └──────┬───────┘                    │
│                         │                   │                            │
│                    CI (test)           CD (deploy)                       │
└─────────────────────────┼───────────────────┼────────────────────────────┘
                          │                   │
                          ▼                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                               AWS                                        │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                            VPC                                      │ │
│  │                                                                     │ │
│  │   ┌─────────────────┐              ┌─────────────────┐             │ │
│  │   │ Public Subnet A │              │ Public Subnet C │             │ │
│  │   └────────┬────────┘              └────────┬────────┘             │ │
│  │            │         ┌──────────┐           │                      │ │
│  │            └────────▶│   ALB    │◀──────────┘                      │ │
│  │                      │ (Blue/   │                                  │ │
│  │                      │  Green)  │                                  │ │
│  │                      └────┬─────┘                                  │ │
│  │                           │                                        │ │
│  │   ┌─────────────────┐     │      ┌─────────────────┐              │ │
│  │   │ Private Subnet A│◀────┴─────▶│ Private Subnet C│              │ │
│  │   │  (ECS Fargate)  │            │  (ECS Fargate)  │              │ │
│  │   │  Auto Scaling   │            │  Auto Scaling   │              │ │
│  │   └────────┬────────┘            └────────┬────────┘              │ │
│  │            │                              │                        │ │
│  │            └──────────────┬───────────────┘                        │ │
│  │                           ▼                                        │ │
│  │              ┌─────────────────────┐                               │ │
│  │              │  Isolated Subnet    │                               │ │
│  │              │  RDS PostgreSQL     │                               │ │
│  │              └─────────────────────┘                               │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  外部サービス:                                                            │
│  ┌─────────┐  ┌─────────────┐  ┌────────────────┐                       │
│  │   ECR   │  │  CodeDeploy │  │ Secrets Manager│                       │
│  │ (image) │  │ (B/G deploy)│  │  (DB password) │                       │
│  └─────────┘  └─────────────┘  └────────────────┘                       │
└─────────────────────────────────────────────────────────────────────────┘
```

### CDKスタック構成

```
TextMessaging-dev-Network
    │
    ├──▶ TextMessaging-dev-Database (depends on Network)
    │
    ├──▶ TextMessaging-dev-ECR (independent)
    │
    └──▶ TextMessaging-dev-ECS (depends on Network, Database, ECR)
```

| スタック | 主要リソース | 説明 |
|---------|-------------|------|
| Network | VPC, Subnet, SG | ネットワーク基盤 |
| Database | RDS PostgreSQL | データ永続化層 |
| ECR | ECR Repository | コンテナイメージ保存 |
| ECS | ECS, ALB, CodeDeploy | アプリケーション実行環境 |

---

## ネットワーク構成

### VPC設計

```
VPC CIDR: 10.0.0.0/16 (65,536 IPs)
│
├── Public Subnet A (10.0.0.0/24)
│   └── ALB, NAT Gateway
│
├── Public Subnet C (10.0.1.0/24)
│   └── ALB (冗長構成)
│
├── Private Subnet A (10.0.2.0/24)
│   └── ECS Fargate タスク
│
├── Private Subnet C (10.0.3.0/24)
│   └── ECS Fargate タスク
│
├── Isolated Subnet A (10.0.4.0/24)
│   └── RDS PostgreSQL
│
└── Isolated Subnet C (10.0.5.0/24)
    └── RDS PostgreSQL (Multi-AZ時)
```

### サブネットタイプの違い

| タイプ | インターネットアクセス | 用途 |
|--------|---------------------|------|
| **Public** | 双方向可能（IGW経由） | ALB、踏み台サーバー |
| **Private** | 外向きのみ（NAT経由） | ECS、Lambda |
| **Isolated** | 不可 | RDS、ElastiCache |

### セキュリティグループ

```
インターネット
    │
    ▼ HTTP/HTTPS (80, 443)
┌───────────────┐
│ ALB SG        │
└───────┬───────┘
        │
        ▼ TCP 8080
┌───────────────┐
│ ECS SG        │
└───────┬───────┘
        │
        ▼ TCP 5432
┌───────────────┐
│ RDS SG        │
└───────────────┘
```

**最小権限の原則**: 必要な通信のみを許可し、それ以外は全て拒否。

---

## コンピュート (ECS Fargate)

### Fargate とは

- **サーバーレスコンテナ実行環境**
- EC2インスタンスの管理が不要
- タスク単位でCPU/メモリを割り当て
- 使用した分だけ課金

### ECS の構成要素

```
ECS Cluster
└── ECS Service
    └── ECS Task (Fargate)
        └── Container (text-messaging-app)
```

| コンポーネント | 説明 |
|--------------|------|
| **Cluster** | タスクの論理的なグループ |
| **Service** | タスクの desired count を維持 |
| **Task Definition** | コンテナの実行方法を定義 |
| **Task** | 実行中のコンテナ群 |

### タスク定義の設定

```json
{
  "cpu": "256",           // 0.25 vCPU
  "memory": "512",        // 512 MB
  "containerDefinitions": [{
    "name": "text-messaging-app",
    "portMappings": [{"containerPort": 8080}],
    "healthCheck": {
      "command": ["CMD-SHELL", "wget ... /health"],
      "interval": 30
    }
  }]
}
```

---

## データベース (RDS PostgreSQL)

### RDS の特徴

- **マネージドデータベースサービス**
- 自動バックアップ、パッチ適用
- Secrets Manager でパスワード管理

### 設定内容

| 項目 | 値 | 説明 |
|------|-----|------|
| エンジン | PostgreSQL 16.4 | 最新の安定バージョン |
| インスタンス | db.t3.micro | 開発用（無料利用枠） |
| ストレージ | 20GB (GP3) | 自動スケーリング対応 |
| Multi-AZ | No | 開発用（本番ではYes推奨） |
| 暗号化 | Yes | データ保護 |

### Secrets Manager 連携

```
ECS Task
    │
    │ 1. タスク起動時にシークレット取得
    ▼
┌───────────────────┐
│ Secrets Manager   │
│ (DATABASE_URL)    │
└───────────────────┘
    │
    │ 2. 接続情報を環境変数に注入
    ▼
Container
    │
    │ 3. PostgreSQL に接続
    ▼
RDS
```

---

## ロードバランサー (ALB)

### ALB の役割

```
ユーザー
    │
    │ HTTP Request
    ▼
┌─────────────────────────────────────────┐
│                 ALB                      │
│  ┌─────────────┐   ┌─────────────┐      │
│  │ Listener    │   │ Listener    │      │
│  │ (Port 80)   │   │ (Port 8080) │      │
│  │ 本番用       │   │ テスト用     │      │
│  └──────┬──────┘   └──────┬──────┘      │
│         │                 │              │
│    ┌────┴────┐      ┌────┴────┐         │
│    │  Blue   │      │  Green  │         │
│    │  TG     │      │  TG     │         │
│    └────┬────┘      └────┬────┘         │
└─────────┼────────────────┼──────────────┘
          │                │
          ▼                ▼
     ECS Tasks         ECS Tasks
     (現バージョン)      (新バージョン)
```

### ヘルスチェック設定

| 項目 | 値 |
|------|-----|
| Path | /health |
| Protocol | HTTP |
| Interval | 30秒 |
| Healthy threshold | 2回 |
| Unhealthy threshold | 3回 |

---

## デプロイ (Blue/Green with CodeDeploy)

### Blue/Green デプロイとは

**ダウンタイムゼロ**でアプリケーションを更新する手法。

```
デプロイ前:
┌─────────────────────────────────┐
│              ALB                │
│         ┌─────────┐             │
│ ────────│  Blue   │────────▶   │
│ 100%    │  (v1)   │            │
│         └─────────┘             │
└─────────────────────────────────┘

デプロイ中:
┌─────────────────────────────────┐
│              ALB                │
│    ┌─────────┐  ┌─────────┐    │
│ ───│  Blue   │  │  Green  │    │
│100%│  (v1)   │  │  (v2)   │0%  │
│    └─────────┘  └─────────┘    │
└─────────────────────────────────┘
                  ↑
           新バージョン起動・ヘルスチェック

デプロイ後:
┌─────────────────────────────────┐
│              ALB                │
│             ┌─────────┐         │
│        ────▶│  Green  │────     │
│             │  (v2)   │ 100%   │
│             └─────────┘         │
└─────────────────────────────────┘
```

### デプロイフロー

1. **新タスク起動**: Green Target Group に新バージョンのタスクを起動
2. **ヘルスチェック**: タスクが正常に動作することを確認
3. **トラフィック切り替え**: ALB が Green にトラフィックを送信
4. **旧タスク終了**: Blue Target Group の旧タスクを終了

### ロールバック

デプロイ失敗時は自動的に Blue にトラフィックを戻す。

---

## Auto Scaling

### スケーリングの仕組み

```
CloudWatch Metrics
    │
    │ CPU使用率 > 70%
    ▼
┌───────────────────┐
│ Scaling Policy    │
│ (Target Tracking) │
└─────────┬─────────┘
          │
          ▼
┌───────────────────┐
│   ECS Service     │
│ desiredCount: 1→2 │
└───────────────────┘
          │
          ▼
    新しいタスク起動
```

### 設定値

| 項目 | 値 |
|------|-----|
| 最小タスク数 | 1 |
| 最大タスク数 | 4 |
| CPU目標使用率 | 70% |
| メモリ目標使用率 | 70% |
| スケールアウト待機 | 60秒 |
| スケールイン待機 | 300秒 |

---

## CI/CD パイプライン

### ワークフロー図

```
┌──────────────┐
│   PR作成     │
└──────┬───────┘
       │
       ▼
┌──────────────┐     ┌──────────────────────────┐
│  CI Workflow │────▶│ - go test                │
│  (ci.yml)    │     │ - go build               │
│              │     │ - golangci-lint          │
│              │     │ - docker build (no push) │
└──────┬───────┘     └──────────────────────────┘
       │
       │ マージ
       ▼
┌──────────────┐     ┌──────────────────────────┐
│  CD Workflow │────▶│ - AWS OIDC 認証          │
│ (deploy.yml) │     │ - docker build & push    │
│              │     │ - タスク定義更新          │
│              │     │ - CodeDeploy B/G デプロイ │
│              │     │ - ヘルスチェック          │
└──────────────┘     └──────────────────────────┘
```

### OIDC 認証

```
GitHub Actions
    │
    │ 1. OIDC トークン発行
    ▼
┌───────────────────┐
│ AWS STS           │
│ AssumeRoleWith    │
│ WebIdentity       │
└─────────┬─────────┘
          │
          │ 2. 一時的な認証情報
          ▼
    AWS リソースにアクセス
```

**メリット**: シークレットキーの管理が不要でセキュア。

---

## セキュリティ

### 多層防御

```
┌─────────────────────────────────────────────┐
│ 1. ネットワーク分離                          │
│    - Private/Isolated Subnet               │
│    - Security Group による最小権限          │
└─────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│ 2. シークレット管理                          │
│    - Secrets Manager でパスワード管理        │
│    - IAM ロールによるアクセス制御            │
└─────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│ 3. コンテナセキュリティ                      │
│    - 非 root ユーザーで実行                 │
│    - ECR イメージスキャン                   │
└─────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│ 4. データ保護                               │
│    - RDS ストレージ暗号化                   │
│    - TLS 通信 (将来)                        │
└─────────────────────────────────────────────┘
```

### IAM ロール

| ロール | 用途 |
|--------|------|
| Task Execution Role | ECS エージェントが使用（ECRプル、ログ出力） |
| Task Role | アプリケーションコンテナが使用（AWS サービスアクセス） |
| GitHub Actions Role | CI/CD 用（OIDC 認証） |

---

## コスト最適化

### 開発環境向け設定

| 項目 | 本番推奨 | 開発設定 | コスト削減効果 |
|------|---------|---------|---------------|
| NAT Gateway | 2個 | 1個 | ~$45/月 |
| RDS Multi-AZ | Yes | No | ~$15/月 |
| RDS インスタンス | t3.small | t3.micro | ~$10/月 |
| ECS タスク最大数 | 10 | 4 | 変動 |

### 概算コスト (開発環境)

| サービス | 概算/月 |
|---------|--------|
| ECS Fargate | ~$15 |
| RDS PostgreSQL | ~$15 |
| ALB | ~$20 |
| NAT Gateway | ~$45 |
| ECR | ~$1 |
| **合計** | **~$96** |

---

## 運用ガイド

### 初回デプロイ

```bash
# 1. CDKプロジェクト初期化
cd infra && npm install && npm run build

# 2. CDK Bootstrap (初回のみ)
npx cdk bootstrap

# 3. 全スタックデプロイ
npx cdk deploy --all

# 4. 初回イメージプッシュ
aws ecr get-login-password | docker login --username AWS --password-stdin <ACCOUNT>.dkr.ecr.ap-northeast-1.amazonaws.com
docker build -t text-messaging-app .
docker tag text-messaging-app:latest <ACCOUNT>.dkr.ecr.ap-northeast-1.amazonaws.com/text-messaging-app-dev:latest
docker push <ACCOUNT>.dkr.ecr.ap-northeast-1.amazonaws.com/text-messaging-app-dev:latest

# 5. ECSサービス更新（初回イメージ適用）
aws ecs update-service --cluster text-messaging-dev --service text-messaging-dev --force-new-deployment
```

### GitHub Secrets 設定

GitHub リポジトリの Settings > Secrets and variables > Actions で設定:

| Secret 名 | 説明 |
|-----------|------|
| `AWS_ROLE_ARN` | OIDC 用 IAM ロールの ARN |

### OIDC 用 IAM ロール作成

```bash
# IAM Identity Provider 作成（初回のみ）
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1

# IAM ロール作成（信頼ポリシー）
cat <<EOF > trust-policy.json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::<ACCOUNT>:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:<OWNER>/<REPO>:*"
        }
      }
    }
  ]
}
EOF

aws iam create-role \
  --role-name GitHubActionsRole \
  --assume-role-policy-document file://trust-policy.json
```

### 検証コマンド

```bash
# ALB DNS名を取得
aws cloudformation describe-stacks \
  --stack-name TextMessaging-dev-ECS \
  --query "Stacks[0].Outputs[?OutputKey=='AlbDnsName'].OutputValue" \
  --output text

# ヘルスチェック
curl http://<ALB_DNS>/health

# メッセージ投稿
curl -X POST http://<ALB_DNS>/messages \
  -H "Content-Type: application/json" \
  -d '{"sender":"alice","content":"Hello from AWS!"}'

# メッセージ取得
curl http://<ALB_DNS>/messages
```

### トラブルシューティング

#### ECSタスクが起動しない

```bash
# タスクの停止理由を確認
aws ecs describe-tasks \
  --cluster text-messaging-dev \
  --tasks <TASK_ID> \
  --query "tasks[0].stoppedReason"

# CloudWatch Logs を確認
aws logs tail /ecs/text-messaging-dev --follow
```

#### データベース接続エラー

```bash
# ECS Exec でコンテナに入る
aws ecs execute-command \
  --cluster text-messaging-dev \
  --task <TASK_ID> \
  --container text-messaging-app \
  --interactive \
  --command "/bin/sh"

# コンテナ内で接続確認
wget --spider http://localhost:8080/health
```

#### デプロイ失敗

```bash
# CodeDeploy のデプロイ状況を確認
aws deploy list-deployments \
  --application-name text-messaging-dev \
  --deployment-group-name text-messaging-dev-dg

# デプロイ詳細
aws deploy get-deployment --deployment-id <DEPLOYMENT_ID>
```

---

## まとめ

このインフラストラクチャは以下の原則に基づいて設計されています：

1. **セキュリティファースト**: ネットワーク分離、最小権限、シークレット管理
2. **高可用性**: マルチAZ、Auto Scaling、ヘルスチェック
3. **運用効率**: IaC (CDK)、CI/CD 自動化、Blue/Green デプロイ
4. **コスト最適化**: 開発環境向けの適切なサイジング

学習を進める中で、各コンポーネントを実際に操作し、理解を深めてください。
