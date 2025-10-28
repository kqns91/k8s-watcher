package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidConfig(t *testing.T) {
	// 有効な設定ファイルを作成
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	validConfig := `
namespace: production

resources:
  - kind: Pod
  - kind: Deployment

filters:
  - resource: Pod
    eventTypes: [ADDED, DELETED]

notifier:
  slack:
    webhookUrl: "https://hooks.slack.com/services/TEST/WEBHOOK/URL"
    template: "Test template"
`

	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	// 基本的な検証
	if cfg.Namespace != "production" {
		t.Errorf("Namespace = %v, want production", cfg.Namespace)
	}

	if len(cfg.Resources) != 2 {
		t.Errorf("len(Resources) = %v, want 2", len(cfg.Resources))
	}

	if cfg.Notifier.Slack.WebhookURL != "https://hooks.slack.com/services/TEST/WEBHOOK/URL" {
		t.Errorf("WebhookURL = %v, want https://hooks.slack.com/services/TEST/WEBHOOK/URL", cfg.Notifier.Slack.WebhookURL)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("LoadConfig() error = nil, want error for nonexistent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `
namespace: test
resources:
  - kind: Pod
  invalid yaml here!!!
`

	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("LoadConfig() error = nil, want error for invalid YAML")
	}
}

func TestValidate_MissingNamespace(t *testing.T) {
	cfg := &Config{
		Resources: []ResourceConfig{
			{Kind: "Pod"},
		},
		Notifier: NotifierConfig{
			Slack: SlackConfig{
				WebhookURL: "https://example.com",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() error = nil, want error for missing namespace")
	}
}

func TestValidate_MissingResources(t *testing.T) {
	cfg := &Config{
		Namespace: "default",
		Resources: []ResourceConfig{},
		Notifier: NotifierConfig{
			Slack: SlackConfig{
				WebhookURL: "https://example.com",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() error = nil, want error for missing resources")
	}
}

func TestValidate_MissingWebhookURL(t *testing.T) {
	cfg := &Config{
		Namespace: "default",
		Resources: []ResourceConfig{
			{Kind: "Pod"},
		},
		Notifier: NotifierConfig{
			Slack: SlackConfig{
				WebhookURL: "",
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() error = nil, want error for missing webhook URL")
	}
}

func TestValidate_DefaultTemplate(t *testing.T) {
	cfg := &Config{
		Namespace: "default",
		Resources: []ResourceConfig{
			{Kind: "Pod"},
		},
		Notifier: NotifierConfig{
			Slack: SlackConfig{
				WebhookURL: "https://example.com",
				Template:   "",
			},
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}

	// デフォルトテンプレートが設定されているか確認
	if cfg.Notifier.Slack.Template == "" {
		t.Error("Template is empty, expected default template to be set")
	}
}

func TestGetFilterForResource(t *testing.T) {
	cfg := &Config{
		Filters: []FilterConfig{
			{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
			},
			{
				Resource:   "Deployment",
				EventTypes: []string{"ADDED", "UPDATED"},
			},
		},
	}

	tests := []struct {
		name     string
		kind     string
		wantNil  bool
		wantKind string
	}{
		{
			name:     "existing filter for Pod",
			kind:     "Pod",
			wantNil:  false,
			wantKind: "Pod",
		},
		{
			name:     "existing filter for Deployment",
			kind:     "Deployment",
			wantNil:  false,
			wantKind: "Deployment",
		},
		{
			name:    "non-existing filter for Service",
			kind:    "Service",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := cfg.GetFilterForResource(tt.kind)

			if tt.wantNil {
				if filter != nil {
					t.Errorf("GetFilterForResource() = %v, want nil", filter)
				}
			} else {
				if filter == nil {
					t.Fatal("GetFilterForResource() = nil, want non-nil")
				}
				if filter.Resource != tt.wantKind {
					t.Errorf("filter.Resource = %v, want %v", filter.Resource, tt.wantKind)
				}
			}
		})
	}
}

func TestLoadConfig_ComplexConfiguration(t *testing.T) {
	// 複雑な設定ファイルのテスト
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "complex.yaml")

	complexConfig := `
namespace: production

resources:
  - kind: Pod
  - kind: Deployment
  - kind: Service

filters:
  - resource: Pod
    eventTypes: [DELETED]
    labels:
      environment: production
      tier: frontend
  - resource: Deployment
    eventTypes: [ADDED, UPDATED, DELETED]
  - resource: Service
    eventTypes: [ADDED, DELETED]

notifier:
  slack:
    webhookUrl: "https://hooks.slack.com/services/XXX/YYY/ZZZ"
    template: |
      :kubernetes: *[{{ .Kind }}]* {{ .Namespace }}/{{ .Name }}
      Action: {{ .EventType }}
`

	if err := os.WriteFile(configPath, []byte(complexConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	// リソース数の確認
	if len(cfg.Resources) != 3 {
		t.Errorf("len(Resources) = %v, want 3", len(cfg.Resources))
	}

	// フィルター数の確認
	if len(cfg.Filters) != 3 {
		t.Errorf("len(Filters) = %v, want 3", len(cfg.Filters))
	}

	// Podフィルターのラベル確認
	podFilter := cfg.GetFilterForResource("Pod")
	if podFilter == nil {
		t.Fatal("Pod filter is nil")
	}

	if len(podFilter.Labels) != 2 {
		t.Errorf("len(PodFilter.Labels) = %v, want 2", len(podFilter.Labels))
	}

	if podFilter.Labels["environment"] != "production" {
		t.Errorf("PodFilter.Labels[environment] = %v, want production", podFilter.Labels["environment"])
	}
}
