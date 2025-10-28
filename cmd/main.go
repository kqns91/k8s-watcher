package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kqns91/kube-watcher/pkg/config"
	"github.com/kqns91/kube-watcher/pkg/dedup"
	"github.com/kqns91/kube-watcher/pkg/filter"
	"github.com/kqns91/kube-watcher/pkg/formatter"
	"github.com/kqns91/kube-watcher/pkg/notifier"
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

	// Initialize formatter
	fmt, err := formatter.NewFormatter(cfg.Notifier.Slack.Template)
	if err != nil {
		log.Fatalf("Failed to create formatter: %v", err)
	}

	// Initialize notifier
	slackNotifier := notifier.NewSlackNotifier(cfg.Notifier.Slack.WebhookURL)

	// Initialize filter
	eventFilter := filter.NewFilter(cfg)

	// Initialize deduplicator if enabled
	var deduplicator *dedup.Deduplicator
	if cfg.Deduplication.Enabled {
		ttl := time.Duration(cfg.Deduplication.TTLSeconds) * time.Second
		deduplicator = dedup.NewDeduplicator(ttl, cfg.Deduplication.MaxCacheSize)
		defer deduplicator.Stop()
		log.Printf("Deduplication enabled: TTL=%v, MaxCacheSize=%d", ttl, cfg.Deduplication.MaxCacheSize)
	}

	// Create event handler
	eventHandler := func(event *watcher.Event) {
		// Apply filters
		if !eventFilter.ShouldProcess(event) {
			log.Printf("Event filtered out: %s %s/%s (%s)", event.Kind, event.Namespace, event.Name, event.EventType)
			return
		}

		// Apply deduplication if enabled
		if deduplicator != nil {
			key := dedup.EventKey{
				Kind:      event.Kind,
				Namespace: event.Namespace,
				Name:      event.Name,
				EventType: event.EventType,
			}
			if !deduplicator.ShouldProcess(key, event) {
				log.Printf("Event deduplicated: %s %s/%s (%s)", event.Kind, event.Namespace, event.Name, event.EventType)
				return
			}
		}

		// Format message as Slack attachment
		slackMessage := fmt.FormatSlackMessage(event)

		// Send notification
		if err := slackNotifier.SendMessage(slackMessage); err != nil {
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
