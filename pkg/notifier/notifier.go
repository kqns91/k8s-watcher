// Package notifier provides notification delivery to external services.
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Notifier sends notifications to external services
type Notifier interface {
	Send(message string) error
}

// SlackNotifier sends notifications to Slack via webhook
type SlackNotifier struct {
	webhookURL string
	httpClient *http.Client
}

// SlackMessage represents a Slack message payload
type SlackMessage struct {
	Text        string            `json:"text,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment represents a Slack message attachment
type SlackAttachment struct {
	Color     string              `json:"color,omitempty"`
	Title     string              `json:"title,omitempty"`
	Text      string              `json:"text,omitempty"`
	Fields    []SlackAttachmentField `json:"fields,omitempty"`
	Timestamp int64               `json:"ts,omitempty"`
}

// SlackAttachmentField represents a field in a Slack attachment
type SlackAttachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackNotifier creates a new SlackNotifier
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send sends a message to Slack
func (s *SlackNotifier) Send(message string) error {
	payload := SlackMessage{
		Text: message,
	}

	return s.SendMessage(&payload)
}

// SendMessage sends a SlackMessage to Slack
func (s *SlackNotifier) SendMessage(payload *SlackMessage) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	req, err := http.NewRequest("POST", s.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API returned non-200 status code: %d", resp.StatusCode)
	}

	return nil
}
