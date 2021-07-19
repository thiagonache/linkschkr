package links

type Cache struct {
	data map[string]string
}

func NewCache() *Cache {
	return &Cache{
		data: map[string]string{},
	}
}

func (c *Cache) Get(key string) (string, bool) {
	value, ok := c.data[key]
	return value, ok
}

func (c *Cache) Store(key, value string) {
	c.data[key] = value
}
