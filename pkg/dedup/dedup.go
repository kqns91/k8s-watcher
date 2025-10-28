// Package dedup provides event deduplication using LRU cache.
package dedup

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// EventKey represents a unique key for an event
type EventKey struct {
	Kind      string
	Namespace string
	Name      string
	EventType string
}

// EventSignature represents the full signature of an event for deduplication
type EventSignature struct {
	Key       EventKey
	Signature string // Hash of relevant fields
}

// CacheEntry represents a cached event with timestamp
type CacheEntry struct {
	Signature string
	Timestamp time.Time
}

// Deduplicator provides event deduplication functionality
type Deduplicator struct {
	cache    map[string]CacheEntry
	mu       sync.RWMutex
	ttl      time.Duration
	maxSize  int
	cleanupC chan struct{}
	stopC    chan struct{}
}

// NewDeduplicator creates a new Deduplicator with specified TTL and max cache size
func NewDeduplicator(ttl time.Duration, maxSize int) *Deduplicator {
	d := &Deduplicator{
		cache:    make(map[string]CacheEntry),
		ttl:      ttl,
		maxSize:  maxSize,
		cleanupC: make(chan struct{}, 1),
		stopC:    make(chan struct{}),
	}

	// Start background cleanup goroutine
	go d.cleanupLoop()

	return d
}

// ShouldProcess checks if an event should be processed (not a duplicate)
func (d *Deduplicator) ShouldProcess(key EventKey, data interface{}) bool {
	signature := d.generateSignature(data)
	cacheKey := d.makeCacheKey(key)

	d.mu.RLock()
	entry, exists := d.cache[cacheKey]
	d.mu.RUnlock()

	if exists {
		// Check if signature matches and entry is still valid
		if entry.Signature == signature && time.Since(entry.Timestamp) < d.ttl {
			// Duplicate event within TTL
			return false
		}
	}

	// New event or expired cache entry, update cache
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check cache size and evict oldest entry if necessary
	if len(d.cache) >= d.maxSize {
		d.evictOldest()
	}

	d.cache[cacheKey] = CacheEntry{
		Signature: signature,
		Timestamp: time.Now(),
	}

	// Trigger async cleanup
	select {
	case d.cleanupC <- struct{}{}:
	default:
	}

	return true
}

// generateSignature generates a hash signature for the given data
func (d *Deduplicator) generateSignature(data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		// If marshaling fails, return a timestamp-based signature
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	hash := sha256.Sum256(jsonData)
	return fmt.Sprintf("%x", hash)
}

// makeCacheKey creates a cache key from EventKey
func (d *Deduplicator) makeCacheKey(key EventKey) string {
	return fmt.Sprintf("%s/%s/%s/%s", key.Kind, key.Namespace, key.Name, key.EventType)
}

// evictOldest removes the oldest entry from the cache
func (d *Deduplicator) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true

	for k, v := range d.cache {
		if first || v.Timestamp.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.Timestamp
			first = false
		}
	}

	if oldestKey != "" {
		delete(d.cache, oldestKey)
	}
}

// cleanupLoop periodically removes expired entries from cache
func (d *Deduplicator) cleanupLoop() {
	ticker := time.NewTicker(d.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopC:
			return
		case <-ticker.C:
			d.cleanup()
		case <-d.cleanupC:
			// Immediate cleanup requested
			d.cleanup()
		}
	}
}

// cleanup removes expired entries from cache
func (d *Deduplicator) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for k, v := range d.cache {
		if now.Sub(v.Timestamp) >= d.ttl {
			delete(d.cache, k)
		}
	}
}

// Stop stops the background cleanup goroutine
func (d *Deduplicator) Stop() {
	close(d.stopC)
}

// Stats returns current cache statistics
func (d *Deduplicator) Stats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]interface{}{
		"size":     len(d.cache),
		"max_size": d.maxSize,
		"ttl":      d.ttl.String(),
	}
}
