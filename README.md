# kube-watcher

Namespace限定権限で動作する、軽量なKubernetesリソース監視Bot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org/)
[![Release](https://img.shields.io/github/v/release/kqns91/kube-watcher)](https://github.com/kqns91/kube-watcher/releases)

## 概要

`kube-watcher`は、Kubernetesクラスタ内の特定のNamespace内でリソースの変更を監視し、Slackへ通知を送信する軽量な監視Botです。

BotKubeやRobustaなどの既存ツールは**ClusterRole**（クラスタ全体への権限）が必要ですが、`kube-watcher`は**Namespace限定のRole**のみで動作するため、厳格なRBACポリシーが適用されている環境でも安全にご利用いただけます。

## 主な特徴

- **🔒 セキュア**: ClusterRole不要、Namespace限定のRole権限のみで動作
- **🔍 柔軟な監視**: Pod、Deployment、Serviceなど複数のリソースタイプに対応
- **⚙️ 設定可能なフィルター**: イベントタイプ（作成/更新/削除）やラベルによるフィルタリング
- **🎯 スマートな通知**: 不要な通知を削減する高度なフィルタリング
  - **変更差分フィルタリング** (v0.1.5): 意味のある変更のみを通知（レプリカ数、イメージ、ステータス変化など）
  - **重複イベント抑止** (v0.2.0): LRUキャッシュによる同一イベントの重複通知防止
- **🔄 ホットリロード** (v0.3.0): ConfigMapの変更を自動検知してPod再起動不要で設定反映
- **📦 イベントバッチ処理** (v0.4.0): 複数のイベントをまとめて通知し、通知頻度を最適化
  - 3つのモード（detailed/summary/smart）で柔軟な表示制御
  - スマートモードで重要イベント（削除など）は常に詳細表示
- **🔍 CEL式フィルターDSL** (v0.5.0): 複雑なフィルタリング条件を柔軟に記述
  - Google CEL（Common Expression Language）による高度なフィルタリング
  - イベント理由、ラベル、レプリカ数など多様な条件に対応
  - Deploymentの重複通知問題を解決
- **🎨 リッチな通知**: Slack Attachmentsによる色分けと詳細情報の表示
  - イベントタイプに応じた色分け（追加=緑、更新=黄、削除=赤）
  - コンテナイメージとタグ情報
  - レプリカ数の詳細（Desired/Ready/Current）
  - Podステータス、理由、メッセージなどの詳細情報
- **✨ カスタマイズ可能**: Goテンプレートを使用したSlackメッセージのカスタマイズ
- **🪶 軽量**: 最小限のリソースフットプリント、シンプルな依存関係

## アーキテクチャ

```
┌──────────────┐
│  ConfigMap   │  設定ファイル（ConfigMap）
└──────┬───────┘
       │
       │ (fsnotify watch)
       │
┌──────▼────────┐
│ ConfigWatcher │  設定変更の自動検知・ホットリロード
└───────────────┘
       ┃
       ┃ (reload components)
       ┃
       ▼
┌─────────────┐
│ Kubernetes  │
│   API       │
└──────┬──────┘
       │
       │ (informer/watch)
       │
┌──────▼──────┐
│   Watcher   │  リソース変更の検知 + 変更差分フィルタリング
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
│ Deduplicator│  重複イベント抑止（LRUキャッシュ）
└──────┬──────┘
       │
       │ (unique events)
       │
       ├──────────────────┐
       │                  │
       │ (batching=off)   │ (batching=on)
       │                  │
       │           ┌──────▼──────┐
       │           │   Batcher   │  イベント集約・バッチ処理
       │           └──────┬──────┘
       │                  │
       │                  │ (batch window)
       │                  │
       └──────────────────┤
                          │
                   ┌──────▼──────┐
                   │  Formatter  │  メッセージの整形（Slack Attachments）
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
helm repo add kube-watcher https://kqns91.github.io/kube-watcher/
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
    url: https://kqns91.github.io/kube-watcher/

releases:
  - name: kube-watcher
    namespace: monitoring
    chart: kube-watcher/kube-watcher
    version: ~0.5.0
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
            - resource: Deployment
              # CEL式によるフィルタリング（v0.5.0以降）
              # ReplicaSetUpdatedとNewReplicaSetAvailableを除外
              expression: 'event.eventType == "UPDATED" && event.reason != "ReplicaSetUpdated" && event.reason != "NewReplicaSetAvailable"'
          # 重複排除設定（オプション）
          deduplication:
            enabled: true
            ttlSeconds: 300
            maxCacheSize: 1000
          # バッチ処理設定（オプション）
          batching:
            enabled: false
            windowSeconds: 300
            mode: smart
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
git clone https://github.com/kqns91/kube-watcher.git
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

## Slack通知の表示例

v0.1.4 から、Slack Attachments API を使用したリッチな通知フォーマットに対応しています。

### 通知の色分け

イベントタイプに応じて、メッセージの左側に色が表示されます：

- 🟢 **ADDED（作成）**: 緑色 - 新しいリソースが作成されたとき
- 🟡 **UPDATED（更新）**: 黄色 - 既存のリソースが更新されたとき
- 🔴 **DELETED（削除）**: 赤色 - リソースが削除されたとき

### 表示される詳細情報

リソースタイプに応じて、以下の詳細情報が自動的に表示されます：

#### Deployment の場合
- コンテナ情報（名前とイメージタグ）
- レプリカ情報（Desired / Ready / Current）
- Deployment のステータスと理由

#### Pod の場合
- Podのステータス（Running、Pending、Failed など）
- コンテナイメージ情報
- 理由とメッセージ（エラー時など）

#### Service の場合
- サービスタイプ（ClusterIP、LoadBalancer など）

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
      environment: "production"  # 単一ラベル

  # 複数のラベル条件（すべて一致する必要がある = AND条件）
  - resource: Pod
    eventTypes: ["DELETED"]
    labels:
      environment: "production"
      app: "web"
      tier: "frontend"

  # Deploymentのすべての変更を通知
  - resource: Deployment
    eventTypes: ["ADDED", "UPDATED", "DELETED"]

  # CEL式による高度なフィルタリング（v0.5.0以降）
  # expressionが設定されている場合、eventTypesとlabelsよりも優先されます
  - resource: Deployment
    # ReplicaSetUpdatedとNewReplicaSetAvailableを除外
    expression: 'event.eventType == "UPDATED" && event.reason != "ReplicaSetUpdated" && event.reason != "NewReplicaSetAvailable"'

  - resource: Pod
    # 本番環境のPodで削除またはapp=webのラベルを持つもの
    expression: 'event.eventType == "DELETED" || event.labels.app == "web"'

  - resource: Deployment
    # レプリカ数が3を超える場合のみ通知
    expression: 'has(event.replicas) && event.replicas.desired > 3'

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

# イベント重複排除設定（オプション、v0.2.0以降）
deduplication:
  enabled: true        # 重複排除を有効化
  ttlSeconds: 300      # 5分間同じイベントは通知しない
  maxCacheSize: 1000   # 最大1000エントリをキャッシュ

# イベントバッチ処理設定（オプション、v0.4.0以降）
batching:
  enabled: false       # バッチ処理を有効化
  windowSeconds: 300   # 5分間のイベントをまとめて通知
  mode: smart          # detailed/summary/smart
  smart:
    maxEventsPerGroup: 5    # グループごとに最大5件まで詳細表示
    maxTotalEvents: 20      # 合計20件を超えるとサマリーモード
    alwaysShowDetails:      # 常に詳細表示するイベントタイプ
      - DELETED
```

### テンプレート変数

`template`フィールドで利用可能な変数は以下の通りです。

#### 基本情報

| 変数 | 説明 | 例 |
|------|------|-----|
| `.Kind` | リソースの種類 | `Pod`, `Deployment` |
| `.Namespace` | Namespace名 | `default`, `production` |
| `.Name` | リソース名 | `my-app-123` |
| `.EventType` | イベントタイプ | `ADDED`, `UPDATED`, `DELETED` |
| `.Timestamp` | イベント発生時刻 | `2025-10-28T12:34:56Z` |
| `.Labels` | リソースのラベル | `map[app:web env:prod]` |

#### 詳細情報（v0.1.4以降）

| 変数 | 説明 | 対象リソース |
|------|------|--------------|
| `.Status` | リソースのステータス | Pod |
| `.Reason` | イベントの理由 | Pod, Deployment |
| `.Message` | イベントメッセージ | Pod, Deployment |
| `.Containers` | コンテナ情報（名前、イメージ） | Pod, Deployment |
| `.Replicas` | レプリカ情報（Desired/Ready/Current） | Deployment, ReplicaSet, StatefulSet |
| `.ServiceType` | サービスタイプ | Service |

**注意**: v0.1.4 以降、デフォルトでは Slack Attachments 形式で通知が送信されるため、これらの詳細情報は自動的に整形されて表示されます。カスタムテンプレートを使用する場合のみ、これらの変数を明示的に参照する必要があります。

### CEL式フィルター（v0.5.0以降）

フィルターに`expression`フィールドを指定することで、CEL（Common Expression Language）による高度なフィルタリングが可能です。

#### 利用可能なCELフィールド

| フィールド | 説明 | 例 |
|-----------|------|-----|
| `event.kind` | リソース種類 | `"Pod"`, `"Deployment"` |
| `event.namespace` | Namespace名 | `"default"`, `"production"` |
| `event.name` | リソース名 | `"my-app-123"` |
| `event.eventType` | イベントタイプ | `"ADDED"`, `"UPDATED"`, `"DELETED"` |
| `event.reason` | イベント理由 | `"ReplicaSetUpdated"`, `"ScalingReplicaSet"` |
| `event.message` | イベントメッセージ | 文字列 |
| `event.status` | リソースステータス | `"Running"`, `"Pending"` |
| `event.labels` | ラベル（map） | `event.labels.app == "web"` |
| `event.replicas` | レプリカ情報（構造体） | `event.replicas.desired > 3` |
| `event.containers` | コンテナ情報（配列） | - |
| `event.serviceType` | サービスタイプ | `"ClusterIP"`, `"LoadBalancer"` |

#### CEL式の例

```yaml
# Deploymentの重複通知を除外（最も一般的な使い方）
- resource: Deployment
  expression: 'event.eventType == "UPDATED" && event.reason != "ReplicaSetUpdated" && event.reason != "NewReplicaSetAvailable"'

# 複数のイベントタイプ（IN演算子）
- resource: Pod
  expression: 'event.eventType in ["ADDED", "DELETED"]'

# ラベルと条件の組み合わせ（AND条件）
- resource: Pod
  expression: 'event.namespace == "prod" && event.labels.app == "web" && event.eventType == "DELETED"'

# 複数ラベルのAND条件
- resource: Pod
  expression: >
    event.labels.environment == "production" &&
    event.labels.app == "web" &&
    event.labels.tier == "frontend"

# ラベルのOR条件
- resource: Pod
  expression: 'event.labels.app == "web" || event.labels.app == "api" || event.labels.app == "worker"'

# レプリカ数の条件
- resource: Deployment
  expression: 'has(event.replicas) && event.replicas.desired > 3'

# 複雑なOR条件
- resource: Pod
  expression: 'event.eventType == "DELETED" || (event.eventType == "UPDATED" && event.status != "Running")'

# YAMLの複数行記法を使った読みやすい書き方（推奨）
# > (folded style) を使うと複数行が1行にスペース区切りで結合される
- resource: Deployment
  expression: >
    event.eventType == "UPDATED" &&
    event.reason != "ReplicaSetUpdated" &&
    event.reason != "NewReplicaSetAvailable"
```

**注意**:
- `expression` が設定されている場合、`eventTypes` と `labels` フィールドは無視され、CEL式の評価結果のみが使用されます。
- YAMLの `>` (folded style) を使うと、複数行をスペースで区切った1行として扱われるため、長いCEL式を読みやすく記述できます。

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
make test           # テストの実行
make lint           # コードのLint
make lint-fix       # Lintエラーの自動修正
make fmt            # コードフォーマット
make docker-build   # Dockerイメージのビルド
make deploy         # Kubernetesへのデプロイ
make logs           # ログの表示
make undeploy       # Kubernetesからのアンデプロイ
```

### コード品質

プロジェクトでは[golangci-lint](https://golangci-lint.run/)を使用してコード品質を維持しています。

```bash
# golangci-lintのインストール
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Lintの実行
make lint

# Lintエラーの自動修正
make lint-fix
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
│   │   ├── filter.go
│   │   ├── cel.go              # CEL式評価エンジン
│   │   └── cel_test.go
│   ├── dedup/                  # 重複イベント抑止
│   │   ├── dedup.go
│   │   └── dedup_test.go
│   ├── batcher/                # イベントバッチ処理
│   │   ├── batcher.go
│   │   └── batcher_test.go
│   ├── reload/                 # 設定ホットリロード
│   │   ├── reload.go
│   │   └── reload_test.go
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

### Step 1（完了）✅
- [x] **基本機能の実装** - Kubernetesリソースの監視とSlack通知
- [x] **Helmチャート対応** - Helmによる簡単なデプロイ
- [x] **CI/CD構築** - GitHub Actionsによる自動ビルドとリリース

### Step 2（完了）✅
- [x] **リッチな通知フォーマット** - Slack Attachments APIによる色分け表示
- [x] **詳細情報の表示** - コンテナイメージ、レプリカ数、ステータスなど
- [x] **イベントタイプ別の色分け** - ADDED/UPDATED/DELETED の視覚的区別
- [x] **変更差分フィルタリング** (v0.1.5) - 意味のある変更のみを通知

### Step 3（完了）✅
- [x] **重複イベント抑止（LRUキャッシュ）** (v0.2.0) - 同一イベントの重複通知を防止
- [x] **ConfigMapのホットリロード** (v0.3.0) - Pod再起動なしで設定を自動反映
- [x] **イベント集約とバッチ処理** (v0.4.0) - 複数イベントをまとめて通知、3つのモード対応
- [x] **複雑なルール記述のためのフィルターDSL** (v0.5.0) - CEL式による高度なフィルタリング

### Step 4（将来）
- [ ] 追加の通知先対応（Teams、Discord、汎用Webhook）
- [ ] リソースタイプごとのカスタムテンプレート
- [ ] メトリクスエンドポイント（Prometheus対応）
- [ ] Web UIダッシュボード

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

### 通知が頻繁すぎる場合

kube-watcher には複数の通知削減機能があります：

1. **変更差分フィルタリング** (v0.1.5以降、自動有効)
   - 意味のある変更のみを通知します
   - ResourceVersion が同じ場合は通知しません
   - レプリカ数、イメージ、ステータスなどの重要な変更のみ検出

2. **重複イベント抑止** (v0.2.0以降、要設定)
   - 同じイベントが短時間に複数回発生しても1回だけ通知
   - `deduplication.enabled: true` で有効化
   - `ttlSeconds` で重複判定期間を調整（デフォルト: 300秒）

3. **イベントバッチ処理** (v0.4.0以降、要設定)
   - 複数のイベントをまとめて通知し、通知回数を削減
   - `batching.enabled: true` で有効化
   - `windowSeconds` でバッチウィンドウを調整（デフォルト: 300秒 = 5分）
   - スマートモードで自動的に詳細/サマリーを切り替え
   ```yaml
   batching:
     enabled: true
     windowSeconds: 300  # 5分ごとにまとめて通知
     mode: smart
   ```

4. **CEL式フィルター** (v0.5.0以降、推奨)
   - イベント理由やその他の条件で柔軟にフィルタリング
   - Deploymentの重複通知問題を解決
   ```yaml
   filters:
     - resource: Deployment
       # ReplicaSetUpdatedとNewReplicaSetAvailableを除外
       expression: 'event.eventType == "UPDATED" && event.reason != "ReplicaSetUpdated" && event.reason != "NewReplicaSetAvailable"'
   ```

5. **イベントタイプフィルター**
   - UPDATED イベントを除外することで通知を大幅削減
   ```yaml
   filters:
     - resource: Pod
       eventTypes: ["ADDED", "DELETED"]  # UPDATEDを除外
   ```

### 設定変更が反映されない場合

v0.3.0 以降、ConfigMap の変更は自動的に検知され、Pod の再起動なしで反映されます。

1. **ホットリロードの動作確認**
   ```bash
   # ConfigMapを更新
   kubectl edit configmap kube-watcher-config -n your-namespace

   # ログで設定の再読み込みを確認
   kubectl logs -l app=kube-watcher -n your-namespace -f
   # 以下のようなログが出力されるはずです：
   # "Configuration file changed, reloading..."
   # "Configuration reloaded successfully"
   ```

2. **ホットリロードが動作しない場合**
   - ConfigMap がマウントされているか確認してください
   - ログにエラーが出ていないか確認してください
   - 最終手段として Pod を再起動してください：
     ```bash
     kubectl rollout restart deployment kube-watcher -n your-namespace
     ```

## コントリビューション

プルリクエストを歓迎いたします！以下の手順でご協力ください。

1. このリポジトリをForkしてください
2. フィーチャーブランチを作成してください（`git checkout -b feature/amazing-feature`）
3. 変更をコミットしてください（`git commit -m 'Add some amazing feature'`）
4. ブランチにPushしてください（`git push origin feature/amazing-feature`）
5. プルリクエストを作成してください

### 開発ガイドライン

- コードは`go fmt`でフォーマットしてください
- `make lint`でLintチェックを行い、エラーがないことを確認してください
- 新機能には適切なコメントを追加してください
- 可能な限りテストを追加してください
- コミット前に必ず`make lint`と`make test`を実行してください

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

問題が発生した場合や機能のリクエストがある場合は、[GitHub Issues](https://github.com/kqns91/kube-watcher/issues)にてお気軽にお問い合わせください。

---

**kube-watcher**をご利用いただき、ありがとうございます。
