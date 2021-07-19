package links

import (
	"time"
)

type Cache struct {
	data map[string]CacheItem
	ttl  time.Duration
}

type CacheItem struct {
	entry   string
	expires time.Time
}

func NewCache() *Cache {
	return &Cache{
		data: map[string]CacheItem{},
		ttl:  86400 * time.Second,
	}
}

func (c *Cache) Get(key string) (string, bool) {
	value, ok := c.data[key]
	if value.expires.Sub(time.Now().UTC()) > 0 {
		return value.entry, ok
	}
	return "", false
}

func (c *Cache) Store(key, value string) {
	c.data[key] = CacheItem{entry: value, expires: time.Now().UTC().Add(c.ttl)}
}

func (c *Cache) SetTTL(n int) {
	c.ttl = time.Duration(n) * time.Second
}
