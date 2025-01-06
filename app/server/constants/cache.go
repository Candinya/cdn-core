package constants

import "time"

const (
	CacheKeyInstanceInfo      = "cdn:instance:info:%d"      // 主要是认证使用
	CacheKeyInstanceConfig    = "cdn:instance:config:%d"    // 存储配置文件（ Caddyfile ）
	CacheKeyInstanceFiles     = "cdn:instance:files:%d"     // 针对不同实例设置不同的缓存表，是因为可能会有不同内容的同名文件
	CacheKeyInstanceHeartbeat = "cdn:instance:heartbeat:%d" // 存储心跳数据，即各个文件的更新时间戳
	CacheKeyInstanceLastseen  = "cdn:instance:lastseen:%d"  // 存储上一次心跳通信时间，用于判断是否在线
)

const (
	CacheExpireInstanceInfo      = 1 * time.Hour
	CacheExpireInstanceConfig    = 12 * time.Hour
	CacheExpireInstanceHeartbeat = 1 * time.Hour
	CacheExpireInstanceLastseen  = 12 * time.Hour
)
