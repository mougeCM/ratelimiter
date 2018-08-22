package tkbucket

import (
	"time"
)

// Bucket interface for interacting with leaky buckets: https://en.wikipedia.org/wiki/Leaky_bucket
type Bucket interface {
	// Acquire get the token from the bucket
	// > 0 means that the token was obtained.
	Acquire(count int64) int64
	// TryAcquire try to acquire the token from the bucket
	// return the time it takes to wait for enough tokens
	TryAcquire(count int64) time.Duration
	// Wait try to acquire the token from the bucket
	// If you can't get it, wait automatically until you get it.
	Wait(count int64)
	// Available returns the number of available tokens.
	Available() int64
	// StartTime to get startTime
	StartTime() time.Time
	// Capacity of the bucket.
	Capacity() int64
	// acquire is the internal version - to enable easy testing.
	acquire(now time.Time, count int64) int64
	// tryAcquire is the internal version - to enable easy testing.
	tryAcquire(now time.Time, count int64, maxWait time.Duration) (time.Duration, bool)
	// available is the internal version - to enable easy testing.
	available(now time.Time) int64
}

// Storage interface for generating buckets keyed by a string.
type Storage interface {
	// Ping ensure service connection is valid
	Ping() error
	// Create a bucket with a name, fillInterval, capacity, and rate.
	Create(name string, fillInterval time.Duration, capacity int64) (Bucket, error)
	// CreateWithQuantum a bucket with a name, fillInterval, capacity, and quantum.
	CreateWithQuantum(name string, fillInterval time.Duration, capacity, quantum int64) (Bucket, error)
}
