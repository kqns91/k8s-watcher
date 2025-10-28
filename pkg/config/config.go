// Package config provides configuration management for kube-watcher.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Namespace      string             `yaml:"namespace"`
	Resources      []ResourceConfig   `yaml:"resources"`
	Filters        []FilterConfig     `yaml:"filters"`
	Notifier       NotifierConfig     `yaml:"notifier"`
	Deduplication  DeduplicationConfig `yaml:"deduplication,omitempty"`
}

// ResourceConfig defines which Kubernetes resources to watch
type ResourceConfig struct {
	Kind string `yaml:"kind"`
}

// FilterConfig defines conditions for filtering events
type FilterConfig struct {
	Resource   string            `yaml:"resource"`
	EventTypes []string          `yaml:"eventTypes"`
	Labels     map[string]string `yaml:"labels,omitempty"`
}

// NotifierConfig defines notification settings
type NotifierConfig struct {
	Slack SlackConfig `yaml:"slack"`
}

// SlackConfig contains Slack webhook configuration
type SlackConfig struct {
	WebhookURL string `yaml:"webhookUrl"`
	Template   string `yaml:"template"`
}

// DeduplicationConfig contains event deduplication settings
type DeduplicationConfig struct {
	Enabled      bool   `yaml:"enabled"`
	TTLSeconds   int    `yaml:"ttlSeconds"`
	MaxCacheSize int    `yaml:"maxCacheSize"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	if len(c.Resources) == 0 {
		return fmt.Errorf("at least one resource must be configured")
	}

	if c.Notifier.Slack.WebhookURL == "" {
		return fmt.Errorf("slack webhook URL is required")
	}

	if c.Notifier.Slack.Template == "" {
		c.Notifier.Slack.Template = "[{{ .Kind }}] {{ .Namespace }}/{{ .Name }} was {{ .EventType }}"
	}

	// Set deduplication defaults if not specified
	if c.Deduplication.Enabled {
		if c.Deduplication.TTLSeconds <= 0 {
			c.Deduplication.TTLSeconds = 300 // Default: 5 minutes
		}
		if c.Deduplication.MaxCacheSize <= 0 {
			c.Deduplication.MaxCacheSize = 1000 // Default: 1000 entries
		}
	}

	return nil
}

// GetFilterForResource returns the filter configuration for a given resource kind
func (c *Config) GetFilterForResource(kind string) *FilterConfig {
	for i := range c.Filters {
		if c.Filters[i].Resource == kind {
			return &c.Filters[i]
		}
	}
	return nil
}
