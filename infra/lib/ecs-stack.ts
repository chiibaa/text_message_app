/**
 * =============================================================================
 * ECS Stack - Fargate, ALB, CodeDeploy, Auto Scaling
 * =============================================================================
 *
 * 学習ポイント:
 * 1. ECS Fargate
 *    - サーバーレスコンテナ実行環境
 *    - EC2インスタンス管理が不要
 *    - タスク単位でリソースを割り当て
 *
 * 2. Application Load Balancer (ALB)
 *    - レイヤー7ロードバランシング
 *    - ヘルスチェックによる自動復旧
 *    - Blue/Green用のターゲットグループ
 *
 * 3. Blue/Green デプロイ (CodeDeploy)
 *    - ダウンタイムゼロのデプロイ
 *    - 問題時の自動ロールバック
 *    - トラフィックの段階的切り替え
 *
 * 4. Auto Scaling
 *    - CPU/メモリベースの自動スケール
 *    - 最小/最大タスク数の設定
 *    - ターゲット追跡スケーリング
 * =============================================================================
 */

import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as ecr from 'aws-cdk-lib/aws-ecr';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as elbv2 from 'aws-cdk-lib/aws-elasticloadbalancingv2';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as codedeploy from 'aws-cdk-lib/aws-codedeploy';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';

/**
 * EcsStack のプロパティ
 */
export interface EcsStackProps extends cdk.StackProps {
  /**
   * 環境名
   */
  readonly environment: string;

  /**
   * VPC
   */
  readonly vpc: ec2.Vpc;

  /**
   * ALB用セキュリティグループ
   */
  readonly albSecurityGroup: ec2.SecurityGroup;

  /**
   * ECS用セキュリティグループ
   */
  readonly ecsSecurityGroup: ec2.SecurityGroup;

  /**
   * ECRリポジトリ
   */
  readonly ecrRepository: ecr.Repository;

  /**
   * データベースシークレット
   */
  readonly databaseSecret: secretsmanager.ISecret;

  /**
   * データベースエンドポイント
   */
  readonly databaseEndpoint: string;
}

/**
 * ECSサービスと関連リソースを作成するスタック
 *
 * 作成されるリソース:
 * - ECS Cluster
 * - ECS Task Definition
 * - ECS Service (Fargate)
 * - Application Load Balancer
 * - Target Groups (Blue/Green)
 * - CodeDeploy Application & Deployment Group
 * - Auto Scaling
 */
export class EcsStack extends cdk.Stack {
  /**
   * ALB DNS名（アプリケーションアクセス用）
   */
  public readonly albDnsName: string;

  /**
   * ECSクラスター
   */
  public readonly cluster: ecs.Cluster;

  /**
   * ECSサービス
   */
  public readonly service: ecs.FargateService;

  constructor(scope: Construct, id: string, props: EcsStackProps) {
    super(scope, id, props);

    const {
      environment,
      vpc,
      albSecurityGroup,
      ecsSecurityGroup,
      ecrRepository,
      databaseSecret,
      databaseEndpoint,
    } = props;

    // =========================================================================
    // ECS Cluster
    // =========================================================================
    // 学習ポイント:
    // - クラスターはタスクの論理的なグループ
    // - Container Insightsでモニタリング可能
    // =========================================================================
    this.cluster = new ecs.Cluster(this, 'Cluster', {
      clusterName: `text-messaging-${environment}`,
      vpc,
      containerInsightsV2: ecs.ContainerInsights.ENABLED, // CloudWatch Container Insightsを有効化
    });

    // =========================================================================
    // IAM Roles
    // =========================================================================
    // 学習ポイント:
    // - タスク実行ロール: ECSエージェントが使用（ECRプル、ログ出力など）
    // - タスクロール: アプリケーションコンテナが使用（AWSサービスアクセス）
    // =========================================================================

    // タスク実行ロール
    const executionRole = new iam.Role(this, 'TaskExecutionRole', {
      roleName: `text-messaging-${environment}-execution-role`,
      assumedBy: new iam.ServicePrincipal('ecs-tasks.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName(
          'service-role/AmazonECSTaskExecutionRolePolicy'
        ),
      ],
    });

    // Secrets Manager からシークレットを読み取る権限を追加
    databaseSecret.grantRead(executionRole);

    // タスクロール（アプリケーション用）
    const taskRole = new iam.Role(this, 'TaskRole', {
      roleName: `text-messaging-${environment}-task-role`,
      assumedBy: new iam.ServicePrincipal('ecs-tasks.amazonaws.com'),
    });

    // =========================================================================
    // CloudWatch Logs
    // =========================================================================
    const logGroup = new logs.LogGroup(this, 'LogGroup', {
      logGroupName: `/ecs/text-messaging-${environment}`,
      retention: logs.RetentionDays.ONE_WEEK,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });

    // =========================================================================
    // ECS Task Definition
    // =========================================================================
    // 学習ポイント:
    // - cpu/memoryLimitMiB: タスクに割り当てるリソース
    // - networkMode: awsvpc（Fargateの要件）
    // - containerDefinitions: 実行するコンテナの定義
    // =========================================================================
    const taskDefinition = new ecs.FargateTaskDefinition(this, 'TaskDefinition', {
      family: `text-messaging-${environment}`,
      cpu: 256, // 0.25 vCPU
      memoryLimitMiB: 512, // 512 MB
      executionRole,
      taskRole,
    });

    // アプリケーションコンテナを追加
    const container = taskDefinition.addContainer('app', {
      containerName: 'text-messaging-app',
      // ECRリポジトリのイメージを使用
      image: ecs.ContainerImage.fromEcrRepository(ecrRepository, 'latest'),

      // 環境変数
      environment: {
        PORT: '8080',
        STORAGE_TYPE: 'postgres',
      },

      // シークレット（Secrets Managerから取得）
      // RDS自動生成シークレットのキー: host, port, username, password, dbname, engine
      secrets: {
        DB_HOST: ecs.Secret.fromSecretsManager(databaseSecret, 'host'),
        DB_PORT: ecs.Secret.fromSecretsManager(databaseSecret, 'port'),
        DB_USERNAME: ecs.Secret.fromSecretsManager(databaseSecret, 'username'),
        DB_PASSWORD: ecs.Secret.fromSecretsManager(databaseSecret, 'password'),
        DB_NAME: ecs.Secret.fromSecretsManager(databaseSecret, 'dbname'),
      },

      // ログ設定
      logging: ecs.LogDrivers.awsLogs({
        streamPrefix: 'app',
        logGroup,
      }),

      // ヘルスチェック
      healthCheck: {
        command: ['CMD-SHELL', 'curl -f http://localhost:8080/health || exit 1'],
        interval: cdk.Duration.seconds(30),
        timeout: cdk.Duration.seconds(5),
        retries: 3,
        startPeriod: cdk.Duration.seconds(60),
      },
    });

    // ポートマッピング
    container.addPortMappings({
      containerPort: 8080,
      protocol: ecs.Protocol.TCP,
    });

    // =========================================================================
    // Application Load Balancer
    // =========================================================================
    // 学習ポイント:
    // - インターネット向けALB（Public Subnetに配置）
    // - HTTP/HTTPSリスナーでトラフィックを受け付け
    // - ターゲットグループでECSタスクにルーティング
    // =========================================================================
    const alb = new elbv2.ApplicationLoadBalancer(this, 'ALB', {
      loadBalancerName: `text-messaging-${environment}-alb`,
      vpc,
      internetFacing: true, // インターネットからアクセス可能
      securityGroup: albSecurityGroup,
      vpcSubnets: {
        subnetType: ec2.SubnetType.PUBLIC,
      },
    });

    this.albDnsName = alb.loadBalancerDnsName;

    // =========================================================================
    // Target Groups (Blue/Green)
    // =========================================================================
    // 学習ポイント:
    // - Blue: 現在の本番トラフィック
    // - Green: 新バージョンのデプロイ先
    // - CodeDeployがトラフィックを切り替え
    // =========================================================================

    // Blue Target Group（本番用）
    const blueTargetGroup = new elbv2.ApplicationTargetGroup(this, 'BlueTargetGroup', {
      targetGroupName: `text-msg-${environment}-blue`,
      vpc,
      port: 8080,
      protocol: elbv2.ApplicationProtocol.HTTP,
      targetType: elbv2.TargetType.IP, // Fargate = IP ターゲット
      healthCheck: {
        path: '/health',
        protocol: elbv2.Protocol.HTTP,
        healthyHttpCodes: '200',
        interval: cdk.Duration.seconds(30),
        timeout: cdk.Duration.seconds(5),
        healthyThresholdCount: 2,
        unhealthyThresholdCount: 3,
      },
      deregistrationDelay: cdk.Duration.seconds(30), // ドレイン時間
    });

    // Green Target Group（新バージョン用）
    const greenTargetGroup = new elbv2.ApplicationTargetGroup(this, 'GreenTargetGroup', {
      targetGroupName: `text-msg-${environment}-green`,
      vpc,
      port: 8080,
      protocol: elbv2.ApplicationProtocol.HTTP,
      targetType: elbv2.TargetType.IP,
      healthCheck: {
        path: '/health',
        protocol: elbv2.Protocol.HTTP,
        healthyHttpCodes: '200',
        interval: cdk.Duration.seconds(30),
        timeout: cdk.Duration.seconds(5),
        healthyThresholdCount: 2,
        unhealthyThresholdCount: 3,
      },
      deregistrationDelay: cdk.Duration.seconds(30),
    });

    // =========================================================================
    // ALB Listeners
    // =========================================================================
    // 学習ポイント:
    // - 本番リスナー（ポート80）: ユーザートラフィック
    // - テストリスナー（ポート8080）: デプロイ時のテスト用
    // =========================================================================

    // 本番リスナー（HTTP:80）
    const prodListener = alb.addListener('ProdListener', {
      port: 80,
      protocol: elbv2.ApplicationProtocol.HTTP,
      defaultTargetGroups: [blueTargetGroup],
    });

    // テストリスナー（HTTP:8080）- Blue/Greenデプロイ時のテスト用
    const testListener = alb.addListener('TestListener', {
      port: 8080,
      protocol: elbv2.ApplicationProtocol.HTTP,
      defaultTargetGroups: [greenTargetGroup],
    });

    // =========================================================================
    // ECS Service
    // =========================================================================
    // 学習ポイント:
    // - deploymentController: CODE_DEPLOY でBlue/Greenを使用
    // - desiredCount: 起動するタスク数
    // - assignPublicIp: false（Private Subnetのため）
    // =========================================================================
    this.service = new ecs.FargateService(this, 'Service', {
      serviceName: `text-messaging-${environment}`,
      cluster: this.cluster,
      taskDefinition,
      desiredCount: 1, // 初期タスク数

      // Blue/Green デプロイ用の設定
      deploymentController: {
        type: ecs.DeploymentControllerType.CODE_DEPLOY,
      },

      // ネットワーク設定
      securityGroups: [ecsSecurityGroup],
      vpcSubnets: {
        subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS,
      },
      assignPublicIp: false, // Private Subnetのため不要

      // ヘルスチェック猶予期間
      healthCheckGracePeriod: cdk.Duration.seconds(60),

      // 注: circuitBreaker は CODE_DEPLOY デプロイコントローラーと併用不可
      // Blue/Green デプロイでは CodeDeploy 側の autoRollback が同等の役割を担う

      enableExecuteCommand: true, // ECS Exec を有効化（デバッグ用）
    });

    // サービスをBlueターゲットグループに登録
    this.service.attachToApplicationTargetGroup(blueTargetGroup);

    // =========================================================================
    // Auto Scaling
    // =========================================================================
    // 学習ポイント:
    // - ターゲット追跡スケーリング: メトリクスが目標値を維持するようにスケール
    // - CPU使用率70%を目標に自動調整
    // - 最小1、最大4タスクの範囲で変動
    // =========================================================================
    const scaling = this.service.autoScaleTaskCount({
      minCapacity: 1,
      maxCapacity: 4, // dev環境はコスト考慮で控えめ
    });

    // CPU使用率ベースのスケーリング
    scaling.scaleOnCpuUtilization('CpuScaling', {
      targetUtilizationPercent: 70,
      scaleInCooldown: cdk.Duration.seconds(300), // スケールイン待機時間
      scaleOutCooldown: cdk.Duration.seconds(60), // スケールアウト待機時間
    });

    // メモリ使用率ベースのスケーリング
    scaling.scaleOnMemoryUtilization('MemoryScaling', {
      targetUtilizationPercent: 70,
      scaleInCooldown: cdk.Duration.seconds(300),
      scaleOutCooldown: cdk.Duration.seconds(60),
    });

    // =========================================================================
    // CodeDeploy
    // =========================================================================
    // 学習ポイント:
    // - EcsApplication: CodeDeployのアプリケーション定義
    // - EcsDeploymentGroup: デプロイグループ（Blue/Green設定を含む）
    // - deploymentConfig: トラフィック切り替え方式
    //   - AllAtOnce: 即時切り替え
    //   - Linear/Canary: 段階的切り替え（本番推奨）
    // =========================================================================

    // CodeDeploy Application
    const codeDeployApp = new codedeploy.EcsApplication(this, 'CodeDeployApp', {
      applicationName: `text-messaging-${environment}`,
    });

    // CodeDeploy Deployment Group
    const deploymentGroup = new codedeploy.EcsDeploymentGroup(this, 'DeploymentGroup', {
      application: codeDeployApp,
      deploymentGroupName: `text-messaging-${environment}-dg`,
      service: this.service,

      // Blue/Green 設定
      blueGreenDeploymentConfig: {
        blueTargetGroup,
        greenTargetGroup,
        listener: prodListener,
        testListener,
        terminationWaitTime: cdk.Duration.minutes(5), // 旧タスク終了待機
      },

      // デプロイ設定
      // 学習ポイント:
      // - ALL_AT_ONCE: 即時切り替え（学習用）
      // - LINEAR_10PERCENT_EVERY_1MINUTES: 1分ごとに10%ずつ
      // - CANARY_10PERCENT_5MINUTES: 最初10%、5分後に残り
      deploymentConfig: codedeploy.EcsDeploymentConfig.ALL_AT_ONCE,

      // 自動ロールバック設定
      autoRollback: {
        failedDeployment: true, // デプロイ失敗時
        stoppedDeployment: true, // 手動停止時
        deploymentInAlarm: false, // アラーム発生時（アラーム未設定のため無効）
      },
    });

    // =========================================================================
    // Outputs
    // =========================================================================

    new cdk.CfnOutput(this, 'AlbDnsName', {
      value: this.albDnsName,
      description: 'Application Load Balancer DNS Name',
      exportName: `${environment}-AlbDnsName`,
    });

    new cdk.CfnOutput(this, 'AlbArn', {
      value: alb.loadBalancerArn,
      description: 'ALB ARN',
      exportName: `${environment}-AlbArn`,
    });

    new cdk.CfnOutput(this, 'ClusterName', {
      value: this.cluster.clusterName,
      description: 'ECS Cluster Name',
      exportName: `${environment}-ClusterName`,
    });

    new cdk.CfnOutput(this, 'ServiceName', {
      value: this.service.serviceName,
      description: 'ECS Service Name',
      exportName: `${environment}-ServiceName`,
    });

    new cdk.CfnOutput(this, 'CodeDeployAppName', {
      value: codeDeployApp.applicationName,
      description: 'CodeDeploy Application Name',
      exportName: `${environment}-CodeDeployAppName`,
    });

    new cdk.CfnOutput(this, 'CodeDeployDeploymentGroupName', {
      value: deploymentGroup.deploymentGroupName,
      description: 'CodeDeploy Deployment Group Name',
      exportName: `${environment}-CodeDeployDeploymentGroupName`,
    });

    new cdk.CfnOutput(this, 'TaskDefinitionFamily', {
      value: taskDefinition.family,
      description: 'ECS Task Definition Family',
      exportName: `${environment}-TaskDefinitionFamily`,
    });

    // タグ付け
    cdk.Tags.of(this).add('Environment', environment);
    cdk.Tags.of(this).add('Project', 'TextMessagingApp');
  }
}
