package filter

import (
	"testing"
	"time"

	"github.com/kqns91/kube-watcher/pkg/watcher"
)

func TestNewCELFilter(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		wantErr    bool
	}{
		{
			name:       "valid simple expression",
			expression: `event.eventType == "DELETED"`,
			wantErr:    false,
		},
		{
			name:       "valid complex expression",
			expression: `event.namespace == "prod" && event.eventType in ["ADDED", "UPDATED"]`,
			wantErr:    false,
		},
		{
			name:       "invalid syntax",
			expression: `event.eventType ==`,
			wantErr:    true,
		},
		{
			name:       "empty expression",
			expression: ``,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCELFilter(tt.expression)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCELFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCELFilter_Evaluate(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		event      *watcher.Event
		want       bool
		wantErr    bool
	}{
		{
			name:       "simple eventType match",
			expression: `event.eventType == "DELETED"`,
			event: &watcher.Event{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
				EventType: "DELETED",
				Timestamp: time.Now(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "simple eventType mismatch",
			expression: `event.eventType == "DELETED"`,
			event: &watcher.Event{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
				EventType: "ADDED",
				Timestamp: time.Now(),
			},
			want:    false,
			wantErr: false,
		},
		{
			name:       "label match",
			expression: `event.labels.app == "web"`,
			event: &watcher.Event{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
				EventType: "ADDED",
				Labels:    map[string]string{"app": "web"},
				Timestamp: time.Now(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "complex OR condition",
			expression: `event.labels.app == "web" || event.labels.app == "api"`,
			event: &watcher.Event{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
				EventType: "ADDED",
				Labels:    map[string]string{"app": "api"},
				Timestamp: time.Now(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "AND condition with namespace",
			expression: `event.namespace == "prod" && event.eventType == "DELETED"`,
			event: &watcher.Event{
				Kind:      "Pod",
				Namespace: "prod",
				Name:      "test-pod",
				EventType: "DELETED",
				Timestamp: time.Now(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "reason filter",
			expression: `event.reason != "ReplicaSetUpdated"`,
			event: &watcher.Event{
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deploy",
				EventType: "UPDATED",
				Reason:    "NewReplicaSetAvailable",
				Timestamp: time.Now(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "reason filter - excluded",
			expression: `event.reason != "ReplicaSetUpdated"`,
			event: &watcher.Event{
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deploy",
				EventType: "UPDATED",
				Reason:    "ReplicaSetUpdated",
				Timestamp: time.Now(),
			},
			want:    false,
			wantErr: false,
		},
		{
			name:       "IN operator",
			expression: `event.eventType in ["ADDED", "DELETED"]`,
			event: &watcher.Event{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod",
				EventType: "ADDED",
				Timestamp: time.Now(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "replica count condition",
			expression: `has(event.replicas) && event.replicas.desired > 3`,
			event: &watcher.Event{
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deploy",
				EventType: "UPDATED",
				Replicas: &watcher.ReplicaInfo{
					Desired: 5,
					Ready:   3,
					Current: 4,
				},
				Timestamp: time.Now(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "replica count condition - false",
			expression: `has(event.replicas) && event.replicas.desired > 3`,
			event: &watcher.Event{
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deploy",
				EventType: "UPDATED",
				Replicas: &watcher.ReplicaInfo{
					Desired: 2,
					Ready:   2,
					Current: 2,
				},
				Timestamp: time.Now(),
			},
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewCELFilter(tt.expression)
			if err != nil {
				t.Fatalf("Failed to create CEL filter: %v", err)
			}

			got, err := filter.Evaluate(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("CELFilter.Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CELFilter.Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCELFilter_ComplexScenarios(t *testing.T) {
	t.Run("Deployment ReplicaSet filter", func(t *testing.T) {
		// 元の質問にあったケース：ReplicaSetUpdatedとNewReplicaSetAvailableを除外
		expression := `event.eventType == "UPDATED" && event.reason != "ReplicaSetUpdated" && event.reason != "NewReplicaSetAvailable"`
		filter, err := NewCELFilter(expression)
		if err != nil {
			t.Fatalf("Failed to create filter: %v", err)
		}

		// ReplicaSetUpdatedは除外される
		event1 := &watcher.Event{
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "nginx",
			EventType: "UPDATED",
			Reason:    "ReplicaSetUpdated",
			Timestamp: time.Now(),
		}
		result1, _ := filter.Evaluate(event1)
		if result1 {
			t.Error("ReplicaSetUpdated should be filtered out")
		}

		// NewReplicaSetAvailableは除外される
		event2 := &watcher.Event{
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "nginx",
			EventType: "UPDATED",
			Reason:    "NewReplicaSetAvailable",
			Timestamp: time.Now(),
		}
		result2, _ := filter.Evaluate(event2)
		if result2 {
			t.Error("NewReplicaSetAvailable should be filtered out")
		}

		// その他のUPDATEDは通す
		event3 := &watcher.Event{
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "nginx",
			EventType: "UPDATED",
			Reason:    "ScalingReplicaSet",
			Timestamp: time.Now(),
		}
		result3, _ := filter.Evaluate(event3)
		if !result3 {
			t.Error("Other UPDATED events should pass")
		}
	})
}
