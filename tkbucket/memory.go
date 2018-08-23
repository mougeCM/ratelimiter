package tkbucket

import (
	"sync"
	"time"
)

const infinityDuration time.Duration = 0x7fffffffffffffff

type memoryBucket struct {
	// startTime holds the moment when the bucket was
	// first created and ticks began.
	startTime time.Time
	// capacity holds the overall capacity of the bucket.
	capacity int64
	// quantum holds how many tokens are added on each tick.
	quantum int64
	// fillInterval holds the interval between each tick.
	fillInterval time.Duration
	// mu guards the fields below it.
	mu sync.Mutex
	// avail holds the number of available
	// tokens as of the associated latestTick.
	avail int64
	// latestTick holds the latest tick for which we know
	// the number of tokens in the bucket.
	latestTick int64
}

func (b *memoryBucket) StartTime() time.Time {
	return b.startTime
}

func (b *memoryBucket) Capacity() int64 {
	return b.capacity
}

// Acquire takes up to count immediately available tokens from the bucket
// result > 0，sufficient token
func (b *memoryBucket) Acquire(count int64) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.acquire(time.Now(), count)
}

// TryAcquire try to acquire the token from the bucket
func (b *memoryBucket) TryAcquire(count int64) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	d, _ := b.tryAcquire(time.Now(), count, infinityDuration)
	return d
}

// Wait try to acquire the token from the bucket and wait util to get it
func (b *memoryBucket) Wait(count int64) {
	if d := b.TryAcquire(count); d > 0 {
		time.Sleep(d)
	}
}

// Available returns the number of available tokens.
func (b *memoryBucket) Available() int64 {
	return b.available(time.Now())
}

// acquire is the internal version of TakeAvailable - it takes the
// current time as an argument to enable easy testing.
func (b *memoryBucket) acquire(now time.Time, count int64) int64 {
	if count <= 0 {
		return 0
	}
	b.adjustAvail(b.currentTick(now))
	if b.avail <= 0 {
		return 0
	}
	if count > b.avail {
		count = b.avail
	}
	b.avail -= count
	return count
}

// tryAcquire is the internal version of Take - it takes the current time as
// an argument to enable easy testing.
func (b *memoryBucket) tryAcquire(now time.Time, count int64, maxWait time.Duration) (time.Duration, bool) {
	if count <= 0 {
		return 0, true
	}

	tick := b.currentTick(now)
	b.adjustAvail(tick)
	avail := b.avail - count
	if avail >= 0 {
		b.avail = avail
		return 0, true
	}

	// endTick holds the tick when all the requested tokens will
	// become available.
	endTick := tick + (-avail+b.quantum-1)/b.quantum
	endTime := b.startTime.Add(time.Duration(endTick) * b.fillInterval)
	waitTime := endTime.Sub(now)
	if waitTime > maxWait {
		return 0, false
	}
	// consider multiple requests waiting at the same time
	b.avail = avail
	return waitTime, true
}

// available is the internal version of available - it takes the current time as
// an argument to enable easy testing.
func (b *memoryBucket) available(now time.Time) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.adjustAvail(b.currentTick(now))
	return b.avail
}

// currentTick returns the current time tick, measured
// from b.startTime.
func (b *memoryBucket) currentTick(now time.Time) int64 {
	return int64(now.Sub(b.startTime) / b.fillInterval)
}

// adjustAvail adjusts the current number of tokens
// available in the memoryBucket at the given time, which must
// be in the future (positive) with respect to b.latestTick.
func (b *memoryBucket) adjustAvail(tick int64) {
	if b.avail >= b.capacity {
		return
	}
	b.avail += (tick - b.latestTick) * b.quantum
	if b.avail > b.capacity {
		b.avail = b.capacity
	}
	b.latestTick = tick
}

// MemoryStorage is a memoryBucket factory.
type MemoryStorage struct {
	buckets map[string]*memoryBucket
}

// NewMemoryStorage initializes the in-memory memoryBucket store.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		buckets: make(map[string]*memoryBucket),
	}
}

func (s *MemoryStorage) Ping() error { return nil }

// Create create a memoryBucket.
func (s *MemoryStorage) Create(name string, fillInterval time.Duration, capacity int64) (Bucket, error) {
	b, ok := s.buckets[name]
	if ok {
		return b, nil
	}
	b = create(name, fillInterval, capacity, 1)
	s.buckets[name] = b
	return b, nil
}

// CreateWithQuantum create a memoryBucket with quantum.
func (s *MemoryStorage) CreateWithQuantum(name string, fillInterval time.Duration, capacity, quantum int64) (Bucket, error) {
	b, ok := s.buckets[name]
	if ok {
		return b, nil
	}
	b = create(name, fillInterval, capacity, quantum)
	s.buckets[name] = b
	return b, nil
}

func create(name string, fillInterval time.Duration, capacity, quantum int64) *memoryBucket {
	if fillInterval <= 0 {
		panic("token bucket fill interval is not > 0")
	}
	if capacity <= 0 {
		panic("token bucket capacity is not > 0")
	}
	if quantum <= 0 {
		panic("token bucket quantum is not > 0")
	}
	return &memoryBucket{
		startTime:    time.Now(),
		latestTick:   0,
		fillInterval: fillInterval,
		capacity:     capacity,
		quantum:      quantum,
		avail:        capacity,
	}
}
