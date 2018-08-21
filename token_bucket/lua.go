package token_bucket

const (
	luaCommonFuc = `
		local currentTick = function(nowTime, startTime, fillInterval) 
			-- current time tick, measured from startTime
			-- +500 to reduce accuracy error
			return math.floor((nowTime - startTime + 500) / fillInterval)
		end

		local adjustAvail = function(tick, avail, capacity, latestTick, quantum)
			if avail > capacity
			then
				return avail, tick
			end

			avail = avail + (tick - latestTick) * quantum
			if avail > capacity
			then
				avail = capacity
			end

			return avail, tick
		end
	`

	luaAcquire = luaCommonFuc + `
		local key = KEYS[1]
		local nowTime = tonumber(ARGV[1])
		local count = tonumber(ARGV[2])
		local bulk = redis.call("hmget", key, "start_time", "fill_interval", "capacity", "quantum", "avail", "latest_tick")
		if bulk ~= nil then
			local startTime = tonumber(bulk[1])
			local fillInterval = tonumber(bulk[2])
			local capacity = tonumber(bulk[3])
			local quantum = tonumber(bulk[4])
			local avail = tonumber(bulk[5])
			local latestTick = tonumber(bulk[6])

			local tick = currentTick(nowTime, startTime, fillInterval)
			avail, latestTick = adjustAvail(tick, avail, capacity, latestTick, quantum)
			if avail <= 0 
			then
				return 0
			end

			if count > avail 
			then
				count = avail
			end

			avail = avail - count
			-- Update bucket data
			redis.call("hmset", key, "avail", avail, "latest_tick", latestTick)

			return count
		end

  		return nil
	`

	luaAvailable = luaCommonFuc + `
		local key = KEYS[1]
		local nowTime = tonumber(ARGV[1])
		local bulk = redis.call("hmget", key, "start_time", "fill_interval", "capacity", "quantum", "avail", "latest_tick")
		if bulk ~= nil then
			local startTime = tonumber(bulk[1])
			local fillInterval = tonumber(bulk[2])
			local capacity = tonumber(bulk[3])
			local quantum = tonumber(bulk[4])
			local avail = tonumber(bulk[5])
			local latestTick = tonumber(bulk[6])

			local tick = currentTick(nowTime, startTime, fillInterval)
			avail, latestTick = adjustAvail(tick, avail, capacity, latestTick, quantum)
			-- Update bucket data
			redis.call("hmset", key, "avail", avail, "latest_tick", latestTick)

			return avail
		end

  		return nil
	`

	luaTryAcquire = luaCommonFuc + `
		local key = KEYS[1]
		local nowTime = tonumber(ARGV[1])
		local count = tonumber(ARGV[2])
		local bulk = redis.call("hmget", key, "start_time", "fill_interval", "capacity", "quantum", "avail", "latest_tick")
		if bulk ~= nil then
			local startTime = tonumber(bulk[1])
			local fillInterval = tonumber(bulk[2])
			local capacity = tonumber(bulk[3])
			local quantum = tonumber(bulk[4])
			local avail = tonumber(bulk[5])
			local latestTick = tonumber(bulk[6])

			local tick = currentTick(nowTime, startTime, fillInterval)
			avail, latestTick = adjustAvail(tick, avail, capacity, latestTick, quantum)
			avail = avail - count
			-- Update bucket data
			redis.call("hmset", key, "avail", avail, "latest_tick", latestTick)
			if avail >= 0
			then
				return 0
			end

			local endTick = tick + (-avail + quantum - 1) / quantum
			local endTime = startTime + endTick * fillInterval

			return endTime
		end

  		return nil
	`
)
