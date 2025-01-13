package inits

import (
	"caddy-delivery-network/app/worker/config"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func Config() (*config.Config, error) {
	var cfg config.Config
	{
		mode, exist := os.LookupEnv("MODE")
		cfg.IsProd = exist && strings.HasPrefix(strings.ToLower(mode), "p")
	}

	if serverEp, exist := os.LookupEnv("SERVER_ENDPOINT"); !exist {
		return nil, fmt.Errorf("SERVER_ENDPOINT environment variable not set")
	} else {
		cfg.ServerEndpoint = serverEp
	}

	if instanceIdStr, exist := os.LookupEnv("INSTANCE_ID"); !exist {
		return nil, fmt.Errorf("INSTANCE_ID environment variable not set")
	} else if instanceId, err := strconv.Atoi(instanceIdStr); err != nil {
		return nil, fmt.Errorf("INSTANCE_ID should be an integer")
	} else {
		cfg.InstanceID = uint(instanceId)
	}

	if instanceToken, exist := os.LookupEnv("INSTANCE_TOKEN"); !exist {
		return nil, fmt.Errorf("INSTANCE_TOKEN environment variable not set")
	} else {
		cfg.InstanceToken = instanceToken
	}

	if heartbeatIntervalStr, exist := os.LookupEnv("HEARTBEAT_INTERVAL"); !exist {
		cfg.HeartbeatInterval = 1 * time.Minute // 默认每分钟一次
	} else if interval, err := time.ParseDuration(heartbeatIntervalStr); err != nil {
		return nil, fmt.Errorf("HEARTBEAT_INTERVAL should be a valid duration")
	} else {
		cfg.HeartbeatInterval = interval
	}

	if caddyEp, exist := os.LookupEnv("CADDY_ENDPOINT"); !exist {
		return nil, fmt.Errorf("CADDY_ENDPOINT environment variable not set")
	} else {
		cfg.CaddyEndpoint = caddyEp
	}

	return &cfg, nil
}
