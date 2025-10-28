package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kqns91/kube-watcher/pkg/batcher"
	"github.com/kqns91/kube-watcher/pkg/config"
	"github.com/kqns91/kube-watcher/pkg/dedup"
	"github.com/kqns91/kube-watcher/pkg/filter"
	"github.com/kqns91/kube-watcher/pkg/formatter"
	"github.com/kqns91/kube-watcher/pkg/notifier"
	"github.com/kqns91/kube-watcher/pkg/reload"
	"github.com/kqns91/kube-watcher/pkg/watcher"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting kube-watcher for namespace: %s", cfg.Namespace)

	// Components that can be reloaded
	var (
		fmt           *formatter.Formatter
		eventFilter   *filter.Filter
		deduplicator  *dedup.Deduplicator
		eventBatcher  *batcher.Batcher
		slackNotifier *notifier.SlackNotifier
		mu            sync.RWMutex // Protects the components above
	)

	// Initialize components
	initComponents := func(c *config.Config) error {
		mu.Lock()
		defer mu.Unlock()

		// Initialize formatter
		newFmt, err := formatter.NewFormatter(c.Notifier.Slack.Template)
		if err != nil {
			return err
		}
		fmt = newFmt

		// Initialize notifier
		slackNotifier = notifier.NewSlackNotifier(c.Notifier.Slack.WebhookURL)

		// Initialize filter
		eventFilter = filter.NewFilter(c)

		// Initialize or update deduplicator
		if c.Deduplication.Enabled {
			if deduplicator != nil {
				deduplicator.Stop()
			}
			ttl := time.Duration(c.Deduplication.TTLSeconds) * time.Second
			deduplicator = dedup.NewDeduplicator(ttl, c.Deduplication.MaxCacheSize)
			log.Printf("Deduplication enabled: TTL=%v, MaxCacheSize=%d", ttl, c.Deduplication.MaxCacheSize)
		} else if deduplicator != nil {
			deduplicator.Stop()
			deduplicator = nil
			log.Println("Deduplication disabled")
		}

		// Initialize or update batcher
		if c.Batching.Enabled {
			if eventBatcher != nil {
				eventBatcher.Stop()
			}

			// Create batch handler
			batchHandler := func(batch *batcher.Batch) {
				// Convert batcher.Batch to formatter.EventBatch
				formatterBatch := &formatter.EventBatch{
					Events:    batch.Events,
					StartTime: batch.StartTime,
					EndTime:   batch.EndTime,
				}

				// Format batch message
				mu.RLock()
				currentFormatter := fmt
				currentNotifier := slackNotifier
				currentConfig := c
				mu.RUnlock()

				mode := formatter.BatchMode(currentConfig.Batching.Mode)
				slackMessage := currentFormatter.FormatBatchSlackMessage(
					formatterBatch,
					mode,
					currentConfig.Batching.Smart.MaxEventsPerGroup,
					currentConfig.Batching.Smart.AlwaysShowDetails,
				)

				// Send batch notification
				if err := currentNotifier.SendMessage(slackMessage); err != nil {
					log.Printf("Failed to send batch notification: %v", err)
					return
				}

				log.Printf("Batch notification sent: %d events", len(batch.Events))
			}

			// Create batcher config
			batchConfig := batcher.Config{
				Enabled:       c.Batching.Enabled,
				WindowSeconds: c.Batching.WindowSeconds,
				Mode:          batcher.BatchMode(c.Batching.Mode),
				Smart: batcher.SmartConfig{
					MaxEventsPerGroup: c.Batching.Smart.MaxEventsPerGroup,
					MaxTotalEvents:    c.Batching.Smart.MaxTotalEvents,
					AlwaysShowDetails: c.Batching.Smart.AlwaysShowDetails,
				},
			}

			eventBatcher = batcher.NewBatcher(batchConfig, batchHandler)
			log.Printf("Batching enabled: Window=%ds, Mode=%s", c.Batching.WindowSeconds, c.Batching.Mode)
		} else if eventBatcher != nil {
			eventBatcher.Stop()
			eventBatcher = nil
			log.Println("Batching disabled")
		}

		return nil
	}

	// Initialize components with initial config
	if err := initComponents(cfg); err != nil {
		log.Fatalf("Failed to initialize components: %v", err)
	}
	if deduplicator != nil {
		defer deduplicator.Stop()
	}
	if eventBatcher != nil {
		defer eventBatcher.Stop()
	}

	// Create event handler
	eventHandler := func(event *watcher.Event) {
		// Lock components for reading
		mu.RLock()
		currentFilter := eventFilter
		currentDedup := deduplicator
		currentBatcher := eventBatcher
		currentFormatter := fmt
		currentNotifier := slackNotifier
		mu.RUnlock()

		// Apply filters
		if !currentFilter.ShouldProcess(event) {
			log.Printf("Event filtered out: %s %s/%s (%s)", event.Kind, event.Namespace, event.Name, event.EventType)
			return
		}

		// Apply deduplication if enabled
		if currentDedup != nil {
			key := dedup.EventKey{
				Kind:      event.Kind,
				Namespace: event.Namespace,
				Name:      event.Name,
				EventType: event.EventType,
			}
			if !currentDedup.ShouldProcess(key, event) {
				log.Printf("Event deduplicated: %s %s/%s (%s)", event.Kind, event.Namespace, event.Name, event.EventType)
				return
			}
		}

		// If batching is enabled, add to batcher
		if currentBatcher != nil {
			currentBatcher.Add(event)
			log.Printf("Event added to batch: %s %s/%s (%s)", event.Kind, event.Namespace, event.Name, event.EventType)
			return
		}

		// Otherwise, send immediately
		// Format message as Slack attachment
		slackMessage := currentFormatter.FormatSlackMessage(event)

		// Send notification
		if err := currentNotifier.SendMessage(slackMessage); err != nil {
			log.Printf("Failed to send notification: %v", err)
			return
		}

		log.Printf("Notification sent: %s %s/%s (%s)", event.Kind, event.Namespace, event.Name, event.EventType)
	}

	// Initialize watcher
	w, err := watcher.NewWatcher(cfg, eventHandler)
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}

	// Setup config hot-reload
	configWatcher, err := reload.NewConfigWatcher(*configPath)
	if err != nil {
		log.Printf("Failed to create config watcher: %v (hot-reload disabled)", err)
	} else {
		configWatcher.AddCallback(func(newCfg *config.Config) error {
			log.Printf("Applying new configuration for namespace: %s", newCfg.Namespace)
			return initComponents(newCfg)
		})
		configWatcher.Start()
		defer configWatcher.Stop()
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received shutdown signal, stopping...")
		cancel()
	}()

	// Start watching
	log.Println("Starting watchers...")
	if err := w.Start(ctx); err != nil {
		log.Fatalf("Watcher error: %v", err)
	}

	log.Println("kube-watcher stopped")
}
