package links

type CacheStore interface {
	GetData(key string) string
}

type CacheServer struct {
	Data CacheStore
}

func (c *CacheServer) GetCache(key string) string {
	return c.Data.GetData(key)
}
