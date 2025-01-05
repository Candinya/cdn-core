package constants

import "time"

const (
	CacheKeyInstanceInfo      = "cdn:instance:info:%d"
	CacheKeyInstanceConfig    = "cdn:instance:config:%d"
	CacheKeyInstanceFiles     = "cdn:instance:files:%d"
	CacheKeyInstanceHeartbeat = "cdn:instance:heartbeat:%d"
	CacheKeyInstanceLastseen  = "cdn:instance:lastseen:%d"
)

const (
	CacheExpireInstanceInfo      = 1 * time.Hour
	CacheExpireInstanceConfig    = 12 * time.Hour
	CacheExpireInstanceHeartbeat = 1 * time.Hour
	CacheExpireInstanceLastseen  = 12 * time.Hour
)
