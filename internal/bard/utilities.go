package bard

import (
	"crypto/md5"
	"encoding/hex"
	"time"
)

func HashConversation(conversation []string) string {
	hash := md5.New()
	for _, message := range conversation {
		hash.Write([]byte(message))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func GarbageCollectCache(cache *BardCache) {
	for k, v := range cache.Bards {
		if time.Since(v.LastInteractionTime) > time.Minute*5 {
			delete(cache.Bards, k)
		}
	}
}

func UpdateBardHash(old_hash, hash string) {
	if _, ok := cache.Bards[old_hash]; ok {
		cache.Bards[hash] = cache.Bards[old_hash]
		delete(cache.Bards, old_hash)
	}
}
