package main

import (
	"caddy-delivery-network/app/server/apidocs"
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/gen/oapi/worker"
	"caddy-delivery-network/app/server/handlers"
	"caddy-delivery-network/app/server/inits"
	"caddy-delivery-network/app/server/jwt"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"log"
)

func main() {
	// 初始化配置
	cfg, err := inits.Config()
	if err != nil {
		log.Fatal(fmt.Errorf("error loading config: %w", err))
	}

	// 初始化日志
	l, err := inits.Logger(!cfg.System.IsProd)
	if err != nil {
		log.Fatal(fmt.Errorf("error initializing logger: %w", err))
	}

	// 切换日志系统
	l.Debug("logger initialized")

	// 初始化数据库连接
	db, err := inits.DB(cfg.System.DBConnectionString)
	if err != nil {
		l.Fatal("error initializing DB connection", zap.Error(err))
	}

	// 初始化 redis 连接
	rdb, err := inits.Redis(cfg.System.RedisConnectionString)
	if err != nil {
		l.Fatal("error initializing Redis connection", zap.Error(err))
	}

	// 初始化 JWT
	j, err := jwt.New(cfg.Security.SignatureSecretKey)
	if err != nil {
		l.Fatal("error initializing JWT", zap.Error(err))
	}

	// 准备 handler app
	handlerApp := handlers.NewApp(l, db, rdb, j, cfg.Security.EncryptSecretKey)

	// 准备 echo 服务
	e := echo.New()
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:    true,
		LogStatus: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			l.Info("request",
				zap.String("URI", v.URI),
				zap.Int("status", v.Status),
			)

			return nil
		},
	}))
	e.Use(middleware.Recover())

	// 绑定 echo 服务
	admin.RegisterHandlers(e, handlerApp)
	worker.RegisterHandlers(e, handlerApp)

	// 添加 API 文档
	if !cfg.System.IsProd {
		if swg, err := admin.GetSwagger(); err != nil {
			l.Error("error initializing admin swagger", zap.Error(err))
		} else if swgJson, err := swg.MarshalJSON(); err != nil {
			l.Error("error initializing admin swagger", zap.Error(err))
		} else {
			e.Pre(apidocs.Doc("/api/admin", swgJson))
		}

		if swg, err := worker.GetSwagger(); err != nil {
			l.Error("error initializing worker swagger", zap.Error(err))
		} else if swgJson, err := swg.MarshalJSON(); err != nil {
			l.Error("error initializing worker swagger", zap.Error(err))
		} else {
			e.Pre(apidocs.Doc("/api/worker", swgJson))
		}
	}

	// 启动 echo 服务
	if err := e.Start(cfg.System.Listen); err != nil {
		l.Fatal("shutting down the server", zap.Error(err))
	}
}
