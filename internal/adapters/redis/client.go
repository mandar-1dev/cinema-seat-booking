package redis

import "github.com/redis/go-redis/v9"

// NewClient wraps go-redis so nothing outside this package needs to
// import "github.com/redis/go-redis/v9" directly. If you ever swap Redis
// for something else, this is the only file that changes.
func NewClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}
