package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gitit.cc/social/common/flog"
	"go.uber.org/zap"
)

// NewRedisCache 创建一个 redis cache
func NewRedisCache[K comparable, V any](prefix string, cli *redis.Client, ttlSeconds int64) Cache[K, V] {
	c := &redisCache[K, V]{prefix: prefix, cli: cli, ttl: time.Duration(ttlSeconds) * time.Second}
	c.marshaler = newMarshaler[V]()
	return c
}

type redisCache[K comparable, V any] struct {
	prefix string
	cli    *redis.Client
	ttl    time.Duration

	marshaler marshaler[V]
}

// Set set item
func (c *redisCache[K, V]) Set(ctx context.Context, key K, val V) (err error) {
	b, err := c.marshaler.marshal(val)
	if nil != err {
		flog.Error("marshal data fail", zap.Error(err))
		return
	}
	return c.cli.SetEx(ctx, c.key(key), b, c.ttl).Err()
}

// Get get item
func (c *redisCache[K, V]) Get(ctx context.Context, key K) (val V, ok bool, err error) {
	b, err := c.cli.Get(ctx, c.key(key)).Bytes()
	if err == nil {
		if len(b) > 0 {
			val, err = c.marshaler.unmarshal(b)
		}
		ok = err == nil
	} else if err == redis.Nil {
		err = nil
	}
	return
}
func (c *redisCache[K, V]) MGet(ctx context.Context, keys []K) (vals []V, missIdxs []int, err error) {
	sks := make([]string, len(keys))
	for i := range sks {
		sks[i] = c.key(keys[i])
	}

	vals = make([]V, len(keys))
	missIdxs = make([]int, 0, len(keys))
	bs, err := c.cli.MGet(ctx, sks...).Result()
	if nil != err {
		flog.Error("redis mget fail", zap.Error(err))
		return
	}

	for i := range bs {
		if bs[i] == nil {
			missIdxs = append(missIdxs, i)
		} else {
			vals[i], _ = c.marshaler.unmarshal([]byte(bs[i].(string)))
		}
	}

	return
}

// Del del item
func (c *redisCache[K, V]) Del(ctx context.Context, key K) (err error) {
	return c.cli.Del(ctx, c.key(key)).Err()
}

func (c *redisCache[K, V]) key(k K) string {
	return fmt.Sprintf("%s:%v", c.prefix, k)
}
