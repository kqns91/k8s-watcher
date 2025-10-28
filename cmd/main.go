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

		return nil
	}

	// Initialize components with initial config
	if err := initComponents(cfg); err != nil {
		log.Fatalf("Failed to initialize components: %v", err)
	}
	if deduplicator != nil {
		defer deduplicator.Stop()
	}

	// Create event handler
	eventHandler := func(event *watcher.Event) {
		// Lock components for reading
		mu.RLock()
		currentFilter := eventFilter
		currentDedup := deduplicator
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
