// Package formatter provides message formatting using Go templates.
package formatter

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/yourusername/kube-watcher/pkg/watcher"
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
