package proxy

import (
	"testing"
)

func TestPoolAllocateAndRelease(t *testing.T) {
	p := NewPool([]string{"http://proxy1:8080", "http://proxy2:8080", "http://proxy3:8080"})

	ip1 := p.Allocate()
	ip2 := p.Allocate()
	ip3 := p.Allocate()
	ip4 := p.Allocate()

	if ip1 == ip2 || ip2 == ip3 {
		t.Errorf("round-robin should cycle: got %q, %q, %q, %q", ip1, ip2, ip3, ip4)
	}
	if ip1 != ip4 {
		t.Errorf("4th allocate should be same as 1st: got %q, want %q", ip4, ip1)
	}
}

func TestPoolEmpty(t *testing.T) {
	p := NewPool(nil)
	ip := p.Allocate()
	if ip != "" {
		t.Errorf("empty pool should return empty string, got %q", ip)
	}
}

func TestPoolCount(t *testing.T) {
	p := NewPool([]string{"a", "b", "c"})
	if p.Count() != 3 {
		t.Errorf("Count = %d, want 3", p.Count())
	}
}
