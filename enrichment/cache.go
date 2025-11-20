package enrichment

import (
	"sync"
	"time"
)

// EnrichmentCache кэш для результатов обогащения
type EnrichmentCache struct {
	config    *CacheConfig
	data      map[string]*cacheEntry
	mutex     sync.RWMutex
	stats     *CacheStats
}

type cacheEntry struct {
	result    *EnrichmentResult
	timestamp time.Time
}

// CacheStats статистика кэша
type CacheStats struct {
	Hits   int64 `json:"hits"`
	Misses int64 `json:"misses"`
	Size   int   `json:"size"`
}

// NewEnrichmentCache создает новый кэш
func NewEnrichmentCache(config *CacheConfig) *EnrichmentCache {
	cache := &EnrichmentCache{
		config: config,
		data:   make(map[string]*cacheEntry),
		stats:  &CacheStats{},
	}

	// Запускаем очистку устаревших записей
	if config.Enabled && config.CleanupInterval > 0 {
		go cache.startCleanup()
	}

	return cache
}

// Get возвращает результат из кэша
func (c *EnrichmentCache) Get(key string) (*EnrichmentResult, bool) {
	if !c.config.Enabled {
		c.mutex.Lock()
		c.stats.Misses++
		c.mutex.Unlock()
		return nil, false
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.data[key]
	if !exists {
		c.stats.Misses++
		return nil, false
	}

	// Проверяем TTL
	if time.Since(entry.timestamp) > c.config.TTL {
		c.stats.Misses++
		return nil, false
	}

	c.stats.Hits++
	return entry.result, true
}

// Set сохраняет результат в кэш
func (c *EnrichmentCache) Set(key string, result *EnrichmentResult) {
	if !c.config.Enabled {
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data[key] = &cacheEntry{
		result:    result,
		timestamp: time.Now(),
	}

	c.stats.Size = len(c.data)
}

// Remove удаляет запись из кэша
func (c *EnrichmentCache) Remove(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.data, key)
	c.stats.Size = len(c.data)
}

// Clear очищает весь кэш
func (c *EnrichmentCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data = make(map[string]*cacheEntry)
	c.stats.Size = 0
	c.stats.Hits = 0
	c.stats.Misses = 0
}

// GetStats возвращает статистику кэша
func (c *EnrichmentCache) GetStats() *CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	stats := *c.stats // Копируем
	stats.Size = len(c.data)
	return &stats
}

// startCleanup запускает периодическую очистку устаревших записей
func (c *EnrichmentCache) startCleanup() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup удаляет устаревшие записи
func (c *EnrichmentCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for key, entry := range c.data {
		if now.Sub(entry.timestamp) > c.config.TTL {
			delete(c.data, key)
		}
	}

	c.stats.Size = len(c.data)
}

