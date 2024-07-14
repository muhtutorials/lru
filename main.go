package main

import (
	"fmt"
	"lru/expirable_lru"
	"time"
)

func evictCallback(key int, value any) {
	fmt.Println(key)
}

func main() {
	l, _ := NewWithOnEvict[int, any](4, evictCallback)
	for i := 0; i < 8; i++ {
		l.Add(i, nil)
	}
	if l.Len() != 4 {
		panic(fmt.Sprintf("bad len: %v", l.Len()))
	}

	// make cache with 5 max keys and 10ms TTL
	cache := expirable_lru.NewLRU[string, string](5, nil, time.Millisecond*10)

	cache.Add("key_1", "value_1")
	k, ok := cache.Get("key_1")
	if ok {
		fmt.Printf("value before expiration is found: %v, value: %q\n", ok, k)
	}

	// wait for cache to expire
	time.Sleep(time.Millisecond * 12)

	k, ok = cache.Get("key_1")
	fmt.Printf("value before expiration is found: %v, value: %q\n", ok, k)

	// set value under key_2, would evict old entry because it is already expired.
	cache.Add("key_2", "value_2")

	fmt.Printf("cache len: %d\n", cache.Len())
}
