#!/bin/bash
# =============================================================================
# infra-down.sh - コスト節約のためインフラを停止
# =============================================================================
# 削除対象: ECS, Database, Network スタック
# 保持対象: ECR, OIDC, IAM ロール, GitHub Actions
#
# 使い方: ./scripts/infra-down.sh
# =============================================================================

set -euo pipefail

ENVIRONMENT="dev"
REGION="ap-northeast-1"

echo "========================================="
echo " インフラ停止 (コスト節約モード)"
echo "========================================="
echo ""
echo "削除するスタック:"
echo "  - TextMessaging-${ENVIRONMENT}-ECS      (ALB, ECS, CodeDeploy)"
echo "  - TextMessaging-${ENVIRONMENT}-Database  (RDS PostgreSQL)"
echo "  - TextMessaging-${ENVIRONMENT}-Network   (VPC, NAT Gateway)"
echo ""
echo "保持するスタック:"
echo "  - TextMessaging-${ENVIRONMENT}-ECR       (コンテナイメージ)"
echo ""

read -p "続行しますか？ (y/N): " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
  echo "中止しました。"
  exit 0
fi

echo ""

# ECS スタック削除 (依存関係の順序: ECS → Database → Network)
echo "[1/3] ECS スタックを削除中..."
cd "$(dirname "$0")/../infra"
npx cdk destroy "TextMessaging-${ENVIRONMENT}-ECS" --force
echo "  ✓ ECS スタック削除完了"

echo ""
echo "[2/3] Database スタックを削除中..."
npx cdk destroy "TextMessaging-${ENVIRONMENT}-Database" --force
echo "  ✓ Database スタック削除完了"

echo ""
echo "[3/3] Network スタックを削除中..."
npx cdk destroy "TextMessaging-${ENVIRONMENT}-Network" --force
echo "  ✓ Network スタック削除完了"

echo ""
echo "========================================="
echo " 完了！"
echo "========================================="
echo ""
echo "月額コスト削減: 約 \$80/月"
echo ""
echo "復旧するには: ./scripts/infra-up.sh"
