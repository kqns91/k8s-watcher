# kube-watcher

Namespace限定権限で動作する、軽量なKubernetesリソース監視Bot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/)

## 概要

`kube-watcher`は、Kubernetesクラスタ内の特定のNamespace内でリソースの変更を監視し、Slackへ通知を送信する軽量な監視Botです。

BotKubeやRobustaなどの既存ツールは**ClusterRole**（クラスタ全体への権限）が必要ですが、`kube-watcher`は**Namespace限定のRole**のみで動作するため、厳格なRBACポリシーが適用されている環境でも安全にご利用いただけます。

## 主な特徴

- **🔒 セキュア**: ClusterRole不要、Namespace限定のRole権限のみで動作
- **🔍 柔軟な監視**: Pod、Deployment、Serviceなど複数のリソースタイプに対応
- **⚙️ 設定可能なフィルター**: イベントタイプ（作成/更新/削除）やラベルによるフィルタリング
- **✨ カスタマイズ可能**: Goテンプレートを使用したSlackメッセージのカスタマイズ
- **🪶 軽量**: 最小限のリソースフットプリント、シンプルな依存関係

## アーキテクチャ

```
┌─────────────┐
│ Kubernetes  │
│   API       │
└──────┬──────┘
       │
       │ (informer/watch)
       │
┌──────▼──────┐
│   Watcher   │  リソース変更の検知
└──────┬──────┘
       │
       │ (events)
       │
┌──────▼──────┐
│   Filter    │  設定に基づくフィルタリング
└──────┬──────┘
       │
       │ (filtered events)
       │
┌──────▼──────┐
│  Formatter  │  メッセージの整形
└──────┬──────┘
       │
       │ (formatted message)
       │
┌──────▼──────┐
│  Notifier   │  通知の送信
└──────┬──────┘
       │
       │ (webhook)
       │
┌──────▼──────┐
│    Slack    │
└─────────────┘
```

## クイックスタート

kube-watcherは、以下の3つの方法でデプロイできます：

### 📦 方法1: Helm（推奨）

最も簡単で柔軟な方法です。

```bash
# Helmリポジトリの追加
helm repo add kube-watcher https://yourusername.github.io/kube-watcher/
helm repo update

# インストール
helm install kube-watcher kube-watcher/kube-watcher \
  --set slack.webhookUrl="https://hooks.slack.com/services/YOUR/WEBHOOK/URL" \
  --set namespace="monitoring" \
  --namespace monitoring \
  --create-namespace
```

詳細は [Helm Chartドキュメント](charts/kube-watcher/README.md) をご覧ください。

### 📝 方法2: Helmfile（宣言的管理）

helmfileで宣言的に管理する場合：

```yaml
# helmfile.yaml
repositories:
  - name: kube-watcher
    url: https://yourusername.github.io/kube-watcher/

releases:
  - name: kube-watcher
    namespace: monitoring
    chart: kube-watcher/kube-watcher
    version: ~0.1.0
    values:
      - namespace: monitoring
        slack:
          webhookUrl: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
        config:
          resources:
            - kind: Pod
            - kind: Deployment
          filters:
            - resource: Pod
              eventTypes: [DELETED]
```

```bash
helmfile apply
```

### ⚙️ 方法3: kubectl（マニフェスト直接適用）

### 前提条件

- Kubernetesクラスタ（v1.20以降）
- kubectlの設定済み環境
- Slack Webhook URL（[こちら](https://api.slack.com/messaging/webhooks)から取得可能）

### 1. リポジトリのクローン

```bash
git clone https://github.com/yourusername/kube-watcher.git
cd kube-watcher
```

### 2. Slack Webhookの設定

`deployments/secret.yaml`を編集し、Slack Webhook URLを設定します。

```yaml
stringData:
  slack-webhook-url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
```

### 3. 設定のカスタマイズ（任意）

`deployments/configmap.yaml`を編集して、以下の項目を設定できます。

- 監視対象のリソース
- イベントタイプのフィルター
- ラベルによるフィルター
- メッセージテンプレート

### 4. Kubernetesへのデプロイ

```bash
# 必要に応じてNamespaceを変更（デフォルトは "default"）
# sed -i 's/namespace: default/namespace: your-namespace/g' deployments/*.yaml

# RBACの適用
kubectl apply -f deployments/rbac.yaml

# Secretの適用
kubectl apply -f deployments/secret.yaml

# ConfigMapの適用
kubectl apply -f deployments/configmap.yaml

# Dockerイメージのビルドとプッシュ
docker build -t your-registry/kube-watcher:latest .
docker push your-registry/kube-watcher:latest

# deployment.yamlのイメージを更新
# sed -i 's|image: kube-watcher:latest|image: your-registry/kube-watcher:latest|' deployments/deployment.yaml

# アプリケーションのデプロイ
kubectl apply -f deployments/deployment.yaml
```

### 5. デプロイの確認

```bash
# Podの稼働状況を確認
kubectl get pods -l app=kube-watcher

# ログの確認
kubectl logs -l app=kube-watcher -f
```

## 設定方法

### 監視可能なリソース

以下のKubernetesリソースの監視に対応しています。

- `Pod`
- `Deployment`
- `Service`
- `ConfigMap`
- `Secret`
- `ReplicaSet`
- `StatefulSet`
- `DaemonSet`

### イベントタイプ

- `ADDED`: リソースが作成された
- `UPDATED`: リソースが更新された
- `DELETED`: リソースが削除された

### 設定例

```yaml
namespace: "production"

resources:
  - kind: Pod
  - kind: Deployment

filters:
  # 特定のラベルを持つPodの削除のみ通知
  - resource: Pod
    eventTypes: ["DELETED"]
    labels:
      environment: "production"

  # Deploymentのすべての変更を通知
  - resource: Deployment
    eventTypes: ["ADDED", "UPDATED", "DELETED"]

notifier:
  slack:
    webhookUrl: "${SLACK_WEBHOOK_URL}"
    template: |
      :warning: *[{{ .Kind }}]* `{{ .Namespace }}/{{ .Name }}`
      アクション: *{{ .EventType }}*
      時刻: {{ .Timestamp }}
      {{- if .Labels }}
      ラベル: {{ range $k, $v := .Labels }}{{ $k }}={{ $v }} {{ end }}
      {{- end }}
```

### テンプレート変数

`template`フィールドで利用可能な変数は以下の通りです。

| 変数 | 説明 | 例 |
|------|------|-----|
| `.Kind` | リソースの種類 | `Pod`, `Deployment` |
| `.Namespace` | Namespace名 | `default`, `production` |
| `.Name` | リソース名 | `my-app-123` |
| `.EventType` | イベントタイプ | `ADDED`, `UPDATED`, `DELETED` |
| `.Timestamp` | イベント発生時刻 | `2025-10-28T12:34:56Z` |
| `.Labels` | リソースのラベル | `map[app:web env:prod]` |

## 開発

### ローカル開発環境

```bash
# 依存関係のインストール
go mod download

# ローカルでの実行（kubeconfigが必要）
go run cmd/main.go -config config/config.yaml
```

### ビルド

```bash
# バイナリのビルド
go build -o kube-watcher ./cmd

# Dockerイメージのビルド
docker build -t kube-watcher:latest .
```

### Makefileの利用

プロジェクトにはMakefileが含まれており、以下のコマンドが利用できます。

```bash
make build          # バイナリのビルド
make run            # ローカルでの実行
make docker-build   # Dockerイメージのビルド
make deploy         # Kubernetesへのデプロイ
make logs           # ログの表示
make undeploy       # Kubernetesからのアンデプロイ
```

### プロジェクト構成

```
.
├── cmd/
│   └── main.go                 # アプリケーションのエントリーポイント
├── pkg/
│   ├── config/                 # 設定管理
│   │   └── config.go
│   ├── watcher/                # Kubernetesリソース監視
│   │   └── watcher.go
│   ├── filter/                 # イベントフィルタリング
│   │   └── filter.go
│   ├── formatter/              # メッセージ整形
│   │   └── formatter.go
│   └── notifier/               # 通知送信
│       └── notifier.go
├── config/
│   └── config.yaml             # 設定ファイルのサンプル
├── deployments/
│   ├── rbac.yaml               # RBACマニフェスト
│   ├── secret.yaml             # Webhook URL用Secret
│   ├── configmap.yaml          # 設定用ConfigMap
│   └── deployment.yaml         # Deploymentマニフェスト
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

## RBAC権限

本アプリケーションは**Namespace限定の権限**のみを必要とします。

```yaml
rules:
  - apiGroups: [""]
    resources: ["pods", "services", "configmaps", "secrets", "events"]
    verbs: ["list", "watch", "get"]

  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
    verbs: ["list", "watch", "get"]
```

**ClusterRoleは不要です！** そのため、マルチテナント環境でも安全にご利用いただけます。

## ロードマップ

### Phase 2（計画中）
- [x] **Helmチャート対応** ✅
- [ ] 重複イベント抑止（LRUキャッシュ）
- [ ] ConfigMapのホットリロード
- [ ] 追加の通知先対応（Teams、Discord、汎用Webhook）

### Phase 3（将来）
- [ ] イベントのバッチ処理
- [ ] リソースタイプごとのテンプレート
- [ ] 複雑なルール記述のためのフィルターDSL
- [ ] メトリクスエンドポイント（Prometheus対応）

## トラブルシューティング

### Podが起動しない場合

```bash
# RBACの確認
kubectl get role,rolebinding -n your-namespace

# ログの確認
kubectl logs -l app=kube-watcher -n your-namespace
```

### 通知が届かない場合

1. Secretに設定されたSlack Webhook URLが正しいか確認してください
2. アプリケーションのログでエラーが発生していないか確認してください
3. Webhookを手動でテストしてください

   ```bash
   curl -X POST -H 'Content-type: application/json' \
     --data '{"text":"テストメッセージ"}' \
     YOUR_WEBHOOK_URL
   ```

### イベントが検知されない場合

1. リソースが監視対象のNamespace内に存在することを確認してください
2. フィルター設定を確認してください
3. RBACのリソース権限を確認してください

## コントリビューション

プルリクエストを歓迎いたします！以下の手順でご協力ください。

1. このリポジトリをForkしてください
2. フィーチャーブランチを作成してください（`git checkout -b feature/amazing-feature`）
3. 変更をコミットしてください（`git commit -m 'Add some amazing feature'`）
4. ブランチにPushしてください（`git push origin feature/amazing-feature`）
5. プルリクエストを作成してください

### 開発ガイドライン

- コードは`go fmt`でフォーマットしてください
- 新機能には適切なコメントを追加してください
- 可能な限りテストを追加してください

## ライセンス

本プロジェクトはMITライセンスの下で公開されています。詳細は[LICENSE](LICENSE)ファイルをご覧ください。

## 参考資料

- [Kubernetes client-go](https://github.com/kubernetes/client-go) - 公式Kubernetes Goクライアント
- [Slack Incoming Webhooks](https://api.slack.com/messaging/webhooks) - Slack Webhook API
- [BotKube](https://github.com/kubeshop/botkube) - インスピレーション元
- [Robusta](https://github.com/robusta-dev/robusta) - インスピレーション元

## 設計思想

本プロジェクトは以下の原則に基づいて設計されています。

| 原則 | 内容 |
|------|------|
| セキュア | ClusterRole禁止・Namespace限定アクセスのみ |
| シンプル | 外部依存を最小化・Go標準ライブラリ中心 |
| 拡張性 | watcher/filter/formatter/notifierをinterface分離 |
| 管理容易 | Helm/ConfigMapで構成を完全外部化 |

## サポート

問題が発生した場合や機能のリクエストがある場合は、[GitHub Issues](https://github.com/yourusername/kube-watcher/issues)にてお気軽にお問い合わせください。

---

**kube-watcher**をご利用いただき、ありがとうございます。
