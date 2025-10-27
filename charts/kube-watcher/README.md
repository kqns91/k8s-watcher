# kube-watcher Helm Chart

Namespace限定権限で動作する軽量なKubernetesリソース監視Bot

## インストール

### Helmリポジトリの追加

```bash
helm repo add kube-watcher https://yourusername.github.io/kube-watcher/
helm repo update
```

### インストール

```bash
# 基本的なインストール
helm install kube-watcher kube-watcher/kube-watcher \
  --set slack.webhookUrl="https://hooks.slack.com/services/YOUR/WEBHOOK/URL" \
  --namespace monitoring \
  --create-namespace

# values.yamlを使用したインストール
helm install kube-watcher kube-watcher/kube-watcher \
  -f values.yaml \
  --namespace monitoring \
  --create-namespace
```

## 設定値

### 基本設定

| パラメータ | 説明 | デフォルト |
|-----------|------|-----------|
| `replicaCount` | レプリカ数 | `1` |
| `namespace` | 監視対象のNamespace | `default` |
| `image.repository` | Dockerイメージリポジトリ | `kube-watcher` |
| `image.tag` | Dockerイメージタグ | `latest` |
| `image.pullPolicy` | イメージプルポリシー | `IfNotPresent` |

### Slack設定

| パラメータ | 説明 | デフォルト |
|-----------|------|-----------|
| `slack.webhookUrl` | Slack Webhook URL（直接指定） | `""` |
| `slack.existingSecret` | 既存のSecret名 | `""` |
| `slack.existingSecretKey` | Secretのキー名 | `webhook-url` |

### 監視設定

| パラメータ | 説明 | デフォルト |
|-----------|------|-----------|
| `config.resources` | 監視対象リソースのリスト | `[Pod, Deployment, Service]` |
| `config.filters` | フィルター設定のリスト | 下記参照 |
| `config.messageTemplate` | Slackメッセージテンプレート | 下記参照 |

### リソース制限

| パラメータ | 説明 | デフォルト |
|-----------|------|-----------|
| `resources.limits.cpu` | CPU制限 | `200m` |
| `resources.limits.memory` | メモリ制限 | `256Mi` |
| `resources.requests.cpu` | CPU要求 | `100m` |
| `resources.requests.memory` | メモリ要求 | `128Mi` |

### RBAC設定

| パラメータ | 説明 | デフォルト |
|-----------|------|-----------|
| `rbac.create` | RBACリソースを作成するか | `true` |
| `rbac.extraRules` | 追加のRBACルール | `[]` |

## 使用例

### 例1: 基本的な設定

```yaml
# values.yaml
namespace: production

slack:
  webhookUrl: "https://hooks.slack.com/services/XXX/YYY/ZZZ"

config:
  resources:
    - kind: Pod
    - kind: Deployment

  filters:
    - resource: Pod
      eventTypes: [ADDED, DELETED]
```

```bash
helm install kube-watcher kube-watcher/kube-watcher -f values.yaml
```

### 例2: 既存のSecretを使用

```bash
# Secretを作成
kubectl create secret generic slack-webhook \
  --from-literal=webhook-url="https://hooks.slack.com/services/XXX/YYY/ZZZ" \
  -n monitoring

# Helmでインストール
helm install kube-watcher kube-watcher/kube-watcher \
  --set slack.existingSecret="slack-webhook" \
  --set namespace="monitoring" \
  --namespace monitoring
```

### 例3: ラベルフィルタリング

```yaml
# values.yaml
config:
  filters:
    - resource: Pod
      eventTypes: [DELETED]
      labels:
        environment: production
        tier: frontend
```

### 例4: カスタムメッセージテンプレート

```yaml
# values.yaml
config:
  messageTemplate: |
    🚨 *アラート*
    種別: {{ .Kind }}
    名前: {{ .Namespace }}/{{ .Name }}
    アクション: {{ .EventType }}
    時刻: {{ .Timestamp }}
    {{- if .Labels }}
    ラベル: {{ range $k, $v := .Labels }}
      • {{ $k }}: {{ $v }}
    {{- end }}
    {{- end }}
```

## Helmfileでの使用

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
          existingSecret: slack-webhook
        config:
          resources:
            - kind: Pod
            - kind: Deployment
            - kind: Service
          filters:
            - resource: Pod
              eventTypes: [DELETED]
```

```bash
helmfile apply
```

## アップグレード

```bash
helm upgrade kube-watcher kube-watcher/kube-watcher \
  -f values.yaml \
  --namespace monitoring
```

## アンインストール

```bash
helm uninstall kube-watcher --namespace monitoring
```

## テンプレートの確認

```bash
# レンダリング結果の確認
helm template kube-watcher kube-watcher/kube-watcher -f values.yaml

# デバッグ
helm install kube-watcher kube-watcher/kube-watcher --debug --dry-run
```

## トラブルシューティング

### Podが起動しない

```bash
# Pod状態の確認
kubectl get pods -l app.kubernetes.io/name=kube-watcher -n monitoring

# ログの確認
kubectl logs -l app.kubernetes.io/name=kube-watcher -n monitoring

# イベントの確認
kubectl get events -n monitoring --sort-by='.lastTimestamp'
```

### RBAC権限エラー

```bash
# Roleの確認
kubectl get role -n monitoring

# RoleBindingの確認
kubectl get rolebinding -n monitoring

# ServiceAccountの確認
kubectl get serviceaccount -n monitoring
```

## 開発

### ローカルでのChart確認

```bash
# Lintチェック
helm lint charts/kube-watcher

# テンプレート確認
helm template test charts/kube-watcher -f charts/kube-watcher/values.yaml

# インストール（ローカルChart）
helm install kube-watcher ./charts/kube-watcher -f values.yaml
```

## ライセンス

MIT License

## リンク

- [GitHub リポジトリ](https://github.com/yourusername/kube-watcher)
- [Issue トラッカー](https://github.com/yourusername/kube-watcher/issues)
