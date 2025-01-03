package handlers

import (
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/gen/oapi/worker"
	"caddy-delivery-network/app/server/jwt"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var _ admin.ServerInterface = (*App)(nil)
var _ worker.ServerInterface = (*App)(nil)

type App struct {
	l   *zap.Logger   // 日志
	db  *gorm.DB      // 数据库
	rdb *redis.Client // Redis
	jwt *jwt.JWT      // JWT ，用于无状态验证
	esk []byte        // 加密用密钥 (EncryptSecretKey)
}

func NewApp(l *zap.Logger, db *gorm.DB, rdb *redis.Client, j *jwt.JWT, esk string) *App {
	return &App{
		l:   l,
		db:  db,
		rdb: rdb,
		jwt: j,
		esk: []byte(esk),
	}
}
