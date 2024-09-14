package scache

import (
	"context"
	"reflect"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
	"gitit.cc/social/common/flog"
	"gitit.cc/social/common/scache/internal/cache"
	"go.uber.org/zap"
)

const (
	cStepMask = 0xff
)

// GCache generic cache，泛型cache，也就是key和value的类型是固定的
type GCache[K comparable, V any] struct {
	c cache.Cache[K, V]

	opts Options

	// 统计信息，这里每次get要操作total和hit，不保证原子性，只是为了log，不做太复杂。
	// slice和map共用一份数据，可能会导致有些log不出来。因为只是统计，所以不做复杂处理
	oneTotal   uint64
	oneHit     uint64
	batchTotal uint64
	batchHit   uint64
	batchCount uint64 // batchGet的次数，每cStepMask次打印一次log
}

// NewLRUCache 创建一个存在本地的LRU cache，直接存对象，不做marshal。
// 对象用指针的方式存储，注意使用者不要对对象进行修改
func NewLRUCache[K comparable, V any](maxItem int, ttlSeconds int64, opts ...Option) *GCache[K, V] {
	c := &GCache[K, V]{c: cache.NewLRUCache[K, V](maxItem, ttlSeconds)}
	for _, o := range opts {
		o(&c.opts)
	}

	return c
}

// NewRedisCache 创建一个存在redis的cache，对象会经过序列化，如果是pb格式用pb
// 序列化，否则用json序列化
func NewRedisCache[K comparable, V any](prefix string, cli *redis.Client, ttlSeconds int64, opts ...Option) *GCache[K, V] {
	c := &GCache[K, V]{c: cache.NewRedisCache[K, V](prefix, cli, ttlSeconds)}
	for _, o := range opts {
		o(&c.opts)
	}
	return c
}

// One 获取一个对象
func (c *GCache[K, V]) One(ctx context.Context, key K, fallback func(ctx context.Context, key K) (V, error)) (V, error) {
	v, ok, err := c.c.Get(ctx, key)
	total := atomic.AddUint64(&c.oneTotal, 1)
	var hit uint64
	if ok {
		hit = 1
	}
	hit = atomic.AddUint64(&c.oneHit, hit)
	if total&cStepMask == 0 {
		flog.Info("scache one hit", zap.Uint64("total", total), zap.Uint64("hit", hit))
	}
	if fallback == nil || ok {
		return v, err
	}

	if nil != err {
		flog.Error("scache get one fail", zap.Any("key", key), zap.Error(err))
		return v, err // TODO 这个策略要想想
	}

	v, err = fallback(ctx, key)
	if nil == err && (notEmpty(v) || c.opts.cacheEmpty) {
		err = c.c.Set(ctx, key, v)
	}
	return v, err
}

// Add 直接添加缓存
func (c *GCache[K, V]) Add(ctx context.Context, key K, val V) error {
	return c.c.Set(ctx, key, val)
}

// Slice 回源函数返回值是slice的场景
// 回退函数 fallback 的 keys 参数表示未命中缓存的key数组,
// 需要非常注意的是回退函数返回值数组的顺序必须和keys参数的顺序对应且数量一致,
// 某个key的确不存在时也需要在返回值对应的位置插入 nil, 否则缓存将会错乱.
// 建议使用 Map 方法来做批量数据的缓存, 这样处理起来比较简单方便和不容易出错.
func (c *GCache[K, V]) Slice(ctx context.Context, keys []K, fallback func(ctx context.Context, keys []K) ([]V, error)) ([]V, error) {
	vs, missIdxs, err := c.c.MGet(ctx, keys)

	total := atomic.AddUint64(&c.batchTotal, uint64(len(keys)))
	hit := atomic.AddUint64(&c.batchHit, uint64(len(keys)-len(missIdxs)))
	// TODO 要加数据上报

	if atomic.AddUint64(&c.batchCount, 1)&cStepMask == 0 {
		flog.Info("scache slice hit", zap.Uint64("total", total), zap.Uint64("hit", hit))
	}

	var miss []K
	if nil == err {
		if len(missIdxs) == 0 {
			return vs, nil
		}
		if len(missIdxs) < len(keys) {
			miss = make([]K, len(missIdxs))
			for i := range miss {
				miss[i] = keys[missIdxs[i]]
			}
		} else {
			miss = keys
		}
	}

	// 未命中缓存的，需要回源
	fbs, err := fallback(ctx, miss)
	if nil == err {
		for i := range fbs {
			vs[missIdxs[i]] = fbs[i]
			if notEmpty(fbs[i]) || c.opts.cacheEmpty {
				_ = c.c.Set(ctx, miss[i], fbs[i])
			}
		}
		return vs, nil
	}
	return nil, err // todo 部分失败，要返回吗？
}

// Map 回源函数返回值是map的场景。这个函数和slice很像，但是也不做公共提取了，不然代码可读性太差
func (c *GCache[K, V]) Map(ctx context.Context, keys []K, fallback func(ctx context.Context, keys []K) (map[K]V, error)) (map[K]V, error) {
	vs, missIdxs, err := c.c.MGet(ctx, keys)

	total := atomic.AddUint64(&c.batchTotal, uint64(len(keys)))
	hit := atomic.AddUint64(&c.batchHit, uint64(len(keys)-len(missIdxs)))
	// TODO 要加数据上报

	if atomic.AddUint64(&c.batchCount, 1)&cStepMask == 0 {
		flog.Info("scache map hit", zap.Uint64("total", total), zap.Uint64("hit", hit))
	}
	var miss []K
	if nil == err {
		if len(missIdxs) == 0 {
			return sliceToMap(keys, vs), nil
		}
		if len(missIdxs) < len(keys) {
			miss = make([]K, len(missIdxs))
			for i := range miss {
				miss[i] = keys[missIdxs[i]]
			}
		} else {
			miss = keys
		}
	}

	// 未命中缓存的，需要回源
	fbs, err := fallback(ctx, miss)
	if nil == err {
		for k, v := range fbs {
			if notEmpty(v) || c.opts.cacheEmpty {
				_ = c.c.Set(ctx, k, v)
			}
		}
		if len(missIdxs) < len(keys) {
			j := 0
			for i := range keys { // 这段逻辑有点晦涩，missIdxs存的是没有命中的索引下标
				if j >= len(missIdxs) || i != missIdxs[j] {
					fbs[keys[i]] = vs[i]
				} else {
					j++
				}
			}
		}
		return fbs, nil
	}
	return nil, err // todo 部分失败，要返回吗？
}

// Del 删除缓存
func (c *GCache[K, V]) Del(ctx context.Context, keys ...K) {
	for _, k := range keys {
		_ = c.c.Del(ctx, k)
	}
}

// 判断一个值是否是nil，用到了反射，暂时没有更好的方法
func notEmpty(v any) bool {
	vl := reflect.ValueOf(v)
	return vl.Kind() != reflect.Pointer || !vl.IsNil()
}

func sliceToMap[K comparable, V any](keys []K, vals []V) map[K]V {
	m := make(map[K]V, len(keys))
	for i := range keys {
		m[keys[i]] = vals[i]
	}
	return m
}
