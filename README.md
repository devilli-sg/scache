# scache
通用缓存库，支持本地缓存和redis缓存

## Features
- 目前支持泛型接口，也就是必须指定key和value的类型
- 本地缓存会直接存对象，如果用的是指针务必保证不要修改缓存内容。本地缓存目前支持：
  - LRU缓存
- 非本地缓存会对对象进行序列化后保存。
  - protobuf对象会用proto来序列化和反序列化
  - 其他对象会用json进行序列化和反序列化

## 使用方法
- 可以参考[example](example/main.go) 或者 [test](cache_test.go)