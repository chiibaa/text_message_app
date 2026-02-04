#!/usr/bin/env node
/**
 * =============================================================================
 * CDK Application Entry Point
 * =============================================================================
 *
 * 学習ポイント:
 * 1. CDK App
 *    - 全てのスタックを含むアプリケーション
 *    - `cdk deploy --all` で全スタックをデプロイ
 *
 * 2. スタック依存関係
 *    - EcsStack は NetworkStack, DatabaseStack, EcrStack に依存
 *    - CDKが自動的にデプロイ順序を決定
 *
 * 3. 環境設定
 *    - env: AWSアカウントとリージョンを指定
 *    - 環境変数から取得するか、デフォルト値を使用
 * =============================================================================
 */

import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { NetworkStack } from '../lib/network-stack';
import { DatabaseStack } from '../lib/database-stack';
import { EcrStack } from '../lib/ecr-stack';
import { EcsStack } from '../lib/ecs-stack';

// =============================================================================
// 設定
// =============================================================================

// 環境名（環境変数から取得、デフォルトは 'dev'）
const environment = process.env.ENVIRONMENT || 'dev';

// AWSアカウント・リージョン設定
// 学習ポイント:
// - CDK_DEFAULT_ACCOUNT: `cdk bootstrap` 時に設定されるデフォルトアカウント
// - CDK_DEFAULT_REGION: `cdk bootstrap` 時に設定されるデフォルトリージョン
// - 明示的に指定することも可能（本番環境推奨）
const env: cdk.Environment = {
  account: process.env.CDK_DEFAULT_ACCOUNT || process.env.AWS_ACCOUNT_ID,
  region: process.env.CDK_DEFAULT_REGION || process.env.AWS_REGION || 'ap-northeast-1',
};

// =============================================================================
// CDK App
// =============================================================================

const app = new cdk.App();

// =============================================================================
// Stack Instantiation
// =============================================================================
// 学習ポイント:
// - スタックは ID で識別される（CloudFormation スタック名になる）
// - props で設定と依存関係を渡す
// - 依存関係があるスタックは自動的に順序付けされる
// =============================================================================

// 1. Network Stack（最初に作成：他のスタックが依存）
const networkStack = new NetworkStack(app, `TextMessaging-${environment}-Network`, {
  stackName: `TextMessaging-${environment}-Network`,
  environment,
  env,
  description: 'Text Messaging App - VPC, Subnets, Security Groups',

  // スタック削除時の保護（本番環境では有効化推奨）
  terminationProtection: false,
});

// 2. Database Stack（Network Stackに依存）
const databaseStack = new DatabaseStack(app, `TextMessaging-${environment}-Database`, {
  stackName: `TextMessaging-${environment}-Database`,
  environment,
  env,
  description: 'Text Messaging App - RDS PostgreSQL',

  // Network Stack からの参照
  vpc: networkStack.vpc,
  rdsSecurityGroup: networkStack.rdsSecurityGroup,

  terminationProtection: false,
});

// 明示的な依存関係（通常は自動検出されるが、明示的に指定）
databaseStack.addDependency(networkStack);

// 3. ECR Stack（独立：他のスタックに依存しない）
const ecrStack = new EcrStack(app, `TextMessaging-${environment}-ECR`, {
  stackName: `TextMessaging-${environment}-ECR`,
  environment,
  env,
  description: 'Text Messaging App - ECR Repository',

  terminationProtection: false,
});

// 4. ECS Stack（Network, Database, ECR に依存）
const ecsStack = new EcsStack(app, `TextMessaging-${environment}-ECS`, {
  stackName: `TextMessaging-${environment}-ECS`,
  environment,
  env,
  description: 'Text Messaging App - ECS Fargate, ALB, CodeDeploy',

  // 他のスタックからの参照
  vpc: networkStack.vpc,
  albSecurityGroup: networkStack.albSecurityGroup,
  ecsSecurityGroup: networkStack.ecsSecurityGroup,
  ecrRepository: ecrStack.repository,
  databaseSecret: databaseStack.databaseSecret,
  databaseEndpoint: databaseStack.databaseEndpoint,

  terminationProtection: false,
});

// 明示的な依存関係
ecsStack.addDependency(networkStack);
ecsStack.addDependency(databaseStack);
ecsStack.addDependency(ecrStack);

// =============================================================================
// Tags
// =============================================================================
// 学習ポイント:
// - タグは全リソースに適用される
// - コスト管理、リソース整理に重要
// =============================================================================

cdk.Tags.of(app).add('Application', 'TextMessagingApp');
cdk.Tags.of(app).add('Environment', environment);
cdk.Tags.of(app).add('ManagedBy', 'CDK');

// =============================================================================
// Aspects（オプション）
// =============================================================================
// 学習ポイント:
// - Aspects を使って全リソースにポリシーを適用可能
// - 例: 全S3バケットのバージョニングを強制
// =============================================================================

// 例: 全てのLambda関数にタイムアウト制限を適用
// cdk.Aspects.of(app).add(new LambdaTimeoutAspect(30));

// CDK シンセサイズ（テンプレート生成）
app.synth();
