### 限流之令牌桶设计方案

### 1. 存储[灵活扩展]

_1.本地内存

>* 采用`map结构`存储   

```Golang
type MemoryStorage struct {
	buckets map[string]*memoryBucket
}

type memoryBucket struct {
	mu sync.Mutex
	
	startTime time.Time           // 桶创建的时间
	fillInterval time.Duration    // 放入令牌的时间间隔
	capacity int64                // 桶容量
	quantum int64                 // 量子[每个fill_interval间隔放入令牌的数量]
	avail int64                   // 桶中可用的令牌数量
	latestTick int64              // 保存最新的刻度，方便计算桶中的令牌数量
}
```

_2.Redis_

>* 采用`Hash表`结构      
>* 可以考虑使用`Lua+Redis`，原子操作进行计算并且减少网络开销

```Golang
type RedisStorage struct {
	Client *redis.Client
}
```

### 2. 支持级别

_集群级别_

- 使用Redis方案存储

_服务级别_

- 使用本地内存方案存储

_用户级别[`每个用户一个桶`]_

- 根据需求`本地内存`和`Redis`方案都可选择

### 3. 上层接口设计

**存储通用接口**

```
type Storage interface {
	// Ping               确保服务是否正常
	Ping() error
	// Create             创建bucket
	Create(name string, fillInterval time.Duration, capacity int64) (Bucket, error)
	// CreateWithQuantum  根据一个量子创建bucket
	CreateWithQuantum(name string, fillInterval time.Duration, capacity, quantum int64) (Bucket, error)
}
```

**令牌桶通用接口**

```
type Bucket interface {
	// Acquire     从桶中获取令牌[>0代表获取到了令牌]
	Acquire(count int64) int64
	// TryAcquire  参试从桶中获取令牌[返回需要等待获取到足够令牌所需的时间]
	TryAcquire(count int64) time.Duration
	// Wait        参试去从桶中获取令牌，获取不到则自动等待直到获取到为止
	Wait(count int64)
	// Available   获取可用的令牌数
	Available() int64
	// StartTime   获取桶创建时间
	StartTime() time.Time
	// Capacity    获取桶的容量
	Capacity() int64
}
```

### 4. 设计优势

- 支持灵活扩展存储模式
- 支持多级别限流策略
- 使用动态计算代替单独协程添加令牌数，增强灵活性
- Redis限流策略加入lua脚本支持，保证整个代码块的原子性并且减少网络开销，提升性能
- 支持多种获取令牌策略

### 5. 实际需求方案确定

_1.当可用令牌不足时怎么处理请求？_

- 直接返回错误，上层处理错误

_2.使用`集群+用户级别方案`，怎么保证Redis桶的失效[防止内存溢出]？_

- 对key设定有效期[暂定3小时]

_3.怎么对某个服务中的某个接口的某个黑名单用户进行qps限制?_

> 黑名单结构存储     

```
	存储在Redis，使用Set(集合)
	key   -> service:{serviceid}:bucket:bl
	value -> {userid}
	
	redis.sadd(key, value)
```

> 令牌桶结构存储

```
	存储在Redis，使用Hash(哈希表)
	key   -> service:{serviceid}:method:{name}:userid:{userid}:tk_bucket
	value -> start_time      桶创建的时间
	         fill_interval   放入令牌的时间间隔
	         capacity        桶容量
	         quantum         量子[每个fill_interval间隔放入令牌的数量]
	         avail           桶中可用的令牌数量
	         latest_tick     保存最新的刻度，方便计算桶中的令牌数量
	
	redis.hmset(key, field, value)     
```

_4.解封接口_