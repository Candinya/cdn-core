package config

type Config struct {
	System struct {
		IsProd                bool   // 是否为生产环境
		Listen                string // 监听地址
		DBConnectionString    string // Postgres 数据库的连接字符串
		RedisConnectionString string // Redis 数据库的连接字符串
	}
	Security struct {
		EncryptSecretKey   string // 加密密钥，用于加密数据库中的敏感信息（例如证书），设定后不能更改
		SignatureSecretKey string // 签名密钥，用于产生签名（例如 JWT ），更新会导致旧有会话失效，但不影响使用
	}
}
