package links

import (
	"time"
)

type cache struct {
	data map[string]cacheItem
	ttl  time.Duration
}

type cacheItem struct {
	entry   string
	expires time.Time
}

// NewCache instantiates and returns a new cache object
func NewCache() *cache {
	return &cache{
		data: map[string]cacheItem{},
		ttl:  86400 * time.Second,
	}
}

// Get returns a string and a bool for a given key. There are two possible
// combinations of values returned.
// The value and true when the key exists and has not been expired yet.
// An empty string and false when the key does not exist or has been expired.
func (c *cache) Get(key string) (string, bool) {
	value, ok := c.data[key]
	if value.expires.Sub(time.Now().UTC()) > 0 {
		return value.entry, ok
	}
	return "", false
}

// Store adds a new entry in the cache for a given key and value calculating the
// expires field accordingly to the default ttl.
func (c *cache) Store(key, value string) {
	c.data[key] = cacheItem{entry: value, expires: time.Now().UTC().Add(c.ttl)}
}

// SetTTL updates the default TTL value for the cache.
func (c *cache) SetTTL(n int) {
	c.ttl = time.Duration(n) * time.Second
}
