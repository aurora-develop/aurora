package bard

import "time"

type BardCache struct {
	Bards map[string]*Bard
}

var cache *BardCache

func init() {
	cache = &BardCache{
		Bards: make(map[string]*Bard),
	}
	go func() {
		for {
			GarbageCollectCache(cache)
			time.Sleep(time.Minute)
		}
	}()
}
