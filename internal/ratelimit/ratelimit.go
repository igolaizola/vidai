package ratelimit

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

type Lock interface {
	Lock(ctx context.Context) func()
	LockWithDuration(ctx context.Context, d time.Duration) func()
}

type lock struct {
	lck      *sync.Mutex
	duration time.Duration
}

// New creates a new rate limit lock.
func New(d time.Duration) Lock {
	return &lock{
		lck:      &sync.Mutex{},
		duration: d,
	}
}

// Lock locks the rate limit for the given duration and returns a function that
// unlocks the rate limit with a delay time based on the given duration.
func (l *lock) LockWithDuration(ctx context.Context, d time.Duration) func() {
	l.lck.Lock()
	// Apply a factor between 0.85 and 1.15 to the duration
	d = time.Duration(float64(d) * (0.85 + rand.Float64()*0.3))
	return func() {
		defer l.lck.Unlock()
		select {
		case <-ctx.Done():
		case <-time.After(d):
		}
	}
}

// Lock locks the rate limit for the default duration and returns a function that
// unlocks the rate limit with a delay time based on the default duration.
func (l *lock) Lock(ctx context.Context) func() {
	return l.LockWithDuration(ctx, l.duration)
}
