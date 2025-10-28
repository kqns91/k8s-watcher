// Package formatter provides message formatting using Go templates.
package formatter

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/kqns91/kube-watcher/pkg/notifier"
	"github.com/kqns91/kube-watcher/pkg/watcher"
)

// BatchMode represents the batching mode
type BatchMode string

const (
	BatchModeDetailed BatchMode = "detailed"
	BatchModeSummary  BatchMode = "summary"
	BatchModeSmart    BatchMode = "smart"
)

// EventBatch represents a batch of events with timing info
type EventBatch struct {
	Events    []*watcher.Event
	StartTime time.Time
	EndTime   time.Time
}

// EventGroup represents events grouped by resource and event type
type EventGroup struct {
	Kind      string
	EventType string
	Events    []*watcher.Event
}

// Formatter formats events using Go templates
type Formatter struct {
	tmpl *template.Template
}

// NewFormatter creates a new Formatter with the given template string
func NewFormatter(templateStr string) (*Formatter, error) {
	tmpl, err := template.New("message").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &Formatter{
		tmpl: tmpl,
	}, nil
}

// TemplateData represents data available in templates
type TemplateData struct {
	Kind      string
	Namespace string
	Name      string
	EventType string
	Timestamp string
	Labels    map[string]string
}

// Format formats an event using the configured template
func (f *Formatter) Format(event *watcher.Event) (string, error) {
	data := TemplateData{
		Kind:      event.Kind,
		Namespace: event.Namespace,
		Name:      event.Name,
		EventType: event.EventType,
		Timestamp: event.Timestamp.Format(time.RFC3339),
		Labels:    event.Labels,
	}

	var buf bytes.Buffer
	if err := f.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// FormatSlackMessage formats an event as a Slack message with attachments
func (f *Formatter) FormatSlackMessage(event *watcher.Event) *notifier.SlackMessage {
	// Determine color based on event type
	color := getEventColor(event.EventType)

	// Create title
	title := fmt.Sprintf("[%s] %s/%s", event.Kind, event.Namespace, event.Name)

	// Create fields
	fields := []notifier.SlackAttachmentField{
		{
			Title: "イベントタイプ",
			Value: event.EventType,
			Short: true,
		},
		{
			Title: "時刻",
			Value: event.Timestamp.Format(time.RFC3339),
			Short: true,
		},
	}

	// Add status if available
	if event.Status != "" {
		fields = append(fields, notifier.SlackAttachmentField{
			Title: "ステータス",
			Value: event.Status,
			Short: true,
		})
	}

	// Add service type for services
	if event.ServiceType != "" {
		fields = append(fields, notifier.SlackAttachmentField{
			Title: "サービスタイプ",
			Value: event.ServiceType,
			Short: true,
		})
	}

	// Add replica information if available
	if event.Replicas != nil {
		replicaInfo := fmt.Sprintf("Desired: %d, Ready: %d, Current: %d",
			event.Replicas.Desired, event.Replicas.Ready, event.Replicas.Current)
		fields = append(fields, notifier.SlackAttachmentField{
			Title: "レプリカ",
			Value: replicaInfo,
			Short: false,
		})
	}

	// Add container information if available
	if len(event.Containers) > 0 {
		var containerInfos []string
		for _, c := range event.Containers {
			containerInfos = append(containerInfos, fmt.Sprintf("• %s: `%s`", c.Name, c.Image))
		}
		fields = append(fields, notifier.SlackAttachmentField{
			Title: "コンテナ",
			Value: strings.Join(containerInfos, "\n"),
			Short: false,
		})
	}

	// Add reason if available
	if event.Reason != "" {
		fields = append(fields, notifier.SlackAttachmentField{
			Title: "理由",
			Value: event.Reason,
			Short: false,
		})
	}

	// Add message if available
	if event.Message != "" {
		fields = append(fields, notifier.SlackAttachmentField{
			Title: "メッセージ",
			Value: event.Message,
			Short: false,
		})
	}

	attachment := notifier.SlackAttachment{
		Color:     color,
		Title:     title,
		Fields:    fields,
		Timestamp: event.Timestamp.Unix(),
	}

	return &notifier.SlackMessage{
		Attachments: []notifier.SlackAttachment{attachment},
	}
}

// getEventColor returns the color for an event type
func getEventColor(eventType string) string {
	switch eventType {
	case "ADDED":
		return "good" // green
	case "UPDATED":
		return "warning" // yellow
	case "DELETED":
		return "danger" // red
	default:
		return "#808080" // gray
	}
}

// getEventEmoji returns the emoji for an event type
func getEventEmoji(eventType string) string {
	switch eventType {
	case "ADDED":
		return "✅"
	case "UPDATED":
		return "🟡"
	case "DELETED":
		return "🔴"
	default:
		return "📌"
	}
}

// FormatBatchSlackMessage formats a batch of events as a Slack message
func (f *Formatter) FormatBatchSlackMessage(batch *EventBatch, mode BatchMode, maxEventsPerGroup int, alwaysShowDetails []string) *notifier.SlackMessage {
	totalEvents := len(batch.Events)
	duration := batch.EndTime.Sub(batch.StartTime)

	// Group events by Kind and EventType
	groups := groupEvents(batch.Events)

	// Determine if we should use summary mode
	useSummary := mode == BatchModeSummary || (mode == BatchModeSmart && totalEvents > 20)

	// Create main text
	mainText := fmt.Sprintf("📦 *過去%.0f秒間の変更 (%d件)*", duration.Seconds(), totalEvents)

	var attachments []notifier.SlackAttachment

	for _, group := range groups {
		eventCount := len(group.Events)
		emoji := getEventEmoji(group.EventType)
		color := getEventColor(group.EventType)

		// Check if we should show details for this group
		showDetails := !useSummary && shouldShowDetailsForGroup(mode, group.EventType, eventCount, maxEventsPerGroup, alwaysShowDetails)

		if showDetails {
			// Detailed mode: show individual events
			for _, event := range group.Events {
				title := fmt.Sprintf("%s [%s] %s/%s", emoji, event.Kind, event.Namespace, event.Name)
				fields := buildEventFields(event)

				attachments = append(attachments, notifier.SlackAttachment{
					Color:     color,
					Title:     title,
					Fields:    fields,
					Timestamp: event.Timestamp.Unix(),
				})
			}
		} else {
			// Summary mode: group similar events
			title := fmt.Sprintf("%s %s (%d件)", emoji, group.Kind, eventCount)

			// Create summary fields
			fields := []notifier.SlackAttachmentField{
				{
					Title: "イベントタイプ",
					Value: group.EventType,
					Short: true,
				},
				{
					Title: "件数",
					Value: fmt.Sprintf("%d件", eventCount),
					Short: true,
				},
			}

			// Add resource names (up to 10)
			var names []string
			for i, event := range group.Events {
				if i >= 10 {
					names = append(names, fmt.Sprintf("... 他%d件", eventCount-10))
					break
				}
				names = append(names, event.Name)
			}

			fields = append(fields, notifier.SlackAttachmentField{
				Title: "リソース",
				Value: strings.Join(names, ", "),
				Short: false,
			})

			attachments = append(attachments, notifier.SlackAttachment{
				Color:  color,
				Title:  title,
				Fields: fields,
			})
		}
	}

	return &notifier.SlackMessage{
		Text:        mainText,
		Attachments: attachments,
	}
}

// groupEvents groups events by Kind and EventType
func groupEvents(events []*watcher.Event) []EventGroup {
	groupMap := make(map[string]*EventGroup)

	for _, event := range events {
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

// shouldShowDetailsForGroup determines if details should be shown for a group
func shouldShowDetailsForGroup(mode BatchMode, eventType string, eventCount int, maxEventsPerGroup int, alwaysShowDetails []string) bool {
	// Always show details mode
	if mode == BatchModeDetailed {
		return true
	}

	// Check if this event type should always show details
	for _, alwaysType := range alwaysShowDetails {
		if alwaysType == eventType {
			return true
		}
	}

	// Smart mode: show details if count is below threshold
	if mode == BatchModeSmart {
		return eventCount <= maxEventsPerGroup
	}

	return false
}

// buildEventFields builds Slack attachment fields for an event
func buildEventFields(event *watcher.Event) []notifier.SlackAttachmentField {
	fields := []notifier.SlackAttachmentField{
		{
			Title: "イベントタイプ",
			Value: event.EventType,
			Short: true,
		},
		{
			Title: "時刻",
			Value: event.Timestamp.Format(time.RFC3339),
			Short: true,
		},
	}

	// Add status if available
	if event.Status != "" {
		fields = append(fields, notifier.SlackAttachmentField{
			Title: "ステータス",
			Value: event.Status,
			Short: true,
		})
	}

	// Add replica information if available
	if event.Replicas != nil {
		replicaInfo := fmt.Sprintf("Desired: %d, Ready: %d, Current: %d",
			event.Replicas.Desired, event.Replicas.Ready, event.Replicas.Current)
		fields = append(fields, notifier.SlackAttachmentField{
			Title: "レプリカ",
			Value: replicaInfo,
			Short: false,
		})
	}

	// Add container information if available (limit to 3)
	if len(event.Containers) > 0 {
		var containerInfos []string
		for i, c := range event.Containers {
			if i >= 3 {
				containerInfos = append(containerInfos, fmt.Sprintf("... 他%d個", len(event.Containers)-3))
				break
			}
			containerInfos = append(containerInfos, fmt.Sprintf("• %s: `%s`", c.Name, c.Image))
		}
		fields = append(fields, notifier.SlackAttachmentField{
			Title: "コンテナ",
			Value: strings.Join(containerInfos, "\n"),
			Short: false,
		})
	}

	return fields
}
