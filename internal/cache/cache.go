package cache

import "context"

// Cache 定义一个缓存接口
type Cache[K comparable, V any] interface {
	Set(ctx context.Context, key K, val V) (err error)
	Get(ctx context.Context, key K) (val V, ok bool, err error)
	// 返回的vals的len必须和keys一致，missIdxs表示没有命中缓存的索引
	MGet(ctx context.Context, keys []K) (vals []V, missIdxs []int, err error)
	Del(ctx context.Context, key K) (err error)
}
