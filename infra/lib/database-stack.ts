/**
 * =============================================================================
 * Database Stack - RDS PostgreSQL
 * =============================================================================
 *
 * 学習ポイント:
 * 1. RDS インスタンスの構成
 *    - PostgreSQL 16 を使用
 *    - Isolated Subnetに配置（セキュリティ）
 *    - 自動バックアップ、メンテナンスウィンドウ
 *
 * 2. Secrets Manager 連携
 *    - パスワードを自動生成・安全に管理
 *    - ECSタスクからシークレットを参照
 *
 * 3. コスト最適化（dev環境）
 *    - t3.micro インスタンス
 *    - シングルAZ（Multi-AZなし）
 *    - ストレージ自動スケーリング
 * =============================================================================
 */

import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as rds from 'aws-cdk-lib/aws-rds';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';

/**
 * DatabaseStack のプロパティ
 */
export interface DatabaseStackProps extends cdk.StackProps {
  /**
   * 環境名
   */
  readonly environment: string;

  /**
   * NetworkStackで作成されたVPC
   */
  readonly vpc: ec2.Vpc;

  /**
   * RDS用セキュリティグループ
   */
  readonly rdsSecurityGroup: ec2.SecurityGroup;
}

/**
 * RDS PostgreSQL インスタンスを作成するスタック
 *
 * 作成されるリソース:
 * - RDS PostgreSQL インスタンス
 * - Secrets Manager シークレット（DB認証情報）
 * - サブネットグループ（Isolated Subnet使用）
 */
export class DatabaseStack extends cdk.Stack {
  /**
   * RDSインスタンス（他のスタックから参照）
   */
  public readonly database: rds.DatabaseInstance;

  /**
   * データベース認証情報のシークレット
   */
  public readonly databaseSecret: secretsmanager.ISecret;

  /**
   * データベース接続エンドポイント
   */
  public readonly databaseEndpoint: string;

  constructor(scope: Construct, id: string, props: DatabaseStackProps) {
    super(scope, id, props);

    const { environment, vpc, rdsSecurityGroup } = props;

    // =========================================================================
    // Database Credentials (Secrets Manager)
    // =========================================================================
    // 学習ポイント:
    // - CDKがシークレットを自動生成
    // - パスワードはSecrets Managerに安全に保存
    // - ECSタスクはこのシークレットを参照してDB接続
    // =========================================================================

    // 注: RDSインスタンスが自動的にシークレットを作成するため、
    // 明示的なシークレット作成は不要

    // =========================================================================
    // RDS PostgreSQL Instance
    // =========================================================================
    // 学習ポイント:
    // - engine: データベースエンジンとバージョン
    // - instanceType: インスタンスサイズ（コストに直結）
    // - allocatedStorage: 初期ストレージサイズ
    // - maxAllocatedStorage: 自動スケーリングの上限
    // - vpcSubnets: 配置するサブネット（Isolated推奨）
    // - multiAz: 高可用性（本番ではtrue推奨）
    // =========================================================================
    this.database = new rds.DatabaseInstance(this, 'Database', {
      // データベースの識別子
      instanceIdentifier: `text-messaging-${environment}`,
      databaseName: 'textmessaging', // 初期データベース名

      // エンジン設定
      engine: rds.DatabaseInstanceEngine.postgres({
        version: rds.PostgresEngineVersion.VER_16_4,
      }),

      // インスタンス設定（dev環境向け）
      instanceType: ec2.InstanceType.of(
        ec2.InstanceClass.T3,
        ec2.InstanceSize.MICRO, // 開発用（無料利用枠対象）
      ),

      // 認証情報（自動生成）
      credentials: rds.Credentials.fromGeneratedSecret('postgres', {
        secretName: `text-messaging-${environment}/db-credentials`,
      }),

      // ネットワーク設定
      vpc,
      vpcSubnets: {
        subnetType: ec2.SubnetType.PRIVATE_ISOLATED, // 最もセキュアなサブネット
      },
      securityGroups: [rdsSecurityGroup],

      // 高可用性設定（dev環境はコスト削減のためオフ）
      multiAz: false,

      // ストレージ設定
      allocatedStorage: 20, // 20GB（最小）
      maxAllocatedStorage: 100, // 自動スケーリング上限（GB）
      storageType: rds.StorageType.GP3, // 汎用SSD（最新世代）
      storageEncrypted: true, // 暗号化を有効化

      // バックアップ設定
      backupRetention: cdk.Duration.days(7), // 7日間保持
      deleteAutomatedBackups: true, // スタック削除時にバックアップも削除

      // メンテナンス設定
      preferredMaintenanceWindow: 'Sun:03:00-Sun:04:00', // 日曜深夜
      preferredBackupWindow: '02:00-03:00', // バックアップは2-3時

      // パラメータグループ（デフォルト設定を使用）
      parameterGroup: new rds.ParameterGroup(this, 'ParameterGroup', {
        engine: rds.DatabaseInstanceEngine.postgres({
          version: rds.PostgresEngineVersion.VER_16_4,
        }),
        description: `Parameter group for text-messaging-${environment}`,
        parameters: {
          // ログ設定
          'log_statement': 'all', // 全SQLをログ（開発用）
          'log_min_duration_statement': '1000', // 1秒以上のクエリをログ
        },
      }),

      // スタック削除時の挙動
      // 学習ポイント:
      // - SNAPSHOT: 削除前にスナップショットを作成（本番推奨）
      // - DESTROY: 即座に削除（開発用）
      // - RETAIN: 削除せず保持
      removalPolicy: cdk.RemovalPolicy.DESTROY, // dev環境のため削除を許可

      // CloudWatch Logs エクスポート
      cloudwatchLogsExports: ['postgresql'], // PostgreSQLログをCloudWatchに出力
      cloudwatchLogsRetention: cdk.aws_logs.RetentionDays.ONE_WEEK,

      // パフォーマンスインサイト（無効化でコスト削減）
      enablePerformanceInsights: false,

      // 自動マイナーバージョンアップグレード
      autoMinorVersionUpgrade: true,

      // パブリックアクセス不可
      publiclyAccessible: false,
    });

    // シークレットへの参照を保存
    this.databaseSecret = this.database.secret!;
    this.databaseEndpoint = this.database.instanceEndpoint.hostname;

    // =========================================================================
    // Outputs
    // =========================================================================

    new cdk.CfnOutput(this, 'DatabaseEndpoint', {
      value: this.database.instanceEndpoint.hostname,
      description: 'RDS PostgreSQL Endpoint',
      exportName: `${environment}-DatabaseEndpoint`,
    });

    new cdk.CfnOutput(this, 'DatabasePort', {
      value: this.database.instanceEndpoint.port.toString(),
      description: 'RDS PostgreSQL Port',
      exportName: `${environment}-DatabasePort`,
    });

    new cdk.CfnOutput(this, 'DatabaseSecretArn', {
      value: this.databaseSecret.secretArn,
      description: 'Database credentials secret ARN',
      exportName: `${environment}-DatabaseSecretArn`,
    });

    new cdk.CfnOutput(this, 'DatabaseName', {
      value: 'textmessaging',
      description: 'Database name',
      exportName: `${environment}-DatabaseName`,
    });

    // タグ付け
    cdk.Tags.of(this).add('Environment', environment);
    cdk.Tags.of(this).add('Project', 'TextMessagingApp');
  }
}
