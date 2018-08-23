package tkbucket

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
)

const (
	estimateVal  = 500
	bucketExpire = 3 * time.Hour
)

var redisClient = redis.NewClient(&redis.Options{Addr: ":6379"})

//------------------------------------Acquire Test------------------------------------------
func TestRedisAcquire(t *testing.T) {
	asserts := assert.New(t)

	for i, test := range acquire1Tests {
		nrs := NewRedisStorage(bucketExpire, redisClient)
		// NOTE: Reset data
		nrs.Client.FlushDB()

		tb, err := nrs.CreateWithQuantum(fmt.Sprintf("msf_token_bucket_:%d", i), test.fillInterval, test.capacity, test.quantum)
		asserts.Nil(err, "Token bucket create failed")

		for j, req := range test.reqs {
			d := tb.acquire(tb.StartTime().Add(req.time), req.count)
			asserts.Equal(d, req.expect, fmt.Sprintf("test %d.%d, %s, got %v want %v", i, j, test.about, d, req.expect))
		}
		fmt.Println("Acquire1Tests:", test.about, "-> success")
	}

	for i, test := range acquire2Tests {
		nrs := NewRedisStorage(bucketExpire, redisClient)
		// NOTE: Reset data
		nrs.Client.FlushDB()

		tb, err := nrs.Create(fmt.Sprintf("msf_token_bucket_:%d", i), test.fillInterval, test.capacity)
		asserts.Nil(err, "Token bucket create failed")

		// The number of tokens taked by the test is correct.
		c := tb.acquire(tb.StartTime(), test.take)
		asserts.Equal(c, test.take, fmt.Sprintf("#%d: %s, take = %d, want = %d", i, test.about, c, test.take))
		// It is correct to test the remaining number of tokens.
		c = tb.available(tb.StartTime())
		asserts.Equal(c, test.expectCountAfterTake, fmt.Sprintf("#%d: %s, after take, available = %d, want = %d", i, test.about, c, test.expectCountAfterTake))
		// After sleep，It is correct to test the remaining number of tokens.
		c = tb.available(tb.StartTime().Add(test.sleep))
		asserts.Equal(c, test.expectCountAfterSleep, fmt.Sprintf("#%d: %s, after some time it should fill in new tokens, available = %d, want = %d",
			i, test.about, c, test.expectCountAfterSleep))
		fmt.Println("Acquire2Tests:", test.about, "-> success")
	}
}

//------------------------------------TryAcquire Test------------------------------------------
func TestRedisTryAcquire(t *testing.T) {
	asserts := assert.New(t)

	for i, test := range tryAcquireTests {
		nrs := NewRedisStorage(bucketExpire, redisClient)
		// NOTE: Reset data
		nrs.Client.FlushDB()

		tb, err := nrs.Create(fmt.Sprintf("msf_token_bucket_:%d", i), test.fillInterval, test.capacity)
		asserts.Nil(err, "Token bucket create failed")

		for j, req := range test.reqs {
			d, ok := tb.tryAcquire(tb.StartTime().Add(req.time), req.count, infinityDuration)
			asserts.Equal(ok, true, fmt.Sprintf("unexpect: waitTime > maxWait(%v)", infinityDuration))
			abs := math.Abs(float64(d.Nanoseconds() - req.expectWait.Nanoseconds()))
			if abs > estimateVal {
				asserts.Equal(d, req.expectWait, fmt.Sprintf("test %d.%d, %s, got %v want %v, abs(%v) exceed estimate", i, j, test.about, d, req.expectWait, abs))
			}
		}
		fmt.Println("TryAcquireTest:", test.about, "-> success")
	}
}

func TestRedisPanics(t *testing.T) {
	asserts := assert.New(t)

	asserts.NotPanics(func() {
		nrs := NewRedisStorage(bucketExpire, redisClient)
		// NOTE: Reset data
		nrs.Client.FlushDB()

		nrs.Create("msf_redis_bucket", 1, 1)
	}, "token bucket fill interval is not > 0")

	asserts.NotPanics(func() {
		nrs := NewRedisStorage(bucketExpire, redisClient)
		// NOTE: Reset data
		nrs.Client.FlushDB()

		nrs.Create("msf_redis_bucket", 1, 1)
	}, "token bucket capacity is not > 0")

	asserts.NotPanics(func() {
		nrs := NewRedisStorage(bucketExpire, redisClient)
		// NOTE: Reset data
		nrs.Client.FlushDB()

		nrs.CreateWithQuantum("msf_redis_bucket", 1, 2, 10)
	}, "token bucket quantum is not > 0")
}

//------------------------------------Benchmark------------------------------------------
func BenchmarkRedisWait(b *testing.B) {
	nrs := NewRedisStorage(bucketExpire, redisClient)
	// NOTE: Reset data
	nrs.Client.FlushDB()

	tb, _ := nrs.Create("msf_token_bucket", 1, 16*1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Wait(1)
	}
}

func BenchmarkRedisAcquire(b *testing.B) {
	nrs := NewRedisStorage(bucketExpire, redisClient)
	// NOTE: Reset data
	nrs.Client.FlushDB()

	tb, _ := nrs.Create("msf_token_bucket", 1, 16*1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Acquire(1)
	}
}
