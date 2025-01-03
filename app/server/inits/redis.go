package inits

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

func Redis(conn string) (rdb *redis.Client, err error) {
	if options, err := redis.ParseURL(conn); err != nil {
		return nil, fmt.Errorf("parse redis url error: %v", err)
	} else {
		rdb = redis.NewClient(options)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err = rdb.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("redis ping error: %v", err)
		}
	}
	return rdb, nil
}
