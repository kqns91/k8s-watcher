package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourusername/kube-watcher/pkg/config"
	"github.com/yourusername/kube-watcher/pkg/filter"
	"github.com/yourusername/kube-watcher/pkg/formatter"
	"github.com/yourusername/kube-watcher/pkg/notifier"
	"github.com/yourusername/kube-watcher/pkg/watcher"
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

	// Create event handler
	eventHandler := func(event *watcher.Event) {
		// Apply filters
		if !eventFilter.ShouldProcess(event) {
			log.Printf("Event filtered out: %s %s/%s (%s)", event.Kind, event.Namespace, event.Name, event.EventType)
			return
		}

		// Format message
		message, err := fmt.Format(event)
		if err != nil {
			log.Printf("Failed to format event: %v", err)
			return
		}

		// Send notification
		if err := slackNotifier.Send(message); err != nil {
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
