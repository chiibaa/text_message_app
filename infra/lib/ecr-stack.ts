/**
 * =============================================================================
 * ECR Stack - Container Registry
 * =============================================================================
 *
 * 学習ポイント:
 * 1. ECR (Elastic Container Registry)
 *    - Dockerイメージをプライベートに保存
 *    - ECS Fargateから直接プル
 *    - IAMによるアクセス制御
 *
 * 2. ライフサイクルポリシー
 *    - 古いイメージを自動削除
 *    - ストレージコストを削減
 *    - 最新N個のイメージを保持
 *
 * 3. イメージスキャン
 *    - プッシュ時に脆弱性スキャン
 *    - セキュリティベストプラクティス
 * =============================================================================
 */

import * as cdk from 'aws-cdk-lib';
import * as ecr from 'aws-cdk-lib/aws-ecr';
import { Construct } from 'constructs';

/**
 * EcrStack のプロパティ
 */
export interface EcrStackProps extends cdk.StackProps {
  /**
   * 環境名
   */
  readonly environment: string;
}

/**
 * ECRリポジトリを作成するスタック
 *
 * 作成されるリソース:
 * - ECRリポジトリ
 * - ライフサイクルポリシー
 */
export class EcrStack extends cdk.Stack {
  /**
   * 作成されたECRリポジトリ
   */
  public readonly repository: ecr.Repository;

  constructor(scope: Construct, id: string, props: EcrStackProps) {
    super(scope, id, props);

    const { environment } = props;

    // =========================================================================
    // ECR Repository
    // =========================================================================
    // 学習ポイント:
    // - repositoryName: リポジトリの名前（URI に含まれる）
    // - imageScanOnPush: プッシュ時の自動脆弱性スキャン
    // - imageTagMutability: タグの上書き可否
    //   - MUTABLE: 同じタグで上書き可能（:latestなど）
    //   - IMMUTABLE: タグは一度きり（バージョン管理に推奨）
    // - lifecycleRules: 古いイメージの自動削除
    // =========================================================================
    this.repository = new ecr.Repository(this, 'Repository', {
      repositoryName: `text-messaging-app-${environment}`,

      // セキュリティ設定
      imageScanOnPush: true, // プッシュ時に脆弱性スキャン
      imageTagMutability: ecr.TagMutability.MUTABLE, // latestタグを使用可能に

      // スタック削除時の挙動
      // 学習ポイント:
      // - 本番環境ではRETAINを推奨（イメージを保持）
      // - 開発環境ではDESTROYも選択可能
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      emptyOnDelete: true, // 削除時にイメージも削除

      // ライフサイクルポリシー
      // 古いイメージを自動削除してコストを削減
      lifecycleRules: [
        {
          // ルール1: untagged イメージを1日後に削除
          rulePriority: 1,
          description: 'Remove untagged images after 1 day',
          tagStatus: ecr.TagStatus.UNTAGGED,
          maxImageAge: cdk.Duration.days(1),
        },
        {
          // ルール2: タグ付きイメージは最新10個を保持
          rulePriority: 2,
          description: 'Keep only the 10 most recent images',
          tagStatus: ecr.TagStatus.ANY,
          maxImageCount: 10,
        },
      ],
    });

    // =========================================================================
    // Outputs
    // =========================================================================

    // リポジトリURI（docker push/pull で使用）
    new cdk.CfnOutput(this, 'RepositoryUri', {
      value: this.repository.repositoryUri,
      description: 'ECR Repository URI',
      exportName: `${environment}-EcrRepositoryUri`,
    });

    // リポジトリARN（IAMポリシーで使用）
    new cdk.CfnOutput(this, 'RepositoryArn', {
      value: this.repository.repositoryArn,
      description: 'ECR Repository ARN',
      exportName: `${environment}-EcrRepositoryArn`,
    });

    // リポジトリ名（CI/CDスクリプトで使用）
    new cdk.CfnOutput(this, 'RepositoryName', {
      value: this.repository.repositoryName,
      description: 'ECR Repository Name',
      exportName: `${environment}-EcrRepositoryName`,
    });

    // タグ付け
    cdk.Tags.of(this).add('Environment', environment);
    cdk.Tags.of(this).add('Project', 'TextMessagingApp');
  }
}
