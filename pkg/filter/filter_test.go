package filter

import (
	"testing"

	"github.com/yourusername/kube-watcher/pkg/config"
	"github.com/yourusername/kube-watcher/pkg/watcher"
)

func TestFilter_ShouldProcess_EventTypeFiltering(t *testing.T) {
	tests := []struct {
		name          string
		filterConfig  *config.FilterConfig
		event         *watcher.Event
		shouldProcess bool
	}{
		{
			name: "DELETED event matches DELETED filter",
			filterConfig: &config.FilterConfig{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
			},
			event: &watcher.Event{
				Kind:      "Pod",
				EventType: "DELETED",
			},
			shouldProcess: true,
		},
		{
			name: "ADDED event does not match DELETED filter",
			filterConfig: &config.FilterConfig{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
			},
			event: &watcher.Event{
				Kind:      "Pod",
				EventType: "ADDED",
			},
			shouldProcess: false,
		},
		{
			name: "UPDATED event matches multiple event types filter",
			filterConfig: &config.FilterConfig{
				Resource:   "Deployment",
				EventTypes: []string{"ADDED", "UPDATED", "DELETED"},
			},
			event: &watcher.Event{
				Kind:      "Deployment",
				EventType: "UPDATED",
			},
			shouldProcess: true,
		},
		{
			name: "empty event types filter allows all events",
			filterConfig: &config.FilterConfig{
				Resource:   "Service",
				EventTypes: []string{},
			},
			event: &watcher.Event{
				Kind:      "Service",
				EventType: "ADDED",
			},
			shouldProcess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Filters: []config.FilterConfig{*tt.filterConfig},
			}
			f := NewFilter(cfg)

			got := f.ShouldProcess(tt.event)
			if got != tt.shouldProcess {
				t.Errorf("ShouldProcess() = %v, want %v", got, tt.shouldProcess)
			}
		})
	}
}

func TestFilter_ShouldProcess_LabelFiltering(t *testing.T) {
	tests := []struct {
		name          string
		filterConfig  *config.FilterConfig
		event         *watcher.Event
		shouldProcess bool
	}{
		{
			name: "matching single label",
			filterConfig: &config.FilterConfig{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
				Labels: map[string]string{
					"app": "web",
				},
			},
			event: &watcher.Event{
				Kind:      "Pod",
				EventType: "DELETED",
				Labels: map[string]string{
					"app": "web",
				},
			},
			shouldProcess: true,
		},
		{
			name: "non-matching label value",
			filterConfig: &config.FilterConfig{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
				Labels: map[string]string{
					"app": "web",
				},
			},
			event: &watcher.Event{
				Kind:      "Pod",
				EventType: "DELETED",
				Labels: map[string]string{
					"app": "api",
				},
			},
			shouldProcess: false,
		},
		{
			name: "matching multiple labels",
			filterConfig: &config.FilterConfig{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
				Labels: map[string]string{
					"app":         "web",
					"environment": "production",
				},
			},
			event: &watcher.Event{
				Kind:      "Pod",
				EventType: "DELETED",
				Labels: map[string]string{
					"app":         "web",
					"environment": "production",
					"version":     "v1.0.0",
				},
			},
			shouldProcess: true,
		},
		{
			name: "missing required label",
			filterConfig: &config.FilterConfig{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
				Labels: map[string]string{
					"app":         "web",
					"environment": "production",
				},
			},
			event: &watcher.Event{
				Kind:      "Pod",
				EventType: "DELETED",
				Labels: map[string]string{
					"app": "web",
				},
			},
			shouldProcess: false,
		},
		{
			name: "empty label filter allows all",
			filterConfig: &config.FilterConfig{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
				Labels:     map[string]string{},
			},
			event: &watcher.Event{
				Kind:      "Pod",
				EventType: "DELETED",
				Labels: map[string]string{
					"app": "web",
				},
			},
			shouldProcess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Filters: []config.FilterConfig{*tt.filterConfig},
			}
			f := NewFilter(cfg)

			got := f.ShouldProcess(tt.event)
			if got != tt.shouldProcess {
				t.Errorf("ShouldProcess() = %v, want %v", got, tt.shouldProcess)
			}
		})
	}
}

func TestFilter_ShouldProcess_NoFilterConfig(t *testing.T) {
	// フィルター設定がない場合は、すべてのイベントを通過させるべき
	cfg := &config.Config{
		Filters: []config.FilterConfig{},
	}
	f := NewFilter(cfg)

	event := &watcher.Event{
		Kind:      "Pod",
		EventType: "ADDED",
		Labels: map[string]string{
			"app": "test",
		},
	}

	got := f.ShouldProcess(event)
	if !got {
		t.Errorf("ShouldProcess() = %v, want true (no filter should allow all)", got)
	}
}

func TestFilter_ShouldProcess_CombinedFiltering(t *testing.T) {
	// イベントタイプとラベルの両方の条件を満たす必要がある
	tests := []struct {
		name          string
		filterConfig  *config.FilterConfig
		event         *watcher.Event
		shouldProcess bool
	}{
		{
			name: "both event type and labels match",
			filterConfig: &config.FilterConfig{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
				Labels: map[string]string{
					"environment": "production",
				},
			},
			event: &watcher.Event{
				Kind:      "Pod",
				EventType: "DELETED",
				Labels: map[string]string{
					"environment": "production",
				},
			},
			shouldProcess: true,
		},
		{
			name: "event type matches but labels do not",
			filterConfig: &config.FilterConfig{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
				Labels: map[string]string{
					"environment": "production",
				},
			},
			event: &watcher.Event{
				Kind:      "Pod",
				EventType: "DELETED",
				Labels: map[string]string{
					"environment": "development",
				},
			},
			shouldProcess: false,
		},
		{
			name: "labels match but event type does not",
			filterConfig: &config.FilterConfig{
				Resource:   "Pod",
				EventTypes: []string{"DELETED"},
				Labels: map[string]string{
					"environment": "production",
				},
			},
			event: &watcher.Event{
				Kind:      "Pod",
				EventType: "ADDED",
				Labels: map[string]string{
					"environment": "production",
				},
			},
			shouldProcess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Filters: []config.FilterConfig{*tt.filterConfig},
			}
			f := NewFilter(cfg)

			got := f.ShouldProcess(tt.event)
			if got != tt.shouldProcess {
				t.Errorf("ShouldProcess() = %v, want %v", got, tt.shouldProcess)
			}
		})
	}
}
