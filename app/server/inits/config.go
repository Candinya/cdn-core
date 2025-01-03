package inits

import (
	"caddy-delivery-network/app/server/config"
	"fmt"
	"os"
	"strings"
)

func Config() (cfg *config.Config, err error) {
	// 手动配置映射，如果这里有什么自动映射工具就好了， viper 好像处理这种基于环境变量的配置也不是很方便
	{
		mode, exist := os.LookupEnv("MODE")
		cfg.System.IsProd = exist && strings.HasPrefix(strings.ToLower(mode), "p")
	}

	if listen, exist := os.LookupEnv("LISTEN"); !exist {
		cfg.System.Listen = ":1323" // 默认监听地址
	} else {
		cfg.System.Listen = listen
	}

	if dbconn, exist := os.LookupEnv("DB_CONN"); !exist {
		return nil, fmt.Errorf("DB_CONN environment variable not set")
	} else {
		cfg.System.DBConnectionString = dbconn
	}

	if redisconn, exist := os.LookupEnv("REDIS_CONN"); !exist {
		return nil, fmt.Errorf("REDIS_CONN environment variable not set")
	} else {
		cfg.System.RedisConnectionString = redisconn
	}

	if encsk, exist := os.LookupEnv("ENCRYPT_SECRET_KEY"); !exist {
		return nil, fmt.Errorf("ENCRYPT_SECRET_KEY environment variable not set")
	} else {
		cfg.Security.EncryptSecretKey = encsk
	}

	if sigsk, exist := os.LookupEnv("SIGNATURE_SECRET_KEY"); !exist {
		return nil, fmt.Errorf("SIGNATURE_SECRET_KEY environment variable not set")
	} else {
		cfg.Security.SignatureSecretKey = sigsk
	}

	return cfg, nil
}
