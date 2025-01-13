package handlers

import (
	"caddy-delivery-network/app/worker/config"
	"go.uber.org/zap"
	"sync"
	"time"
)

type App struct {
	cfg *config.Config
	l   *zap.Logger

	lastConfigUpdate int64
	ticker           *time.Ticker
	stopChan         chan struct{}
	lock             *sync.Mutex
}

func NewApp(cfg *config.Config, l *zap.Logger) *App {
	return &App{
		cfg: cfg,
		l:   l,
	}
}

func (a *App) Start() {
	a.ticker = time.NewTicker(a.cfg.HeartbeatInterval)
	a.stopChan = make(chan struct{})
	go a.loop()
}

func (a *App) loop() {
	select {
	case <-a.ticker.C:
		a.l.Debug("heartbeat loop")
		a.heartbeat()
	case <-a.stopChan:
		a.l.Debug("stop heartbeat loop")
		close(a.stopChan)
		break
	}
}

func (a *App) Stop() {
	a.ticker.Stop()
	a.stopChan <- struct{}{}
}
