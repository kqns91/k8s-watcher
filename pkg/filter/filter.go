// Package filter provides event filtering functionality based on configured rules.
package filter

import (
	"github.com/kqns91/kube-watcher/pkg/config"
	"github.com/kqns91/kube-watcher/pkg/watcher"
)

// Filter checks if an event should be processed based on configured rules
type Filter struct {
	config *config.Config
}

// NewFilter creates a new Filter instance
func NewFilter(cfg *config.Config) *Filter {
	return &Filter{
		config: cfg,
	}
}

// ShouldProcess determines if an event should be processed
func (f *Filter) ShouldProcess(event *watcher.Event) bool {
	// Get filter configuration for this resource kind
	filterConfig := f.config.GetFilterForResource(event.Kind)
	if filterConfig == nil {
		// No filter configured, allow by default
		return true
	}

	// Check event type
	if !f.matchesEventType(event.EventType, filterConfig.EventTypes) {
		return false
	}

	// Check labels if specified
	if len(filterConfig.Labels) > 0 && !f.matchesLabels(event.Labels, filterConfig.Labels) {
		return false
	}

	return true
}

// matchesEventType checks if the event type matches any of the configured types
func (f *Filter) matchesEventType(eventType string, allowedTypes []string) bool {
	if len(allowedTypes) == 0 {
		return true
	}

	for _, t := range allowedTypes {
		if t == eventType {
			return true
		}
	}

	return false
}

// matchesLabels checks if the event labels match all configured labels
func (f *Filter) matchesLabels(eventLabels, requiredLabels map[string]string) bool {
	if len(requiredLabels) == 0 {
		return true
	}

	for key, value := range requiredLabels {
		if eventLabels[key] != value {
			return false
		}
	}

	return true
}
