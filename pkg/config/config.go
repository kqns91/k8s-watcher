// Package config provides configuration management for kube-watcher.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Namespace      string              `yaml:"namespace"`
	Resources      []ResourceConfig    `yaml:"resources"`
	Filters        []FilterConfig      `yaml:"filters"`
	Notifier       NotifierConfig      `yaml:"notifier"`
	Deduplication  DeduplicationConfig `yaml:"deduplication,omitempty"`
	Batching       BatchingConfig      `yaml:"batching,omitempty"`
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

// BatchingConfig contains event batching settings
type BatchingConfig struct {
	Enabled       bool                `yaml:"enabled"`
	WindowSeconds int                 `yaml:"windowSeconds"`
	Mode          string              `yaml:"mode"` // "detailed" | "summary" | "smart"
	Smart         SmartBatchingConfig `yaml:"smart"`
}

// SmartBatchingConfig contains smart batching settings
type SmartBatchingConfig struct {
	MaxEventsPerGroup int      `yaml:"maxEventsPerGroup"`
	MaxTotalEvents    int      `yaml:"maxTotalEvents"`
	AlwaysShowDetails []string `yaml:"alwaysShowDetails"`
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

	// Validate and set batching defaults
	if c.Batching.Enabled {
		if c.Batching.WindowSeconds < 30 {
			return fmt.Errorf("batching.windowSeconds must be at least 30 seconds (got %d)", c.Batching.WindowSeconds)
		}
		if c.Batching.WindowSeconds > 600 {
			fmt.Printf("Warning: batching.windowSeconds is %d (>10min). Consider using a shorter window for better responsiveness.\n", c.Batching.WindowSeconds)
		}

		// Set default mode if not specified
		if c.Batching.Mode == "" {
			c.Batching.Mode = "smart"
		}

		// Validate mode
		validModes := map[string]bool{"detailed": true, "summary": true, "smart": true}
		if !validModes[c.Batching.Mode] {
			return fmt.Errorf("batching.mode must be one of: detailed, summary, smart (got %s)", c.Batching.Mode)
		}

		// Set smart batching defaults
		if c.Batching.Mode == "smart" {
			if c.Batching.Smart.MaxEventsPerGroup <= 0 {
				c.Batching.Smart.MaxEventsPerGroup = 5 // Default: show details for up to 5 events per group
			}
			if c.Batching.Smart.MaxTotalEvents <= 0 {
				c.Batching.Smart.MaxTotalEvents = 20 // Default: force summary if >20 total events
			}
			if len(c.Batching.Smart.AlwaysShowDetails) == 0 {
				c.Batching.Smart.AlwaysShowDetails = []string{"DELETED"} // Default: always show deleted events
			}
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
