package handler

import (
	"testing"
	"time"
)

func TestCacheTrackerFirstSeenIsCreation(t *testing.T) {
	ct := &cacheTracker{seen: make(map[string]time.Time), ttl: 5 * time.Minute}
	written, cached := ct.record("conv1", "system instructions", "user input")
	if written == 0 {
		t.Fatalf("expected cache_write_tokens > 0 on first sight, got %d", written)
	}
	if cached != 0 {
		t.Fatalf("expected cached_tokens = 0 on first sight, got %d", cached)
	}
}

func TestCacheTrackerSecondSeenIsRead(t *testing.T) {
	ct := &cacheTracker{seen: make(map[string]time.Time), ttl: 5 * time.Minute}
	ct.record("conv1", "system instructions", "user input")
	written, cached := ct.record("conv1", "system instructions", "user input")
	if cached == 0 {
		t.Fatalf("expected cached_tokens > 0 on reuse, got %d", cached)
	}
	if written != 0 {
		t.Fatalf("expected cache_write_tokens = 0 on reuse, got %d", written)
	}
}

func TestCacheTrackerEmptyInputIsNoop(t *testing.T) {
	ct := &cacheTracker{seen: make(map[string]time.Time), ttl: 5 * time.Minute}
	written, cached := ct.record("conv1", "", "")
	if cached != 0 || written != 0 {
		t.Fatalf("expected zero cache for empty input, got cached=%d written=%d", cached, written)
	}
}
