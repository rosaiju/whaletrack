package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestBucket_ImmediateTokens(t *testing.T) {
	// A new bucket starts full, so the first N requests should be instant.
	b := NewBucket(10, 5)

	start := time.Now()
	for i := 0; i < 5; i++ {
		if err := b.Wait(context.Background()); err != nil {
			t.Fatalf("Wait failed: %v", err)
		}
	}
	elapsed := time.Since(start)

	// 5 tokens should be consumed nearly instantly (< 50ms)
	if elapsed > 50*time.Millisecond {
		t.Errorf("5 immediate tokens took %v, expected < 50ms", elapsed)
	}
}

func TestBucket_RateLimiting(t *testing.T) {
	// After exhausting burst, should wait for refill.
	b := NewBucket(100, 1) // 100/sec rate, burst of 1

	// First request is instant (uses the 1 burst token)
	if err := b.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Second request must wait for refill (~10ms at 100/sec)
	start := time.Now()
	if err := b.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)

	if elapsed < 5*time.Millisecond {
		t.Errorf("expected rate-limited wait, got %v", elapsed)
	}
}

func TestBucket_ContextCancellation(t *testing.T) {
	// Should return error when context is cancelled.
	b := NewBucket(1, 0) // zero burst = always has to wait

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := b.Wait(ctx)
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestBucket_ConcurrentAccess(t *testing.T) {
	// Verify thread safety with concurrent access.
	b := NewBucket(1000, 100)

	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			b.Wait(context.Background())
			done <- struct{}{}
		}()
	}

	for i := 0; i < 50; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent requests")
		}
	}
}
