# Text Messaging App - デプロイ手順書

## 目次

1. [前提条件](#1-前提条件)
2. [AWS CLI 設定](#2-aws-cli-設定)
3. [CDK Bootstrap](#3-cdk-bootstrap)
4. [インフラデプロイ（CDK）](#4-インフラデプロイcdk)
5. [初回 Docker イメージのビルド & プッシュ](#5-初回-docker-イメージのビルド--プッシュ)
6. [ECS サービスの更新](#6-ecs-サービスの更新)
7. [動作確認](#7-動作確認)
8. [GitHub CI/CD の設定（自動デプロイ）](#8-github-cicd-の設定自動デプロイ)
9. [クリーンアップ（リソース削除）](#9-クリーンアップリソース削除)
10. [トラブルシューティング](#10-トラブルシューティング)

---

## 1. 前提条件

### 必要なツール

| ツール | バージョン | 確認コマンド |
|--------|-----------|-------------|
| AWS CLI | v2 | `aws --version` |
| Node.js | 18+ | `node --version` |
| npm | 9+ | `npm --version` |
| Docker | 20+ | `docker --version` |
| Go | 1.22+ | `go version` |
| Git | 2+ | `git --version` |

### 必要な情報

| 項目 | 例 |
|------|-----|
| AWS アカウント ID | `123456789012` |
| AWS リージョン | `ap-northeast-1` |
| GitHub リポジトリオーナー | `tasukuchiba` |
| GitHub リポジトリ名 | `text_messaging_app` |

---

## 2. AWS CLI 設定

AWS CLI が未設定の場合、認証情報を設定する。

```bash
aws configure
```

| プロンプト | 入力値 |
|-----------|--------|
| AWS Access Key ID | IAM ユーザーのアクセスキー |
| AWS Secret Access Key | IAM ユーザーのシークレットキー |
| Default region name | `ap-northeast-1` |
| Default output format | `json` |

設定確認:

```bash
aws sts get-caller-identity
```

正常なら以下のようなレスポンスが返る:

```json
{
    "UserId": "AIDXXXXXXXXXXXXXXXXX",
    "Account": "123456789012",
    "Arn": "arn:aws:iam::123456789012:user/your-user"
}
```

---

## 3. CDK Bootstrap

CDK が CloudFormation テンプレートやアセットを格納するための S3 バケット等を作成する。
対象リージョンで **初回のみ** 実行が必要。

```bash
cd infra
npm install
npx cdk bootstrap aws://<ACCOUNT_ID>/ap-northeast-1
```

成功すると以下のような出力が表示される:

```
 ⏳  Bootstrapping environment aws://123456789012/ap-northeast-1...
 ✅  Environment aws://123456789012/ap-northeast-1 bootstrapped
```

---

## 4. インフラデプロイ（CDK）

### 4-1. デプロイ前の差分確認（任意）

```bash
cd infra
npx cdk diff
```

### 4-2. 全スタックのデプロイ

```bash
cd infra
npx cdk deploy --all
```

途中で IAM リソース作成の確認プロンプトが出るので `y` を入力して承認する。

### 作成されるスタックと所要時間の目安

| 順序 | スタック名 | 主要リソース | 所要時間 |
|------|-----------|-------------|---------|
| 1 | TextMessaging-dev-Network | VPC, Subnet, NAT Gateway, Security Group | 約 3 分 |
| 2 | TextMessaging-dev-ECR | ECR リポジトリ | 約 1 分 |
| 3 | TextMessaging-dev-Database | RDS PostgreSQL | 約 10〜15 分 |
| 4 | TextMessaging-dev-ECS | ECS Cluster, ALB, Service, CodeDeploy | 約 5 分 |

> **注意**: ECS Service はこの時点ではダミーイメージ (`amazon/amazon-ecs-sample`) で起動する。次のステップで実際のアプリイメージに差し替える。

### 4-3. デプロイ結果の確認

```bash
# 各スタックの出力値を確認
aws cloudformation describe-stacks \
  --stack-name TextMessaging-dev-ECS \
  --query "Stacks[0].Outputs" \
  --output table
```

---

## 5. 初回 Docker イメージのビルド & プッシュ

### 5-1. ECR にログイン

```bash
aws ecr get-login-password --region ap-northeast-1 | \
  docker login --username AWS --password-stdin \
  <ACCOUNT_ID>.dkr.ecr.ap-northeast-1.amazonaws.com
```

成功すると `Login Succeeded` と表示される。

### 5-2. Docker イメージのビルド

```bash
# プロジェクトルートで実行
docker build -t text-messaging-app .
```

### 5-3. タグ付け & プッシュ

```bash
docker tag text-messaging-app:latest \
  <ACCOUNT_ID>.dkr.ecr.ap-northeast-1.amazonaws.com/text-messaging-app-dev:latest

docker push \
  <ACCOUNT_ID>.dkr.ecr.ap-northeast-1.amazonaws.com/text-messaging-app-dev:latest
```

### 5-4. プッシュ結果の確認

```bash
aws ecr describe-images \
  --repository-name text-messaging-app-dev \
  --query "imageDetails[*].{Tags:imageTags,PushedAt:imagePushedAt,Size:imageSizeInBytes}" \
  --output table
```

---

## 6. ECS サービスの更新

ダミーイメージから実際のアプリイメージに切り替える。

```bash
aws ecs update-service \
  --cluster text-messaging-dev \
  --service text-messaging-dev \
  --force-new-deployment
```

### デプロイ状況の確認

```bash
# サービスの状態を監視（安定するまで待つ）
aws ecs wait services-stable \
  --cluster text-messaging-dev \
  --services text-messaging-dev

echo "サービスが安定しました"
```

または手動で状態を確認:

```bash
aws ecs describe-services \
  --cluster text-messaging-dev \
  --services text-messaging-dev \
  --query "services[0].{Status:status,Running:runningCount,Desired:desiredCount}" \
  --output table
```

---

## 7. 動作確認

### 7-1. ALB DNS 名の取得

```bash
ALB_DNS=$(aws cloudformation describe-stacks \
  --stack-name TextMessaging-dev-ECS \
  --query "Stacks[0].Outputs[?OutputKey=='AlbDnsName'].OutputValue" \
  --output text)

echo "アプリケーション URL: http://$ALB_DNS"
```

### 7-2. ヘルスチェック

```bash
curl http://$ALB_DNS/health
```

正常なら `200 OK` でレスポンスが返る。

### 7-3. API テスト

```bash
# メッセージ投稿
curl -X POST http://$ALB_DNS/messages \
  -H "Content-Type: application/json" \
  -d '{"sender":"alice","content":"Hello from AWS!"}'

# メッセージ取得
curl http://$ALB_DNS/messages
```

---

## 8. GitHub CI/CD の設定（自動デプロイ）

> ステップ 7 までで手動デプロイは完了している。
> ここからは `main` ブランチへの push で自動デプロイする設定を行う。

### 8-1. GitHub リポジトリの作成 & プッシュ

```bash
# ブランチ名を main に変更（現在 master の場合）
git branch -m master main

# リモートリポジトリを追加してプッシュ
git remote add origin https://github.com/<OWNER>/text_messaging_app.git
git push -u origin main
```

### 8-2. OIDC ID プロバイダーの作成（AWS 側、初回のみ）

GitHub Actions が AWS に OIDC で認証するためのプロバイダーを作成する。

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1
```

### 8-3. GitHub Actions 用 IAM ロールの作成

#### 信頼ポリシーの作成

```bash
cat <<'EOF' > trust-policy.json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::<ACCOUNT_ID>:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:<OWNER>/text_messaging_app:*"
        }
      }
    }
  ]
}
EOF
```

> `<ACCOUNT_ID>` と `<OWNER>` は実際の値に置き換えること。

#### ロールの作成

```bash
aws iam create-role \
  --role-name GitHubActionsRole \
  --assume-role-policy-document file://trust-policy.json
```

#### 必要なポリシーのアタッチ

```bash
# ECR（イメージプッシュ）
aws iam attach-role-policy \
  --role-name GitHubActionsRole \
  --policy-arn arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPowerUser

# ECS（タスク定義・サービス操作）
aws iam attach-role-policy \
  --role-name GitHubActionsRole \
  --policy-arn arn:aws:iam::aws:policy/AmazonECS_FullAccess

# CodeDeploy（Blue/Green デプロイ）
aws iam attach-role-policy \
  --role-name GitHubActionsRole \
  --policy-arn arn:aws:iam::aws:policy/AWSCodeDeployFullAccess

# CloudFormation（スタック出力の読み取り）
aws iam attach-role-policy \
  --role-name GitHubActionsRole \
  --policy-arn arn:aws:iam::aws:policy/AWSCloudFormationReadOnlyAccess
```

#### ロール ARN の確認

```bash
aws iam get-role \
  --role-name GitHubActionsRole \
  --query "Role.Arn" \
  --output text
```

### 8-4. GitHub Secrets の設定

GitHub リポジトリの **Settings > Secrets and variables > Actions** で以下を登録:

| Secret 名 | 値 |
|-----------|-----|
| `AWS_ROLE_ARN` | `arn:aws:iam::<ACCOUNT_ID>:role/GitHubActionsRole` |

### 8-5. 自動デプロイの動作確認

`main` ブランチにコード変更を push して、GitHub Actions の Deploy ワークフローが正常に実行されることを確認する。

```bash
# 例: 軽微な変更を push
git checkout main
git add .
git commit -m "test: trigger deploy"
git push origin main
```

GitHub の **Actions** タブで `Deploy` ワークフローの実行状況を確認。

### 自動デプロイのトリガー対象

以下のファイルが `main` ブランチに push されると自動デプロイが実行される:

- `**.go`
- `go.mod` / `go.sum`
- `Dockerfile`
- `appspec.yml`
- `taskdef.json`
- `.github/workflows/deploy.yml`

---

## 9. クリーンアップ（リソース削除）

不要になった場合、以下の順序で削除する。

```bash
# CDK スタックの削除（全スタック）
cd infra
npx cdk destroy --all

# GitHub Actions 用 IAM ロールの削除
aws iam detach-role-policy --role-name GitHubActionsRole \
  --policy-arn arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPowerUser
aws iam detach-role-policy --role-name GitHubActionsRole \
  --policy-arn arn:aws:iam::aws:policy/AmazonECS_FullAccess
aws iam detach-role-policy --role-name GitHubActionsRole \
  --policy-arn arn:aws:iam::aws:policy/AWSCodeDeployFullAccess
aws iam detach-role-policy --role-name GitHubActionsRole \
  --policy-arn arn:aws:iam::aws:policy/AWSCloudFormationReadOnlyAccess
aws iam delete-role --role-name GitHubActionsRole

# OIDC プロバイダーの削除
OIDC_ARN=$(aws iam list-open-id-connect-providers \
  --query "OpenIDConnectProviderList[?ends_with(Arn,'token.actions.githubusercontent.com')].Arn" \
  --output text)
aws iam delete-open-id-connect-provider --open-id-connect-provider-arn $OIDC_ARN
```

---

## 10. トラブルシューティング

### ECS タスクが起動しない

```bash
# タスクの停止理由を確認
aws ecs list-tasks --cluster text-messaging-dev --desired-status STOPPED --query "taskArns[0]" --output text | \
  xargs -I {} aws ecs describe-tasks --cluster text-messaging-dev --tasks {} \
  --query "tasks[0].stoppedReason" --output text

# CloudWatch Logs を確認
aws logs tail /ecs/text-messaging-dev --follow
```

### データベース接続エラー

```bash
# ECS Exec でコンテナに入って確認
TASK_ARN=$(aws ecs list-tasks --cluster text-messaging-dev --query "taskArns[0]" --output text)

aws ecs execute-command \
  --cluster text-messaging-dev \
  --task $TASK_ARN \
  --container text-messaging-app \
  --interactive \
  --command "/bin/sh"
```

### CodeDeploy デプロイ失敗

```bash
# 最新のデプロイ ID を取得
DEPLOYMENT_ID=$(aws deploy list-deployments \
  --application-name text-messaging-dev \
  --deployment-group-name text-messaging-dev-dg \
  --query "deployments[0]" --output text)

# デプロイ詳細を確認
aws deploy get-deployment \
  --deployment-id $DEPLOYMENT_ID \
  --query "deploymentInfo.{Status:status,Error:errorInformation}" \
  --output table
```

### CDK デプロイ失敗

```bash
# CloudFormation イベントを確認
aws cloudformation describe-stack-events \
  --stack-name TextMessaging-dev-ECS \
  --query "StackEvents[?ResourceStatus=='CREATE_FAILED'].[LogicalResourceId,ResourceStatusReason]" \
  --output table
```

---

## デプロイフロー全体図

```
ステップ 1   AWS CLI 設定
    ↓
ステップ 2   cdk bootstrap（初回のみ）
    ↓
ステップ 3   cdk deploy --all（インフラ構築）  ← 約 20〜25 分
    ↓
ステップ 4   docker build & push（初回イメージ）
    ↓
ステップ 5   ecs update-service（実イメージ適用）
    ↓
ステップ 6   curl で動作確認
    ↓
    ↓         ======= ここまでで手動デプロイ完了 =======
    ↓
ステップ 7   GitHub リポジトリ作成 & push
    ↓
ステップ 8   OIDC + IAM ロール作成
    ↓
ステップ 9   GitHub Secrets 設定
    ↓
    ✅        main ブランチへの push で自動デプロイ
```
