// Package ratelimit implements a token bucket rate limiter.
//
// SEC EDGAR enforces a 10 requests/second limit. The token bucket algorithm
// maintains a fixed-capacity bucket of tokens that refills at a steady rate.
// Each request consumes one token; if the bucket is empty, the caller blocks
// until a token becomes available.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Bucket is a thread-safe token bucket rate limiter.
type Bucket struct {
	capacity   float64
	tokens     float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewBucket creates a rate limiter that allows `rate` operations per second
// with a burst capacity of `burst`.
func NewBucket(rate float64, burst int) *Bucket {
	return &Bucket{
		capacity:   float64(burst),
		tokens:     float64(burst), // start full
		refillRate: rate,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available or the context is cancelled.
func (b *Bucket) Wait(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		b.mu.Lock()
		b.refill()
		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}
		// Calculate how long until the next token arrives
		deficit := 1.0 - b.tokens
		waitDur := time.Duration(deficit / b.refillRate * float64(time.Second))
		b.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDur):
			// loop back and try to acquire
		}
	}
}

// refill adds tokens based on elapsed time. Must be called with mu held.
func (b *Bucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.lastRefill = now
}
