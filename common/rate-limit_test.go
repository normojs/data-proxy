package common

import "testing"

func TestInMemoryRateLimiterAllowsThenBlocks(t *testing.T) {
	var lim InMemoryRateLimiter
	lim.Init(0) // no janitor needed for short test

	key := "CT:127.0.0.1"
	// max 2 requests in 60s window
	if !lim.Request(key, 2, 60) {
		t.Fatal("first request should pass")
	}
	if !lim.Request(key, 2, 60) {
		t.Fatal("second request should pass")
	}
	if lim.Request(key, 2, 60) {
		t.Fatal("third request should be rate limited")
	}
}

func TestInMemoryRateLimiterIndependentKeys(t *testing.T) {
	var lim InMemoryRateLimiter
	lim.Init(0)
	if !lim.Request("A", 1, 60) {
		t.Fatal("A first")
	}
	if lim.Request("A", 1, 60) {
		t.Fatal("A second should block")
	}
	if !lim.Request("B", 1, 60) {
		t.Fatal("B should be independent")
	}
}
