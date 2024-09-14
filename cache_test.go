package scache_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gitit.cc/social/common/scache"
)

type SayHelloRes struct {
	Hi string
}

var (
	mLRUCacheSayHello        *scache.GCache[int64, *SayHelloRes]
	mLRUCacheSayHelloNoEmpty *scache.GCache[int64, *SayHelloRes]

	mRedisCacheSayHello        *scache.GCache[int64, *SayHelloRes]
	mRedisCacheSayHelloNoEmpty *scache.GCache[int64, *SayHelloRes]

	mStringCache *scache.GCache[string, string]
	mBytesCache  *scache.GCache[string, []byte]

	mAgeTTL func()
)

func fallbackOne(ctx context.Context, uid int64) (*SayHelloRes, error) {
	if uid > 10 {
		return &SayHelloRes{Hi: fmt.Sprintf("hello %d", uid)}, nil
	}
	return nil, nil
}

func fallbackSlice(ctx context.Context, uids []int64) ([]*SayHelloRes, error) {
	ret := make([]*SayHelloRes, len(uids))
	for i, u := range uids {
		if u > 10 {
			ret[i] = &SayHelloRes{Hi: fmt.Sprintf("hello %d", u)}
		} else {
			ret[i] = nil
		}
	}
	return ret, nil
}

func fallbackMap(ctx context.Context, uids []int64) (map[int64]*SayHelloRes, error) {
	ret := make(map[int64]*SayHelloRes, len(uids))
	for _, u := range uids {
		if u > 10 {
			ret[u] = &SayHelloRes{Hi: fmt.Sprintf("hello %d", u)}
		} else {
			ret[u] = nil
		}
	}
	return ret, nil
}

func strFallbackOne(_ context.Context, s string) (string, error) {
	return "str:" + s, nil
}

func bytesFallbackOne(_ context.Context, s string) ([]byte, error) {
	return []byte("bytes:" + s), nil
}

func TestLURInit(t *testing.T) {
	mLRUCacheSayHello = scache.NewLRUCache[int64, *SayHelloRes](5, 1, scache.CacheEmpty(true))
	mLRUCacheSayHelloNoEmpty = scache.NewLRUCache[int64, *SayHelloRes](5, 1)
}

func TestRedisInit(t *testing.T) {
	const useMini = true
	const ttl = time.Second
	const age = ttl + time.Second

	var cli *redis.Client

	if useMini {
		svr, _ := miniredis.Run()
		cli = redis.NewClient(&redis.Options{Addr: svr.Addr()})
		mAgeTTL = func() { svr.FastForward(age) }
	} else {
		cli = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    "redis-test",
			SentinelAddrs: []string{"192.168.61.231:26379"},
		})
		cli.FlushDB(context.Background())
		mAgeTTL = func() { time.Sleep(age) }
	}

	mRedisCacheSayHello = scache.NewRedisCache[int64, *SayHelloRes]("hello", cli, 1, scache.CacheEmpty(true))
	mRedisCacheSayHelloNoEmpty = scache.NewRedisCache[int64, *SayHelloRes]("hello-noempt", cli, 1)

	mStringCache = scache.NewRedisCache[string, string]("string", cli, 1)
	mBytesCache = scache.NewRedisCache[string, []byte]("bytes", cli, 1)
}

func testCache(c *scache.GCache[int64, *SayHelloRes], t *testing.T) {
	ret, err := c.One(context.Background(), 123, fallbackOne)
	fmt.Println(ret, err)

	ret, err = c.One(context.Background(), 124, fallbackOne)
	fmt.Println(ret, err)

	ret, err = c.One(context.Background(), 123, fallbackOne)
	fmt.Println(ret, err)

	ret, err = c.One(context.Background(), 3, fallbackOne)
	fmt.Println(ret, err)

	ret, err = c.One(context.Background(), 3, fallbackOne)
	fmt.Println(ret, err)

	rets, err := c.Slice(context.Background(), []int64{123, 124}, fallbackSlice)
	fmt.Println(rets, err)

	rets, err = c.Slice(context.Background(), []int64{123, 124, 125}, fallbackSlice)
	fmt.Println(rets, err)

	rets, err = c.Slice(context.Background(), []int64{223, 224, 225}, fallbackSlice)
	fmt.Println(rets, err)

	retm, err := c.Map(context.Background(), []int64{223, 224, 225}, fallbackMap)
	fmt.Println(retm, err)

	retm, err = c.Map(context.Background(), []int64{323, 324, 325}, fallbackMap)
	fmt.Println(retm, err)

	retm, err = c.Map(context.Background(), []int64{223, 224, 425}, fallbackMap)
	fmt.Println(retm, err)

	// test log
	for i := 0; i < 0xfff; i++ {
		_, _ = c.One(context.Background(), 30, fallbackOne)
		_, _ = c.Slice(context.Background(), []int64{123, 124, 125}, fallbackSlice)
		_, _ = c.Map(context.Background(), []int64{223, 224, 425}, fallbackMap)
	}
}

func TestLRU(t *testing.T) {
	testCache(mLRUCacheSayHello, t)
	testCache(mLRUCacheSayHelloNoEmpty, t)
}
func TestRedis(t *testing.T) {
	testCache(mRedisCacheSayHello, t)
	testCache(mRedisCacheSayHelloNoEmpty, t)

	fmt.Println("~~~~~~~~~~~ age ttl ~~~~~~~~~~~~~~~`~")
	mAgeTTL()
	testCache(mRedisCacheSayHello, t)
	testCache(mRedisCacheSayHelloNoEmpty, t)

	ctx := context.Background()
	fmt.Println("~~~~~~~~~~~ str bytes ~~~~~~~~~~~~~~~`~")
	ret, err := mStringCache.One(ctx, "hello", strFallbackOne)
	fmt.Println(ret, err)
	ret, err = mStringCache.One(ctx, "hello", strFallbackOne)
	fmt.Println(ret, err)

	retb, err := mBytesCache.One(ctx, "hello", bytesFallbackOne)
	fmt.Println(retb, err)
	retb, err = mBytesCache.One(ctx, "hello", bytesFallbackOne)
	fmt.Println(retb, err)
}
