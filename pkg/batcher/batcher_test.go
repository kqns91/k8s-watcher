package batcher

import (
	"testing"
	"time"

	"github.com/kqns91/kube-watcher/pkg/watcher"
)

func TestBatcher_Add(t *testing.T) {
	callbackCalled := false
	var receivedBatch *Batch

	callback := func(batch *Batch) {
		callbackCalled = true
		receivedBatch = batch
	}

	config := Config{
		Enabled:       true,
		WindowSeconds: 1, // 1 second for testing
		Mode:          BatchModeSmart,
	}

	b := NewBatcher(config, callback)
	defer b.Stop()

	// Add an event
	event := &watcher.Event{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "test-pod",
		EventType: "ADDED",
		Timestamp: time.Now(),
	}

	b.Add(event)

	// Wait for batch to be flushed
	time.Sleep(1500 * time.Millisecond)

	if !callbackCalled {
		t.Error("Callback was not called")
	}

	if receivedBatch == nil {
		t.Fatal("Received batch is nil")
	}

	if len(receivedBatch.Events) != 1 {
		t.Errorf("Expected 1 event in batch, got %d", len(receivedBatch.Events))
	}

	if receivedBatch.Events[0].Name != "test-pod" {
		t.Errorf("Expected event name 'test-pod', got '%s'", receivedBatch.Events[0].Name)
	}
}

func TestBatcher_MultipleEvents(t *testing.T) {
	var receivedBatch *Batch

	callback := func(batch *Batch) {
		receivedBatch = batch
	}

	config := Config{
		Enabled:       true,
		WindowSeconds: 1,
		Mode:          BatchModeSmart,
	}

	b := NewBatcher(config, callback)
	defer b.Stop()

	// Add multiple events
	for i := 0; i < 5; i++ {
		event := &watcher.Event{
			Kind:      "Pod",
			Namespace: "default",
			Name:      "test-pod",
			EventType: "ADDED",
			Timestamp: time.Now(),
		}
		b.Add(event)
	}

	// Wait for batch to be flushed
	time.Sleep(1500 * time.Millisecond)

	if receivedBatch == nil {
		t.Fatal("Received batch is nil")
	}

	if len(receivedBatch.Events) != 5 {
		t.Errorf("Expected 5 events in batch, got %d", len(receivedBatch.Events))
	}
}

func TestBatch_GroupEvents(t *testing.T) {
	batch := &Batch{
		Events: []*watcher.Event{
			{Kind: "Pod", EventType: "ADDED", Name: "pod1"},
			{Kind: "Pod", EventType: "ADDED", Name: "pod2"},
			{Kind: "Pod", EventType: "DELETED", Name: "pod3"},
			{Kind: "Deployment", EventType: "UPDATED", Name: "deploy1"},
		},
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	groups := batch.GroupEvents()

	if len(groups) != 3 {
		t.Errorf("Expected 3 groups, got %d", len(groups))
	}

	// Check that Pod:ADDED group has 2 events
	foundPodAdded := false
	for _, group := range groups {
		if group.Kind == "Pod" && group.EventType == "ADDED" {
			foundPodAdded = true
			if len(group.Events) != 2 {
				t.Errorf("Expected 2 events in Pod:ADDED group, got %d", len(group.Events))
			}
		}
	}

	if !foundPodAdded {
		t.Error("Pod:ADDED group not found")
	}
}

func TestBatcher_ShouldShowDetails(t *testing.T) {
	tests := []struct {
		name              string
		mode              BatchMode
		eventType         string
		eventCount        int
		maxEventsPerGroup int
		alwaysShowDetails []string
		expected          bool
	}{
		{
			name:              "Detailed mode always shows details",
			mode:              BatchModeDetailed,
			eventType:         "ADDED",
			eventCount:        10,
			maxEventsPerGroup: 5,
			alwaysShowDetails: []string{},
			expected:          true,
		},
		{
			name:              "Summary mode never shows details",
			mode:              BatchModeSummary,
			eventType:         "ADDED",
			eventCount:        3,
			maxEventsPerGroup: 5,
			alwaysShowDetails: []string{},
			expected:          false,
		},
		{
			name:              "Smart mode shows details for DELETED",
			mode:              BatchModeSmart,
			eventType:         "DELETED",
			eventCount:        10,
			maxEventsPerGroup: 5,
			alwaysShowDetails: []string{"DELETED"},
			expected:          true,
		},
		{
			name:              "Smart mode shows details when count is low",
			mode:              BatchModeSmart,
			eventType:         "ADDED",
			eventCount:        3,
			maxEventsPerGroup: 5,
			alwaysShowDetails: []string{},
			expected:          true,
		},
		{
			name:              "Smart mode hides details when count is high",
			mode:              BatchModeSmart,
			eventType:         "ADDED",
			eventCount:        10,
			maxEventsPerGroup: 5,
			alwaysShowDetails: []string{},
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Mode: tt.mode,
				Smart: SmartConfig{
					MaxEventsPerGroup: tt.maxEventsPerGroup,
					AlwaysShowDetails: tt.alwaysShowDetails,
				},
			}

			b := NewBatcher(config, func(batch *Batch) {})
			result := b.ShouldShowDetails(tt.eventType, tt.eventCount)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
