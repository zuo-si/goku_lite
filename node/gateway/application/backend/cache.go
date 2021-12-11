package backend

import (
	"time"

	"github.com/patrickmn/go-cache"
)

var (
	c                      *cache.Cache
	DefaultExpiration      = 5 * time.Minute
	DefaultCleanupInterval = 10 * time.Minute
)

func init() {
	c = cache.New(DefaultExpiration, DefaultCleanupInterval)
}

func getCache(key string) (val interface{}, found bool) {
	return c.Get(key)
}

func setCache(key string, val interface{}) {
	c.Set(key, val, DefaultExpiration)
}
