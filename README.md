# kube-watcher

A lightweight Kubernetes resource monitoring bot with namespace-scoped permissions that sends notifications to Slack.

## Overview

`kube-watcher` monitors Kubernetes resources within a single namespace and sends notifications to Slack when resources are created, updated, or deleted. Unlike tools like BotKube or Robusta that require cluster-wide permissions (ClusterRole), `kube-watcher` operates with minimal namespace-scoped permissions (Role), making it suitable for environments with strict RBAC policies.

## Features

- **Namespace-scoped**: Uses only Role-based permissions (no ClusterRole required)
- **Flexible resource monitoring**: Watch Pods, Deployments, Services, ConfigMaps, Secrets, and more
- **Configurable filters**: Filter events by type (ADDED/UPDATED/DELETED) and resource labels
- **Customizable notifications**: Use Go templates to format Slack messages
- **Lightweight**: Minimal resource footprint

## Architecture

```
┌─────────────┐
│ Kubernetes  │
│   API       │
└──────┬──────┘
       │
       │ (informer/watch)
       │
┌──────▼──────┐
│   Watcher   │
└──────┬──────┘
       │
       │ (events)
       │
┌──────▼──────┐
│   Filter    │
└──────┬──────┘
       │
       │ (filtered events)
       │
┌──────▼──────┐
│  Formatter  │
└──────┬──────┘
       │
       │ (formatted message)
       │
┌──────▼──────┐
│  Notifier   │
└──────┬──────┘
       │
       │ (webhook)
       │
┌──────▼──────┐
│    Slack    │
└─────────────┘
```

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.20+)
- kubectl configured
- Slack webhook URL ([Create one here](https://api.slack.com/messaging/webhooks))

### 1. Clone the repository

```bash
git clone https://github.com/yourusername/kube-watcher.git
cd kube-watcher
```

### 2. Configure Slack webhook

Edit `deployments/secret.yaml` and replace with your Slack webhook URL:

```yaml
stringData:
  slack-webhook-url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
```

### 3. Customize configuration (optional)

Edit `deployments/configmap.yaml` to configure:
- Which resources to watch
- Event type filters
- Label filters
- Message template

### 4. Deploy to Kubernetes

```bash
# Update namespace in all manifests if needed (default is "default")
# sed -i 's/namespace: default/namespace: your-namespace/g' deployments/*.yaml

# Apply RBAC
kubectl apply -f deployments/rbac.yaml

# Apply Secret
kubectl apply -f deployments/secret.yaml

# Apply ConfigMap
kubectl apply -f deployments/configmap.yaml

# Build and push Docker image
docker build -t your-registry/kube-watcher:latest .
docker push your-registry/kube-watcher:latest

# Update deployment.yaml with your image
# sed -i 's|image: kube-watcher:latest|image: your-registry/kube-watcher:latest|' deployments/deployment.yaml

# Deploy application
kubectl apply -f deployments/deployment.yaml
```

### 5. Verify deployment

```bash
# Check if pod is running
kubectl get pods -l app=kube-watcher

# Check logs
kubectl logs -l app=kube-watcher -f
```

## Configuration

### Watched Resources

Supported resource kinds:
- `Pod`
- `Deployment`
- `Service`
- `ConfigMap`
- `Secret`
- `ReplicaSet`
- `StatefulSet`
- `DaemonSet`

### Event Types

- `ADDED`: Resource was created
- `UPDATED`: Resource was modified
- `DELETED`: Resource was removed

### Example Configuration

```yaml
namespace: "production"

resources:
  - kind: Pod
  - kind: Deployment

filters:
  # Only notify for Pod deletions with specific label
  - resource: Pod
    eventTypes: ["DELETED"]
    labels:
      environment: "production"

  # Notify for all Deployment changes
  - resource: Deployment
    eventTypes: ["ADDED", "UPDATED", "DELETED"]

notifier:
  slack:
    webhookUrl: "${SLACK_WEBHOOK_URL}"
    template: |
      :warning: *[{{ .Kind }}]* `{{ .Namespace }}/{{ .Name }}`
      Action: *{{ .EventType }}*
      Time: {{ .Timestamp }}
      {{- if .Labels }}
      Labels: {{ range $k, $v := .Labels }}{{ $k }}={{ $v }} {{ end }}
      {{- end }}
```

### Template Variables

Available in the `template` field:

| Variable | Description | Example |
|----------|-------------|---------|
| `.Kind` | Resource kind | `Pod`, `Deployment` |
| `.Namespace` | Resource namespace | `default`, `production` |
| `.Name` | Resource name | `my-app-123` |
| `.EventType` | Event type | `ADDED`, `UPDATED`, `DELETED` |
| `.Timestamp` | Event timestamp | `2025-10-28T12:34:56Z` |
| `.Labels` | Resource labels | `map[app:web env:prod]` |

## Development

### Local Development

```bash
# Install dependencies
go mod download

# Run locally (requires kubeconfig)
go run cmd/main.go -config config/config.yaml
```

### Build

```bash
# Build binary
go build -o kube-watcher ./cmd

# Build Docker image
docker build -t kube-watcher:latest .
```

### Project Structure

```
.
├── cmd/
│   └── main.go                 # Application entry point
├── pkg/
│   ├── config/                 # Configuration management
│   │   └── config.go
│   ├── watcher/                # Kubernetes resource watching
│   │   └── watcher.go
│   ├── filter/                 # Event filtering logic
│   │   └── filter.go
│   ├── formatter/              # Message formatting
│   │   └── formatter.go
│   └── notifier/               # Notification delivery
│       └── notifier.go
├── config/
│   └── config.yaml             # Example configuration
├── deployments/
│   ├── rbac.yaml               # RBAC manifests
│   ├── secret.yaml             # Secret for webhook URL
│   ├── configmap.yaml          # ConfigMap for config
│   └── deployment.yaml         # Deployment manifest
├── Dockerfile
├── go.mod
└── README.md
```

## RBAC Permissions

The application requires only namespace-scoped permissions:

```yaml
rules:
  - apiGroups: [""]
    resources: ["pods", "services", "configmaps", "secrets", "events"]
    verbs: ["list", "watch", "get"]

  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
    verbs: ["list", "watch", "get"]
```

**No ClusterRole required!** This makes it safe to use in multi-tenant environments.

## Roadmap

### Phase 2 (Planned)
- [ ] Duplicate event suppression (LRU cache)
- [ ] ConfigMap hot-reload
- [ ] Helm chart
- [ ] Additional notifiers (Teams, Discord, generic webhooks)

### Phase 3 (Future)
- [ ] Event batching
- [ ] Per-resource message templates
- [ ] Filter DSL for complex rules
- [ ] Metrics endpoint (Prometheus)

## Troubleshooting

### Pod not starting

```bash
# Check RBAC
kubectl get role,rolebinding -n your-namespace

# Check logs
kubectl logs -l app=kube-watcher -n your-namespace
```

### Not receiving notifications

1. Verify Slack webhook URL in secret
2. Check application logs for errors
3. Test webhook manually:
   ```bash
   curl -X POST -H 'Content-type: application/json' \
     --data '{"text":"Test message"}' \
     YOUR_WEBHOOK_URL
   ```

### Events not being detected

1. Verify resources are in the watched namespace
2. Check filter configuration
3. Review resource permissions in RBAC

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## License

MIT License - see LICENSE file for details

## References

- [Kubernetes client-go](https://github.com/kubernetes/client-go)
- [Slack Incoming Webhooks](https://api.slack.com/messaging/webhooks)
- [BotKube](https://github.com/kubeshop/botkube) (inspiration)
- [Robusta](https://github.com/robusta-dev/robusta) (inspiration)
