package tkbucket

import (
	"strconv"
	"time"

	"log"

	"github.com/go-redis/redis"
)

const (
	startTimeField    = "start_time"
	fillIntervalField = "fill_interval"
	capacityField     = "capacity"
	quantumField      = "quantum"
	availField        = "avail"
	latestTickField   = "latest_tick"
)

type redisBucket struct {
	Key    string
	Client *redis.Client
}

func (r *redisBucket) StartTime() time.Time {
	st, _ := r.Client.HGet(r.Key, startTimeField).Int64()
	return time.Unix(0, st)
}

func (r *redisBucket) Capacity() int64 {
	c, _ := r.Client.HGet(r.Key, capacityField).Int64()
	return c
}

// Acquire takes up to count immediately available tokens from the bucket
// result > 0，sufficient token
func (r *redisBucket) Acquire(count int64) int64 {
	return r.acquire(time.Now(), count)
}

// TryAcquire try to acquire the token from the bucket
func (r *redisBucket) TryAcquire(count int64) time.Duration {
	d, _ := r.tryAcquire(time.Now(), count, infinityDuration)
	return d
}

func (r *redisBucket) Wait(count int64) {
	if d := r.TryAcquire(count); d > 0 {
		time.Sleep(d)
	}
}

// Available returns the number of available tokens.
func (r *redisBucket) Available() int64 {
	return r.available(time.Now())
}

// acquire is the internal version of TakeAvailable - it takes the
// current time as an argument to enable easy testing.
func (r *redisBucket) acquire(now time.Time, count int64) int64 {
	if count <= 0 {
		return 0
	}

	// Execute lua script
	res, err := r.Client.Eval(
		luaAcquire,
		[]string{r.Key},
		strconv.FormatInt(now.UnixNano(), 10),
		count,
	).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("Eval luaAcquire: %v\n", err)
		}
		return 0
	}

	return res.(int64)
}

func (r *redisBucket) tryAcquire(now time.Time, count int64, maxWait time.Duration) (time.Duration, bool) {
	if count <= 0 {
		return 0, true
	}

	// Execute lua script
	res, err := r.Client.Eval(
		luaTryAcquire,
		[]string{r.Key},
		strconv.FormatInt(now.UnixNano(), 10),
		count,
	).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("Eval luaTryAcquire: %v\n", err)
			return 0, false
		} else {
			return 0, true
		}
	}

	// token is enough
	if res.(int64) == 0 {
		return 0, true
	}

	waitTime := time.Duration(res.(int64)-now.UnixNano()) * time.Nanosecond
	if waitTime > maxWait {
		return 0, false
	}
	return waitTime, true
}

// available is the internal version of available - it takes the current time as
// an argument to enable easy testing.
func (r *redisBucket) available(now time.Time) int64 {
	// Execute lua script
	res, err := r.Client.Eval(
		luaAvailable,
		[]string{r.Key},
		strconv.FormatInt(now.UnixNano(), 10),
	).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("Eval luaAvailable: %v\n", err)
		}
		return 0
	}

	return res.(int64)
}

// currentTick returns the current time tick, measured
// from b.startTime.
func (r *redisBucket) currentTick(now time.Time, bucketInfo map[string]string) int64 {
	startTime, _ := strconv.Atoi(bucketInfo[startTimeField])
	fillInterval, _ := strconv.Atoi(bucketInfo[fillIntervalField])

	return (now.UnixNano() - int64(startTime)) / int64(fillInterval)
}

// adjustAvail adjusts the current number of tokens
// available in the memoryBucket at the given time, which must
// be in the future (positive) with respect to b.latestTick.
func (r *redisBucket) adjustAvail(tick int64, bucketInfo map[string]string) (availInt64 int64, latestTickInt64 int64) {
	avail, _ := strconv.Atoi(bucketInfo[availField])
	capacity, _ := strconv.Atoi(bucketInfo[capacityField])
	latestTick, _ := strconv.Atoi(bucketInfo[latestTickField])
	quantum, _ := strconv.Atoi(bucketInfo[quantumField])

	latestTickInt64 = tick
	if avail >= capacity {
		availInt64 = int64(avail)
		return
	}
	avail += (int(tick) - latestTick) * quantum
	if avail > capacity {
		avail = capacity
	}

	availInt64 = int64(avail)
	latestTickInt64 = tick
	return
}

// RedisStorage is a redisBucket factory.
type RedisStorage struct {
	Client *redis.Client
	Expire time.Duration
}

// NewRedisStorage initializes the in-memory redisBucket store.
func NewRedisStorage(client *redis.Client, expire time.Duration) *RedisStorage {
	return &RedisStorage{
		Expire: expire,
		Client: client,
	}
}

func (r *RedisStorage) Ping() error {
	return r.Client.Ping().Err()
}

// Create create a redisBucket.
func (r *RedisStorage) Create(key string, fillInterval time.Duration, capacity int64) (Bucket, error) {
	// ensure our redis connection is valid
	if err := r.Ping(); err != nil {
		return nil, err
	}
	// if bucket aready exist
	if r.Client.Exists(key).Val() == 1 {
		return &redisBucket{Key: key, Client: r.Client}, nil
	}
	b, err := r.create(key, fillInterval, capacity, 1)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// CreateWithQuantum create a memoryBucket with quantum.
func (r *RedisStorage) CreateWithQuantum(key string, fillInterval time.Duration, capacity int64, quantum int64) (Bucket, error) {
	// ensure our redis connection is valid
	if err := r.Ping(); err != nil {
		return nil, err
	}
	// If bucket aready exist
	if r.Client.Exists(key).Val() == 1 {
		return &redisBucket{Key: key, Client: r.Client}, nil
	}
	b, err := r.create(key, fillInterval, capacity, quantum)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *RedisStorage) create(key string, fillInterval time.Duration, capacity, quantum int64) (*redisBucket, error) {
	if fillInterval <= 0 {
		panic("token bucket fill interval is not > 0")
	}
	if capacity <= 0 {
		panic("token bucket capacity is not > 0")
	}
	if quantum <= 0 {
		panic("token bucket quantum is not > 0")
	}

	err := r.Client.HMSet(key, map[string]interface{}{
		startTimeField:    time.Now().UnixNano(),
		latestTickField:   0,
		fillIntervalField: fillInterval.Nanoseconds(),
		capacityField:     capacity,
		quantumField:      quantum,
		availField:        capacity,
	}).Err()
	if err != nil {
		return nil, err
	}
	r.Client.Expire(key, r.Expire)

	return &redisBucket{
		Key:    key,
		Client: r.Client,
	}, nil
}
