package cache

import (
	"context"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
)

// NewLRUCache 创建一个lru cache
func NewLRUCache[K comparable, V any](maxEntrys int, ttlSeconds int64) Cache[K, V] {
	return &lruCache[K, V]{Cache: lru.New(maxEntrys), ttlSeconds: ttlSeconds}
}

type item[T any] struct {
	v      T
	uptime int64
}

type lruCache[K comparable, T any] struct {
	lock sync.Mutex
	*lru.Cache
	ttlSeconds int64
}

// Set 增加或者修改一个item
func (c *lruCache[K, V]) Set(_ context.Context, key K, val V) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.Cache.Add(key, item[V]{v: val, uptime: time.Now().Unix()})
	return nil
}

// Get 读取Item
func (c *lruCache[K, V]) Get(_ context.Context, key K) (val V, ok bool, err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	vf, ok := c.Cache.Get(key)
	if vf != nil {
		it := vf.(item[V])
		if it.uptime+c.ttlSeconds >= time.Now().Unix() {
			val = it.v
		} else {
			ok = false
			c.Cache.Remove(key)
		}
	}
	return
}

// MGet 读取多个item
func (c *lruCache[K, V]) MGet(ctx context.Context, keys []K) (vals []V, missIdxs []int, err error) {
	vals = make([]V, len(keys))
	missIdxs = make([]int, 0, len(keys))
	var ok bool
	for i := range keys {
		vals[i], ok, _ = c.Get(ctx, keys[i])
		if !ok {
			missIdxs = append(missIdxs, i)
		}
	}
	return
}

// Del 删除item
func (c *lruCache[K, T]) Del(_ context.Context, key K) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.Cache.Remove(key)
	return nil
}
