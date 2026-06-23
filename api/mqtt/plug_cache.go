package mqtt

import (
	"context"
	"log/slog"
	"sync"

	"ev-charge-controller/api/internal"
)

// NamespaceSlug is the cache key: the namespace and MQTT slug of a plug.
type NamespaceSlug struct {
	Namespace string
	Slug      string
}

// PlugCache maps (namespace, slug) → plugID, populated lazily from the DB.
type PlugCache struct {
	repo internal.PlugRepo
	mu   sync.RWMutex
	m    map[NamespaceSlug]string
}

// NewPlugCache creates a new PlugCache backed by the given repo.
func NewPlugCache(repo internal.PlugRepo) *PlugCache {
	return &PlugCache{repo: repo, m: make(map[NamespaceSlug]string)}
}

// NewStaticPlugCache creates a PlugCache pre-seeded with the given entries; no DB needed.
// Intended for tests.
func NewStaticPlugCache(entries map[NamespaceSlug]string) *PlugCache {
	m := make(map[NamespaceSlug]string, len(entries))
	for k, v := range entries {
		m[k] = v
	}
	return &PlugCache{m: m}
}

// Lookup returns the plugID for the given namespace+slug, loading from DB on cache miss.
func (c *PlugCache) Lookup(namespace, slug string) (string, bool) {
	key := NamespaceSlug{namespace, slug}

	c.mu.RLock()
	id, ok := c.m[key]
	c.mu.RUnlock()
	if ok {
		return id, true
	}

	if c.repo == nil {
		return "", false
	}

	plug, err := c.repo.FindByNamespaceAndSlug(context.Background(), namespace, slug)
	if err != nil {
		slog.Warn("mqtt: plug cache DB error", "namespace", namespace, "slug", slug, "err", err)
		return "", false
	}
	if plug == nil {
		return "", false
	}

	c.mu.Lock()
	c.m[key] = plug.ID
	c.mu.Unlock()
	return plug.ID, true
}

// Invalidate removes a cached entry so it will be re-loaded on next access.
func (c *PlugCache) Invalidate(namespace, slug string) {
	c.mu.Lock()
	delete(c.m, NamespaceSlug{namespace, slug})
	c.mu.Unlock()
}
