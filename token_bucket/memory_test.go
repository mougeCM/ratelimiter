package token_bucket

import (
	"fmt"
	"time"

	"testing"

	"github.com/stretchr/testify/assert"
)

//------------------------------------Acquire Test------------------------------------------
type acquireReq struct {
	time   time.Duration
	count  int64
	expect int64
}

var acquire1Tests = []struct {
	about        string
	fillInterval time.Duration
	capacity     int64
	quantum      int64
	reqs         []acquireReq
}{{
	about:        "serial requests1",
	fillInterval: 250 * time.Millisecond,
	capacity:     10,
	quantum:      1,
	reqs: []acquireReq{{
		time:   0,
		count:  0,
		expect: 0,
	}, {
		time:   0,
		count:  10,
		expect: 10,
	}, {
		time:   0,
		count:  1,
		expect: 0,
	}, {
		time:   250 * time.Millisecond,
		count:  2,
		expect: 1,
	}},
}, {
	about:        "serial requests2",
	fillInterval: 250 * time.Millisecond,
	capacity:     10,
	quantum:      2,
	reqs: []acquireReq{{
		time:   0,
		count:  0,
		expect: 0,
	}, {
		time:   0,
		count:  10,
		expect: 10,
	}, {
		time:   0,
		count:  1,
		expect: 0,
	}, {
		time:   250 * time.Millisecond,
		count:  2,
		expect: 2,
	}},
}, {
	about:        "concurrent requests",
	fillInterval: 250 * time.Millisecond,
	capacity:     10,
	quantum:      1,
	reqs: []acquireReq{{
		time:   0,
		count:  5,
		expect: 5,
	}, {
		time:   0,
		count:  2,
		expect: 2,
	}, {
		time:   0,
		count:  5,
		expect: 3,
	}, {
		time:   0,
		count:  1,
		expect: 0,
	}},
}, {
	about:        "more than capacity",
	fillInterval: 1 * time.Millisecond,
	capacity:     10,
	quantum:      1,
	reqs: []acquireReq{{
		time:   0,
		count:  10,
		expect: 10,
	}, {
		time:   20 * time.Millisecond,
		count:  15,
		expect: 10,
	}},
}, {
	about:        "within capacity",
	fillInterval: 10 * time.Millisecond,
	capacity:     5,
	quantum:      1,
	reqs: []acquireReq{{
		time:   0,
		count:  5,
		expect: 5,
	}, {
		time:   60 * time.Millisecond,
		count:  5,
		expect: 5,
	}, {
		time:   70 * time.Millisecond,
		count:  1,
		expect: 1,
	}},
}}

var acquire2Tests = []struct {
	about        string
	capacity     int64
	fillInterval time.Duration
	take         int64
	sleep        time.Duration

	expectCountAfterTake  int64
	expectCountAfterSleep int64
}{{
	about:                 "should fill tokens after interval",
	capacity:              5,
	fillInterval:          time.Second,
	take:                  5,
	sleep:                 time.Second,
	expectCountAfterTake:  0,
	expectCountAfterSleep: 1,
}, {
	about:                 "should fill tokens plus existing count",
	capacity:              2,
	fillInterval:          time.Second,
	take:                  1,
	sleep:                 time.Second,
	expectCountAfterTake:  1,
	expectCountAfterSleep: 2,
}, {
	about:                 "shouldn't fill before interval",
	capacity:              2,
	fillInterval:          2 * time.Second,
	take:                  1,
	sleep:                 time.Second,
	expectCountAfterTake:  1,
	expectCountAfterSleep: 1,
}, {
	about:                 "should fill only once after 1*interval before 2*interval",
	capacity:              2,
	fillInterval:          2 * time.Second,
	take:                  1,
	sleep:                 3 * time.Second,
	expectCountAfterTake:  1,
	expectCountAfterSleep: 2,
}}

func TestMemoryAcquire(t *testing.T) {
	asserts := assert.New(t)

	for i, test := range acquire1Tests {
		nmb := NewMemoryBucket()
		tb, err := nmb.CreateWithQuantum(fmt.Sprintf("msf_token_bucket_:%d", i), test.fillInterval, test.capacity, test.quantum)
		asserts.Nil(err, "Token bucket create failed")

		for j, req := range test.reqs {
			d := tb.acquire(tb.StartTime().Add(req.time), req.count)
			asserts.Equal(d, req.expect, fmt.Sprintf("test %d.%d, %s, got %v want %v", i, j, test.about, d, req.expect))
		}
		fmt.Println("Acquire1Tests:", test.about, "-> success")
	}

	for i, test := range acquire2Tests {
		nmb := NewMemoryBucket()
		tb, err := nmb.Create(fmt.Sprintf("msf_token_bucket_:%d", i), test.fillInterval, test.capacity)
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
type tryAcquireReq struct {
	time       time.Duration
	count      int64
	expectWait time.Duration
}

var tryAcquireTests = []struct {
	about        string
	fillInterval time.Duration
	capacity     int64
	reqs         []tryAcquireReq
}{{
	about:        "serial requests",
	fillInterval: 250 * time.Millisecond,
	capacity:     10,
	reqs: []tryAcquireReq{{
		time:       0,
		count:      0,
		expectWait: 0,
	}, {
		time:       0,
		count:      10,
		expectWait: 0,
	}, {
		time:       0,
		count:      1,
		expectWait: 250 * time.Millisecond,
	}, {
		time:       250 * time.Millisecond,
		count:      1,
		expectWait: 250 * time.Millisecond,
	}},
}, {
	about:        "concurrent requests",
	fillInterval: 250 * time.Millisecond,
	capacity:     10,
	reqs: []tryAcquireReq{{
		time:       0,
		count:      10,
		expectWait: 0,
	}, {
		time:       0,
		count:      2,
		expectWait: 500 * time.Millisecond,
	}, {
		time:       0,
		count:      2,
		expectWait: 1000 * time.Millisecond,
	}, {
		time:       0,
		count:      1,
		expectWait: 1250 * time.Millisecond,
	}},
}, {
	about:        "more than capacity",
	fillInterval: 1 * time.Millisecond,
	capacity:     10,
	reqs: []tryAcquireReq{{
		time:       0,
		count:      10,
		expectWait: 0,
	}, {
		time:       20 * time.Millisecond,
		count:      15,
		expectWait: 5 * time.Millisecond,
	}},
}, {
	about:        "sub-quantum time",
	fillInterval: 10 * time.Millisecond,
	capacity:     10,
	reqs: []tryAcquireReq{{
		time:       0,
		count:      10,
		expectWait: 0,
	}, {
		time:       7 * time.Millisecond,
		count:      1,
		expectWait: 3 * time.Millisecond,
	}, {
		time:       8 * time.Millisecond,
		count:      1,
		expectWait: 12 * time.Millisecond,
	}},
}, {
	about:        "within capacity",
	fillInterval: 10 * time.Millisecond,
	capacity:     5,
	reqs: []tryAcquireReq{{
		time:       0,
		count:      5,
		expectWait: 0,
	}, {
		time:       60 * time.Millisecond,
		count:      5,
		expectWait: 0,
	}, {
		time:       60 * time.Millisecond,
		count:      1,
		expectWait: 10 * time.Millisecond,
	}, {
		time:       80 * time.Millisecond,
		count:      2,
		expectWait: 10 * time.Millisecond,
	}},
}}

func TestMemoryTryAcquire(t *testing.T) {
	asserts := assert.New(t)

	for i, test := range tryAcquireTests {
		nmb := NewMemoryBucket()
		tb, err := nmb.Create(fmt.Sprintf("msf_token_bucket_:%d", i), test.fillInterval, test.capacity)
		asserts.Nil(err, "Token bucket create failed")

		for j, req := range test.reqs {
			d, ok := tb.tryAcquire(tb.StartTime().Add(req.time), req.count, infinityDuration)
			asserts.Equal(ok, true, fmt.Sprintf("unexpect: waitTime > maxWait(%v)", infinityDuration))
			asserts.Equal(d, req.expectWait, fmt.Sprintf("test %d.%d, %s, got %v want %v", i, j, test.about, d, req.expectWait))
		}
		fmt.Println("TryAcquireTest:", test.about, "-> success")
	}
}

func TestMemoryPanics(t *testing.T) {
	asserts := assert.New(t)

	asserts.NotPanics(func() {
		nmb := NewMemoryBucket()
		nmb.Create("msf_memory_bucket", 1, 1)
	}, "token bucket fill interval is not > 0")

	asserts.NotPanics(func() {
		nmb := NewMemoryBucket()
		nmb.Create("msf_memory_bucket", 1, 1)
	}, "token bucket capacity is not > 0")

	asserts.NotPanics(func() {
		nmb := NewMemoryBucket()
		nmb.CreateWithQuantum("msf_memory_bucket", 1, 2, 10)
	}, "token bucket quantum is not > 0")
}

//------------------------------------Benchmark------------------------------------------
func BenchmarkMemoryWait(b *testing.B) {
	nmb := NewMemoryBucket()
	tb, _ := nmb.Create("msf_token_bucket", 1, 16*1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Wait(1)
	}
}

func BenchmarkMemoryAcquire(b *testing.B) {
	nmb := NewMemoryBucket()
	tb, _ := nmb.Create("msf_token_bucket", 1, 16*1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Acquire(1)
	}
}
