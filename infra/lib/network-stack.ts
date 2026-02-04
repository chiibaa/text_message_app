/**
 * =============================================================================
 * Network Stack - VPC, Subnet, Security Group
 * =============================================================================
 *
 * 学習ポイント:
 * 1. VPCの構成とサブネット分離
 *    - Public Subnet: インターネットからアクセス可能（ALB用）
 *    - Private Subnet: NATゲートウェイ経由で外部アクセス（ECS用）
 *    - Isolated Subnet: 外部アクセス不可（RDS用）
 *
 * 2. セキュリティグループ
 *    - 最小権限の原則: 必要な通信のみ許可
 *    - ALB → ECS → RDS の通信フロー
 *
 * 3. マルチAZ構成
 *    - 2つのAZに分散して高可用性を確保
 * =============================================================================
 */

import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import { Construct } from 'constructs';

/**
 * NetworkStack のプロパティ
 */
export interface NetworkStackProps extends cdk.StackProps {
  /**
   * 環境名（dev, staging, prod など）
   * リソース命名に使用
   */
  readonly environment: string;
}

/**
 * ネットワークリソースを作成するスタック
 *
 * 作成されるリソース:
 * - VPC (2 AZ)
 * - Public Subnet (ALB用)
 * - Private Subnet (ECS用)
 * - Isolated Subnet (RDS用)
 * - NAT Gateway (Private Subnetからの外部アクセス用)
 * - Security Groups (ALB, ECS, RDS)
 */
export class NetworkStack extends cdk.Stack {
  /**
   * 作成されたVPC（他のスタックから参照）
   */
  public readonly vpc: ec2.Vpc;

  /**
   * ALB用セキュリティグループ
   */
  public readonly albSecurityGroup: ec2.SecurityGroup;

  /**
   * ECS用セキュリティグループ
   */
  public readonly ecsSecurityGroup: ec2.SecurityGroup;

  /**
   * RDS用セキュリティグループ
   */
  public readonly rdsSecurityGroup: ec2.SecurityGroup;

  constructor(scope: Construct, id: string, props: NetworkStackProps) {
    super(scope, id, props);

    const { environment } = props;

    // =========================================================================
    // VPC
    // =========================================================================
    // 学習ポイント:
    // - maxAzs: 使用するAZ数（2つで冗長性を確保）
    // - subnetConfiguration: サブネットの種類と構成
    //   - PUBLIC: インターネットゲートウェイへのルートあり
    //   - PRIVATE_WITH_EGRESS: NAT Gateway経由で外部アクセス可能
    //   - PRIVATE_ISOLATED: 外部アクセス不可（最もセキュア）
    // - natGateways: コスト削減のため1つに制限（本番では2つ推奨）
    // =========================================================================
    this.vpc = new ec2.Vpc(this, 'Vpc', {
      vpcName: `text-messaging-${environment}-vpc`,
      maxAzs: 2, // 2つのアベイラビリティゾーンを使用
      natGateways: 1, // コスト削減（dev環境のため）

      // サブネット構成
      subnetConfiguration: [
        {
          // Public Subnet: ALBを配置
          // インターネットからのトラフィックを受け付ける
          name: 'Public',
          subnetType: ec2.SubnetType.PUBLIC,
          cidrMask: 24, // /24 = 256 IPアドレス
        },
        {
          // Private Subnet: ECS Fargateタスクを配置
          // NAT Gateway経由で外部API呼び出し可能
          name: 'Private',
          subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS,
          cidrMask: 24,
        },
        {
          // Isolated Subnet: RDSを配置
          // インターネットアクセス不可（セキュリティ最優先）
          name: 'Isolated',
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
          cidrMask: 24,
        },
      ],

      // VPC内でDNS解決を有効化（RDSエンドポイント解決に必要）
      enableDnsHostnames: true,
      enableDnsSupport: true,
    });

    // =========================================================================
    // Security Groups
    // =========================================================================
    // 学習ポイント:
    // - セキュリティグループはステートフル（戻りトラフィックは自動許可）
    // - インバウンドルールのみ設定すればOK
    // - 最小権限: 必要な通信のみを明示的に許可
    // =========================================================================

    // ALB用セキュリティグループ
    // インターネットからHTTP/HTTPSを受け付ける
    this.albSecurityGroup = new ec2.SecurityGroup(this, 'AlbSecurityGroup', {
      vpc: this.vpc,
      securityGroupName: `text-messaging-${environment}-alb-sg`,
      description: 'Security group for ALB - allows HTTP/HTTPS from internet',
      allowAllOutbound: true, // アウトバウンドは全て許可
    });

    // HTTP (ポート80) を全てのIPから許可
    this.albSecurityGroup.addIngressRule(
      ec2.Peer.anyIpv4(),
      ec2.Port.tcp(80),
      'Allow HTTP from anywhere'
    );

    // HTTPS (ポート443) を全てのIPから許可
    // 注: 本番環境では証明書設定が必要
    this.albSecurityGroup.addIngressRule(
      ec2.Peer.anyIpv4(),
      ec2.Port.tcp(443),
      'Allow HTTPS from anywhere'
    );

    // ECS用セキュリティグループ
    // ALBからのみトラフィックを受け付ける
    this.ecsSecurityGroup = new ec2.SecurityGroup(this, 'EcsSecurityGroup', {
      vpc: this.vpc,
      securityGroupName: `text-messaging-${environment}-ecs-sg`,
      description: 'Security group for ECS tasks - allows traffic from ALB only',
      allowAllOutbound: true, // 外部API呼び出し用
    });

    // ALBからのトラフィックのみ許可（ポート8080: アプリケーションポート）
    this.ecsSecurityGroup.addIngressRule(
      this.albSecurityGroup,
      ec2.Port.tcp(8080),
      'Allow traffic from ALB'
    );

    // RDS用セキュリティグループ
    // ECSタスクからのみ接続を受け付ける
    this.rdsSecurityGroup = new ec2.SecurityGroup(this, 'RdsSecurityGroup', {
      vpc: this.vpc,
      securityGroupName: `text-messaging-${environment}-rds-sg`,
      description: 'Security group for RDS - allows PostgreSQL from ECS only',
      allowAllOutbound: false, // RDSは外部接続不要
    });

    // ECSからのPostgreSQL接続のみ許可（ポート5432）
    this.rdsSecurityGroup.addIngressRule(
      this.ecsSecurityGroup,
      ec2.Port.tcp(5432),
      'Allow PostgreSQL from ECS tasks'
    );

    // =========================================================================
    // Outputs
    // =========================================================================
    // 他のスタックやデプロイスクリプトから参照するための出力
    // =========================================================================

    new cdk.CfnOutput(this, 'VpcId', {
      value: this.vpc.vpcId,
      description: 'VPC ID',
      exportName: `${environment}-VpcId`,
    });

    new cdk.CfnOutput(this, 'PublicSubnetIds', {
      value: this.vpc.publicSubnets.map(s => s.subnetId).join(','),
      description: 'Public Subnet IDs (comma-separated)',
      exportName: `${environment}-PublicSubnetIds`,
    });

    new cdk.CfnOutput(this, 'PrivateSubnetIds', {
      value: this.vpc.privateSubnets.map(s => s.subnetId).join(','),
      description: 'Private Subnet IDs (comma-separated)',
      exportName: `${environment}-PrivateSubnetIds`,
    });

    new cdk.CfnOutput(this, 'IsolatedSubnetIds', {
      value: this.vpc.isolatedSubnets.map(s => s.subnetId).join(','),
      description: 'Isolated Subnet IDs (comma-separated)',
      exportName: `${environment}-IsolatedSubnetIds`,
    });

    // リソースにタグを追加
    cdk.Tags.of(this).add('Environment', environment);
    cdk.Tags.of(this).add('Project', 'TextMessagingApp');
  }
}
