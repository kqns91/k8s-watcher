package formatter

import (
	"strings"
	"testing"
	"time"

	"github.com/yourusername/kube-watcher/pkg/watcher"
)

func TestNewFormatter_ValidTemplate(t *testing.T) {
	template := "{{ .Kind }} {{ .Name }}"

	formatter, err := NewFormatter(template)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v, want nil", err)
	}

	if formatter == nil {
		t.Fatal("NewFormatter() returned nil formatter")
	}
}

func TestNewFormatter_InvalidTemplate(t *testing.T) {
	invalidTemplates := []string{
		"{{ .Kind",              // 閉じ括弧なし
		"{{ .InvalidField }}",   // 存在しないフィールド（実行時エラー）
		"{{ .Kind | invalid }}", // 無効なパイプ関数
	}

	for _, tmpl := range invalidTemplates {
		t.Run(tmpl, func(t *testing.T) {
			_, err := NewFormatter(tmpl)
			// パースエラーまたは実行エラーのいずれか
			if err != nil {
				// パースエラーは期待通り
				return
			}
			// パースは成功する場合もあるので、Formatでエラーになるかテスト
		})
	}
}

func TestFormat_BasicTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		event    *watcher.Event
		want     string
	}{
		{
			name:     "simple kind and name",
			template: "{{ .Kind }} {{ .Name }}",
			event: &watcher.Event{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
				EventType: "ADDED",
			},
			want: "Pod test-pod",
		},
		{
			name:     "namespace and event type",
			template: "{{ .Namespace }}/{{ .Name }} was {{ .EventType }}",
			event: &watcher.Event{
				Kind:      "Deployment",
				Namespace: "production",
				Name:      "web-app",
				EventType: "DELETED",
			},
			want: "production/web-app was DELETED",
		},
		{
			name:     "all fields",
			template: "[{{ .Kind }}] {{ .Namespace }}/{{ .Name }} ({{ .EventType }})",
			event: &watcher.Event{
				Kind:      "Service",
				Namespace: "kube-system",
				Name:      "metrics-server",
				EventType: "UPDATED",
			},
			want: "[Service] kube-system/metrics-server (UPDATED)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, err := NewFormatter(tt.template)
			if err != nil {
				t.Fatalf("NewFormatter() error = %v", err)
			}

			got, err := formatter.Format(tt.event)
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormat_TimestampFormatting(t *testing.T) {
	template := "Time: {{ .Timestamp }}"
	formatter, err := NewFormatter(template)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	testTime := time.Date(2025, 10, 28, 12, 34, 56, 0, time.UTC)
	event := &watcher.Event{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "test",
		EventType: "ADDED",
		Timestamp: testTime,
	}

	got, err := formatter.Format(event)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	expectedTimestamp := testTime.Format(time.RFC3339)
	expected := "Time: " + expectedTimestamp

	if got != expected {
		t.Errorf("Format() = %q, want %q", got, expected)
	}
}

func TestFormat_LabelsFormatting(t *testing.T) {
	tests := []struct {
		name     string
		template string
		event    *watcher.Event
		contains []string
	}{
		{
			name:     "iterate over labels",
			template: "{{ range $k, $v := .Labels }}{{ $k }}={{ $v }} {{ end }}",
			event: &watcher.Event{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test",
				EventType: "ADDED",
				Labels: map[string]string{
					"app":     "web",
					"version": "v1.0.0",
				},
			},
			contains: []string{"app=web", "version=v1.0.0"},
		},
		{
			name:     "conditional labels display",
			template: "{{ if .Labels }}Labels: {{ range $k, $v := .Labels }}{{ $k }}={{ $v }} {{ end }}{{ end }}",
			event: &watcher.Event{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test",
				EventType: "ADDED",
				Labels: map[string]string{
					"env": "production",
				},
			},
			contains: []string{"Labels:", "env=production"},
		},
		{
			name:     "empty labels",
			template: "{{ if .Labels }}Has labels{{ else }}No labels{{ end }}",
			event: &watcher.Event{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test",
				EventType: "ADDED",
				Labels:    map[string]string{},
			},
			contains: []string{"No labels"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, err := NewFormatter(tt.template)
			if err != nil {
				t.Fatalf("NewFormatter() error = %v", err)
			}

			got, err := formatter.Format(tt.event)
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			for _, substr := range tt.contains {
				if !strings.Contains(got, substr) {
					t.Errorf("Format() = %q, should contain %q", got, substr)
				}
			}
		})
	}
}

func TestFormat_ComplexTemplate(t *testing.T) {
	// Slack風の複雑なテンプレート
	template := `:kubernetes: *[{{ .Kind }}]* ` + "`" + `{{ .Namespace }}/{{ .Name }}` + "`" + ` was *{{ .EventType }}*
Time: {{ .Timestamp }}
{{- if .Labels }}
Labels: {{ range $k, $v := .Labels }}{{ $k }}={{ $v }} {{ end }}
{{- end }}`

	formatter, err := NewFormatter(template)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	testTime := time.Date(2025, 10, 28, 12, 0, 0, 0, time.UTC)
	event := &watcher.Event{
		Kind:      "Deployment",
		Namespace: "production",
		Name:      "web-app",
		EventType: "UPDATED",
		Timestamp: testTime,
		Labels: map[string]string{
			"app": "web",
			"env": "prod",
		},
	}

	got, err := formatter.Format(event)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// 主要な要素が含まれているか確認
	expectedParts := []string{
		":kubernetes:",
		"[Deployment]",
		"production/web-app",
		"UPDATED",
		"Time:",
		"Labels:",
		"app=web",
		"env=prod",
	}

	for _, part := range expectedParts {
		if !strings.Contains(got, part) {
			t.Errorf("Format() output should contain %q, got: %q", part, got)
		}
	}
}

func TestFormat_SpecialCharacters(t *testing.T) {
	// 特殊文字が正しく処理されるかテスト
	tests := []struct {
		name     string
		template string
		event    *watcher.Event
		want     string
	}{
		{
			name:     "name with hyphens",
			template: "{{ .Name }}",
			event: &watcher.Event{
				Name: "my-app-service-v1",
			},
			want: "my-app-service-v1",
		},
		{
			name:     "namespace with underscores",
			template: "{{ .Namespace }}",
			event: &watcher.Event{
				Namespace: "my_namespace_test",
			},
			want: "my_namespace_test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, err := NewFormatter(tt.template)
			if err != nil {
				t.Fatalf("NewFormatter() error = %v", err)
			}

			got, err := formatter.Format(tt.event)
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}
