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
