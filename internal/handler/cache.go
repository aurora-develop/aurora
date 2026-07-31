package handler

import (
	"strings"
	"sync"
	"time"
)

// cacheTracker simulates prompt-caching metrics per conversation. ChatGPT
// Web does not expose real caching, so we fingerprint text blocks and report
// "cache_write" on first sight and "cached" on reuse. Design ported from
// D:/Go/claude2api/handlers/cache.go.
type cacheTracker struct {
	mu   sync.Mutex
	seen map[string]time.Time // fingerprint -> first seen
	ttl  time.Duration
}

var globalCacheTracker = &cacheTracker{
	seen: make(map[string]time.Time),
	ttl:  5 * time.Minute,
}

// estTokens returns a rough token count (~4 chars/token).
func estTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	n := len(text) / 4
	if n < 1 {
		n = 1
	}
	return n
}

// cacheFP derives a stable key for a content block.
func cacheFP(part string) string {
	return part
}

// record processes the request text blocks and returns (cacheWriteTokens, cachedTokens).
// Return order is creation-first so the public RecordCache matches the
// test expectations: first sight yields cache_write > 0, reuse yields cached > 0.
func (t *cacheTracker) record(conversationID, instructions, input string) (cacheWriteTokens, cachedTokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.gc()

	type block struct {
		fp     string
		tokens int
	}
	var blocks []block
	if instructions != "" {
		blocks = append(blocks, block{cacheFP("instructions:" + instructions), estTokens(instructions)})
	}
	if input != "" {
		blocks = append(blocks, block{cacheFP("input:" + input), estTokens(input)})
	}

	for _, b := range blocks {
		if b.tokens == 0 {
			continue
		}
		if _, ok := t.seen[b.fp]; ok {
			cachedTokens += b.tokens
		} else {
			cacheWriteTokens += b.tokens
			t.seen[b.fp] = time.Now()
		}
	}
	return
}

// gc evicts expired fingerprints.
func (t *cacheTracker) gc() {
	now := time.Now()
	for fp, seen := range t.seen {
		if now.Sub(seen) > t.ttl {
			delete(t.seen, fp)
		}
	}
}

// RecordCache is the package-level entry point.
// Returns (cacheWriteTokens, cachedTokens) — creation first, matching the
// claude2api convention and the cacheTracker.record implementation.
func RecordCache(conversationID, instructions, input string) (cacheWriteTokens, cachedTokens int) {
	return globalCacheTracker.record(conversationID, instructions, input)
}
