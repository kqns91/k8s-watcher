package batcher

import (
	"fmt"
	"sync"
	"time"

	"github.com/kqns91/kube-watcher/pkg/watcher"
)

// BatchMode represents the batching mode
type BatchMode string

const (
	BatchModeDetailed BatchMode = "detailed" // Show all events with details
	BatchModeSummary  BatchMode = "summary"  // Show only summary counts
	BatchModeSmart    BatchMode = "smart"    // Smart grouping based on config
)

// SmartConfig represents smart batching configuration
type SmartConfig struct {
	MaxEventsPerGroup int      // Maximum events to show details per group
	MaxTotalEvents    int      // Maximum total events before forcing summary mode
	AlwaysShowDetails []string // Event types to always show details (e.g., "DELETED")
}

// Config represents batching configuration
type Config struct {
	Enabled       bool
	WindowSeconds int
	Mode          BatchMode
	Smart         SmartConfig
}

// Batch represents a collection of events to be sent together
type Batch struct {
	Events    []*watcher.Event
	StartTime time.Time
	EndTime   time.Time
}

// EventGroup represents events grouped by resource type and event type
type EventGroup struct {
	Kind      string
	EventType string
	Events    []*watcher.Event
}

// Batcher collects events and sends them in batches
type Batcher struct {
	config    Config
	events    []*watcher.Event
	mu        sync.Mutex
	timer     *time.Timer
	callback  func(*Batch)
	startTime time.Time
	stopCh    chan struct{}
}

// NewBatcher creates a new Batcher instance
func NewBatcher(config Config, callback func(*Batch)) *Batcher {
	return &Batcher{
		config:    config,
		events:    make([]*watcher.Event, 0),
		callback:  callback,
		startTime: time.Now(),
		stopCh:    make(chan struct{}),
	}
}

// Add adds an event to the current batch
func (b *Batcher) Add(event *watcher.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Add event to the batch
	b.events = append(b.events, event)

	// Start timer if this is the first event
	if len(b.events) == 1 {
		b.startTime = time.Now()
		b.timer = time.AfterFunc(time.Duration(b.config.WindowSeconds)*time.Second, func() {
			b.flush()
		})
	}
}

// flush sends the current batch and resets
func (b *Batcher) flush() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.events) == 0 {
		return
	}

	// Create batch
	batch := &Batch{
		Events:    b.events,
		StartTime: b.startTime,
		EndTime:   time.Now(),
	}

	// Reset state
	b.events = make([]*watcher.Event, 0)
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}

	// Send batch via callback (unlock before calling to avoid deadlock)
	b.mu.Unlock()
	b.callback(batch)
	b.mu.Lock()
}

// Stop stops the batcher and flushes remaining events
func (b *Batcher) Stop() {
	close(b.stopCh)
	b.flush()
}

// GroupEvents groups events by Kind and EventType
func (b *Batch) GroupEvents() []EventGroup {
	groupMap := make(map[string]*EventGroup)

	for _, event := range b.Events {
		key := fmt.Sprintf("%s:%s", event.Kind, event.EventType)
		if group, exists := groupMap[key]; exists {
			group.Events = append(group.Events, event)
		} else {
			groupMap[key] = &EventGroup{
				Kind:      event.Kind,
				EventType: event.EventType,
				Events:    []*watcher.Event{event},
			}
		}
	}

	// Convert map to slice
	groups := make([]EventGroup, 0, len(groupMap))
	for _, group := range groupMap {
		groups = append(groups, *group)
	}

	return groups
}

// ShouldShowDetails determines if details should be shown for an event type
func (b *Batcher) ShouldShowDetails(eventType string, eventCount int) bool {
	// Always show details mode
	if b.config.Mode == BatchModeDetailed {
		return true
	}

	// Never show details mode
	if b.config.Mode == BatchModeSummary {
		return false
	}

	// Smart mode
	if b.config.Mode == BatchModeSmart {
		// Check if this event type should always show details
		for _, alwaysDetailType := range b.config.Smart.AlwaysShowDetails {
			if alwaysDetailType == eventType {
				return true
			}
		}

		// If total events exceed threshold, force summary
		totalEvents := 0
		b.mu.Lock()
		totalEvents = len(b.events)
		b.mu.Unlock()

		if b.config.Smart.MaxTotalEvents > 0 && totalEvents > b.config.Smart.MaxTotalEvents {
			return false
		}

		// If event count exceeds per-group threshold, show summary
		if b.config.Smart.MaxEventsPerGroup > 0 && eventCount > b.config.Smart.MaxEventsPerGroup {
			return false
		}

		return true
	}

	return true
}
