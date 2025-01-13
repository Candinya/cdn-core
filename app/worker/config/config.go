package config

import (
	"time"
)

type Config struct {
	// 基础配置
	IsProd bool

	// 与 Server 通信配置
	ServerEndpoint    string
	InstanceID        uint
	InstanceToken     string
	HeartbeatInterval time.Duration

	// 对 Caddy 控制配置
	CaddyEndpoint string
}
