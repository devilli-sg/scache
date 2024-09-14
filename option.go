package scache

// Options cache的一些选项
type Options struct {
	cacheEmpty bool // 空值也cache
}

// Option  use to init Options
type Option func(opts *Options)

// CacheEmpty 空值也存储
func CacheEmpty(yes bool) Option {
	return func(opts *Options) {
		opts.cacheEmpty = yes
	}
}
