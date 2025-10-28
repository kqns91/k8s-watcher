// Package reload provides configuration hot-reload functionality.
package reload

import (
	"log"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/kqns91/kube-watcher/pkg/config"
)

// ReloadCallback is called when configuration is reloaded
type ReloadCallback func(*config.Config) error

// ConfigWatcher watches configuration file for changes
type ConfigWatcher struct {
	configPath string
	watcher    *fsnotify.Watcher
	callbacks  []ReloadCallback
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// NewConfigWatcher creates a new ConfigWatcher
func NewConfigWatcher(configPath string) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch the directory containing the config file
	// This is necessary for Kubernetes ConfigMap updates
	configDir := filepath.Dir(configPath)
	if err := watcher.Add(configDir); err != nil {
		watcher.Close()
		return nil, err
	}

	cw := &ConfigWatcher{
		configPath: configPath,
		watcher:    watcher,
		callbacks:  make([]ReloadCallback, 0),
		stopCh:     make(chan struct{}),
	}

	return cw, nil
}

// AddCallback adds a callback to be called when config is reloaded
func (cw *ConfigWatcher) AddCallback(cb ReloadCallback) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.callbacks = append(cw.callbacks, cb)
}

// Start begins watching for configuration changes
func (cw *ConfigWatcher) Start() {
	go cw.watchLoop()
	log.Println("Configuration hot-reload enabled")
}

// Stop stops watching for configuration changes
func (cw *ConfigWatcher) Stop() {
	close(cw.stopCh)
	cw.watcher.Close()
}

// watchLoop watches for file system events
func (cw *ConfigWatcher) watchLoop() {
	for {
		select {
		case <-cw.stopCh:
			return

		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our config file
			// Kubernetes ConfigMaps create symlinks, so we need to handle various events
			if event.Name == cw.configPath || filepath.Base(event.Name) == filepath.Base(cw.configPath) {
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					log.Printf("Configuration file changed, reloading...")
					cw.reloadConfig()
				}
			}

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Config watcher error: %v", err)
		}
	}
}

// reloadConfig reloads the configuration and calls callbacks
func (cw *ConfigWatcher) reloadConfig() {
	// Load new configuration
	cfg, err := config.LoadConfig(cw.configPath)
	if err != nil {
		log.Printf("Failed to reload config: %v", err)
		return
	}

	log.Println("Configuration reloaded successfully")

	// Call all callbacks
	cw.mu.RLock()
	callbacks := make([]ReloadCallback, len(cw.callbacks))
	copy(callbacks, cw.callbacks)
	cw.mu.RUnlock()

	for _, cb := range callbacks {
		if err := cb(cfg); err != nil {
			log.Printf("Reload callback error: %v", err)
		}
	}
}
