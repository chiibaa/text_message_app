#!/bin/bash
# =============================================================================
# infra-up.sh - インフラを再構築
# =============================================================================
# CDK デプロイ → Docker イメージ push → 動作確認
# 所要時間: 約 25 分
#
# 使い方: ./scripts/infra-up.sh
# =============================================================================

set -euo pipefail

ENVIRONMENT="dev"
REGION="ap-northeast-1"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_REPO="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com/text-messaging-app-${ENVIRONMENT}"

echo "========================================="
echo " インフラ再構築"
echo "========================================="
echo ""
echo "AWS Account: ${ACCOUNT_ID}"
echo "Region:      ${REGION}"
echo "ECR Repo:    ${ECR_REPO}"
echo ""

# 1. CDK デプロイ
echo "[1/4] CDK デプロイ中... (約 20〜25 分)"
cd "$(dirname "$0")/../infra"
npx cdk deploy --all --require-approval never
echo "  ✓ CDK デプロイ完了"

echo ""

# 2. ECR ログイン
echo "[2/4] ECR ログイン..."
aws ecr get-login-password --region "${REGION}" | \
  docker login --username AWS --password-stdin \
  "${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"
echo "  ✓ ECR ログイン完了"

echo ""

# 3. Docker イメージ ビルド & プッシュ
echo "[3/4] Docker イメージ ビルド & プッシュ..."
cd "$(dirname "$0")/.."
docker build --platform linux/amd64 -t text-messaging-app .
docker tag text-messaging-app:latest "${ECR_REPO}:latest"
docker push "${ECR_REPO}:latest"
echo "  ✓ Docker イメージ プッシュ完了"

echo ""

# 4. 動作確認
echo "[4/4] 動作確認..."
ALB_DNS=$(aws cloudformation describe-stacks \
  --stack-name "TextMessaging-${ENVIRONMENT}-ECS" \
  --query "Stacks[0].Outputs[?OutputKey=='AlbDnsName'].OutputValue" \
  --output text)

echo "  ALB DNS: ${ALB_DNS}"
echo "  ヘルスチェック中..."

for i in {1..10}; do
  if curl -sf "http://${ALB_DNS}/health" > /dev/null 2>&1; then
    echo "  ✓ ヘルスチェック OK"
    break
  fi
  if [ "$i" -eq 10 ]; then
    echo "  ✗ ヘルスチェック失敗 (手動で確認してください)"
    echo "    curl http://${ALB_DNS}/health"
    exit 1
  fi
  echo "  待機中... (${i}/10)"
  sleep 15
done

echo ""
echo "========================================="
echo " 完了！"
echo "========================================="
echo ""
echo "アプリケーション URL: http://${ALB_DNS}"
echo "ヘルスチェック:       http://${ALB_DNS}/health"
echo "API:                  http://${ALB_DNS}/messages"
echo ""
echo "停止するには: ./scripts/infra-down.sh"
