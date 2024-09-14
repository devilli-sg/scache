// Package main
//
//nolint:all
package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"gitit.cc/social/common/scache"
)

type SayHelloRes struct {
	Hi string
}

var (
	// 泛型cache，指定key和value的类型，value的类型建议用指针（但是如果是LRUcache需要小心使用者别修改了缓存）
	mLRUCacheSayHello   *scache.GCache[int64, *SayHelloRes]
	mRedisCacheSayHello *scache.GCache[int64, *SayHelloRes]
)

// 建议的使用方法是封装对应的Get函数，传入回源函数

// 单个Get，可写可不写，可以只写BatchGet，然后用 []int64{uid} 传参
func sayHelloGet(ctx context.Context, uid int64) (*SayHelloRes, error) {
	return mLRUCacheSayHello.One(ctx, uid, func(ctx context.Context, uid int64) (*SayHelloRes, error) {
		// TODO 编写实际的回源函数
		return &SayHelloRes{Hi: fmt.Sprintf("hello %d", uid)}, nil
	})
}

// 批量获取
func sayHelloBatchGet(ctx context.Context, uids []int64) ([]*SayHelloRes, error) {
	// 这里用Slice还是Map看回源函数的类型
	return mLRUCacheSayHello.Slice(ctx, uids, func(ctx context.Context, uids []int64) ([]*SayHelloRes, error) {
		// TODO 编写实际的回源函数
		ret := make([]*SayHelloRes, len(uids))
		for i, u := range uids {
			ret[i] = &SayHelloRes{Hi: fmt.Sprintf("hello %d", u)}
		}
		return ret, nil
	})
}

func initCache() {
	mLRUCacheSayHello = scache.NewLRUCache[int64, *SayHelloRes](5, 3600, scache.CacheEmpty(true))

	cli := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    "redis-test",
		SentinelAddrs: []string{"192.168.61.231:26379"},
	})
	mRedisCacheSayHello = scache.NewRedisCache[int64, *SayHelloRes]("hello", cli, 1, scache.CacheEmpty(true))
}

func main() {
	initCache()
	ret, err := sayHelloGet(context.Background(), 123)
	fmt.Println(ret, err)

	ret, err = sayHelloGet(context.Background(), 124)
	fmt.Println(ret, err)

	rets, err := sayHelloBatchGet(context.Background(), []int64{123, 124})
	fmt.Println(rets, err)

	_ = mRedisCacheSayHello
}
