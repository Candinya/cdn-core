package handlers

import (
	"caddy-delivery-network/app/server/gen/oapi/worker"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

func (a *App) heartbeat() {
	// 设置并发锁，避免读写冲突
	if !a.lock.TryLock() {
		// 上一轮正在处理，跳过这一轮
		return
	}
	defer a.lock.Unlock() // 使用 defer 而非放在最后，来确保在意外的提前返回时也能正常解锁，而非造成死锁

	// 给服务器发送 heartbeat 请求，拉取数据

	// 准备请求的基础信息
	hbPath := fmt.Sprintf("/api/worker/%d/heartbeat", a.cfg.InstanceID)
	hbReqUrl, err := url.JoinPath(a.cfg.ServerEndpoint, hbPath)
	if err != nil {
		a.l.Error("failed to join heartbeat request url", zap.String("server", a.cfg.ServerEndpoint), zap.String("path", hbPath), zap.Error(err))
		return
	}
	hbReq, err := http.NewRequest("GET", hbReqUrl, nil)
	if err != nil {
		a.l.Error("failed to prepare heartbeat request", zap.String("url", hbReqUrl), zap.Error(err))
		return
	}
	hbReq.Header.Set("Authorization", "Bearer "+a.cfg.InstanceToken)

	// 发送请求
	hbRes, err := http.DefaultClient.Do(hbReq)
	if err != nil {
		a.l.Error("failed to send heartbeat request", zap.Any("req", hbReq), zap.Error(err))
		return
	}

	defer hbRes.Body.Close()

	// 解析请求体
	var hbResBody worker.HeartbeatRes
	err = json.NewDecoder(hbRes.Body).Decode(&hbResBody)
	if err != nil {
		a.l.Error("failed to decode heartbeat response", zap.Any("resp", hbResBody), zap.Error(err))
		return
	}

	// 分析文件列表
	for _, fileList := range hbResBody.FilesUpdatedAt {
		if fileStat, err := os.Stat(fileList.Path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// 需要创建文件，先创建目录，文件由之后非返回的条件统一创建
				parentDir := filepath.Dir(fileList.Path)
				if err := os.MkdirAll(parentDir, 0755); err != nil {
					a.l.Error("failed to create parent directory", zap.String("path", parentDir), zap.Error(err))
					continue
				}
			} else {
				a.l.Error("failed to stat file", zap.String("path", fileList.Path), zap.Error(err))
				continue
			}
		} else if fileList.UpdatedAt <= fileStat.ModTime().Unix() {
			// 不需要更新文件，就继续处理下一个了
			continue
		}

		// 文件不存在或需要更新，则需要写入文件
		if err := a.updateFile(fileList.Path); err != nil {
			a.l.Error("failed to update file", zap.String("path", fileList.Path), zap.Error(err))
		}
	}

	// 分析配置是否发生更新
	if hbResBody.ConfigUpdatedAt > a.lastConfigUpdate {
		if err := a.updateConfig(); err != nil {
			a.l.Error("failed to update config", zap.Error(err))
		} else {
			a.lastConfigUpdate = time.Now().Unix() // 使用当前时间戳作为配置更新时间
		}
	}
}

func (a *App) updateFile(fPath string) error {
	// 请求文件数据
	filePath := fmt.Sprintf("/api/worker/%d/file", a.cfg.InstanceID)
	fileReqUrl, err := url.JoinPath(a.cfg.ServerEndpoint, filePath)
	if err != nil {
		a.l.Error("failed to join file request url", zap.String("server", a.cfg.ServerEndpoint), zap.String("path", filePath), zap.Error(err))
		return fmt.Errorf("fail to join file request url: %w", err)
	}
	fileReq, err := http.NewRequest("GET", fileReqUrl, nil)
	if err != nil {
		a.l.Error("failed to prepare file request", zap.String("url", fileReqUrl), zap.Error(err))
		return fmt.Errorf("fail to prepare file request: %w", err)
	}
	fileReq.Header.Set("Authorization", "Bearer "+a.cfg.InstanceToken)
	fileReq.Header.Set("X-File-Path", filePath)

	// 发送请求
	fileRes, err := http.DefaultClient.Do(fileReq)
	if err != nil {
		a.l.Error("failed to send file request", zap.Any("req", fileReq), zap.Error(err))
		return fmt.Errorf("fail to send file request: %w", err)
	}

	defer fileRes.Body.Close()

	// 打开文件
	f, err := os.OpenFile(fPath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		a.l.Error("failed to open file", zap.String("path", fPath), zap.Error(err))
		return fmt.Errorf("fail to open file: %w", err)
	}

	defer f.Close()

	// 写出数据
	if _, err := io.Copy(f, fileRes.Body); err != nil {
		a.l.Error("failed to copy file response", zap.Any("resp", fileReq), zap.Error(err))
		return fmt.Errorf("fail to copy file response: %w", err)
	}

	return nil
}

func (a *App) updateConfig() error {
	// 请求配置数据
	configPath := fmt.Sprintf("/api/worker/%d/config", a.cfg.InstanceID)
	configReqUrl, err := url.JoinPath(a.cfg.ServerEndpoint, configPath)
	if err != nil {
		a.l.Error("failed to join config request url", zap.String("server", a.cfg.ServerEndpoint), zap.String("path", configPath), zap.Error(err))
		return fmt.Errorf("fail to join config request url: %w", err)
	}
	configReq, err := http.NewRequest("GET", configReqUrl, nil)
	if err != nil {
		a.l.Error("failed to prepare config request", zap.String("url", configReqUrl), zap.Error(err))
		return fmt.Errorf("fail to prepare config request: %w", err)
	}
	configReq.Header.Set("Authorization", "Bearer "+a.cfg.InstanceToken)

	// 发送请求
	configRes, err := http.DefaultClient.Do(configReq)
	if err != nil {
		a.l.Error("failed to send config request", zap.Any("req", configReq), zap.Error(err))
		return fmt.Errorf("fail to send config request: %w", err)
	}

	defer configRes.Body.Close()

	// 准备配置更新请求
	caddyConfigUpdateReqUrl, err := url.JoinPath(a.cfg.CaddyEndpoint, "/load")
	if err != nil {
		a.l.Error("failed to prepare caddy config update url", zap.Error(err))
		return fmt.Errorf("fail to prepare caddy config update url: %w", err)
	}
	caddyConfigUpdateReq, err := http.NewRequest("POST", caddyConfigUpdateReqUrl, configReq.Body)
	if err != nil {
		a.l.Error("failed to prepare caddy config update url", zap.Error(err))
		return fmt.Errorf("fail to prepare caddy config update url: %w", err)
	}
	caddyConfigUpdateReq.Header.Set("Content-Type", "text/caddyfile")

	// 发送请求
	caddyConfigUpdateRes, err := http.DefaultClient.Do(caddyConfigUpdateReq)
	if err != nil {
		a.l.Error("failed to send caddy config update request", zap.Any("req", caddyConfigUpdateReq), zap.Error(err))
		return fmt.Errorf("fail to send caddy config update request: %w", err)
	}

	defer caddyConfigUpdateRes.Body.Close()

	if caddyConfigUpdateRes.StatusCode != http.StatusOK {
		a.l.Error("failed to update caddy config", zap.Int("code", caddyConfigUpdateRes.StatusCode), zap.Any("res", caddyConfigUpdateRes), zap.Error(err))
		return fmt.Errorf("failed to update caddy config")
	}

	// 返回
	return nil
}
