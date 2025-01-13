package main

import (
	"caddy-delivery-network/app/worker/handlers"
	"caddy-delivery-network/app/worker/inits"
	"fmt"
	"log"
)

func main() {
	// 初始化配置
	cfg, err := inits.Config()
	if err != nil {
		log.Fatal(fmt.Errorf("error loading config: %w", err))
	}

	// 初始化日志
	l, err := inits.Logger(!cfg.IsProd)
	if err != nil {
		log.Fatal(fmt.Errorf("error initializing logger: %w", err))
	}

	// 切换日志系统
	l.Debug("logger initialized")

	// 开启心跳循环
	handlerApp := handlers.NewApp(cfg, l)
	handlerApp.Start()

	// 卡住进程避免结束
	select {}
}
