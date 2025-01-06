package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Instance struct {
	gorm.Model

	Name         string    `gorm:"column:name"`                  // 实例名称
	Token        uuid.UUID `gorm:"column:token;type:uuid;index"` // 与 worker 通讯时使用的 token
	PreConfig    string    `gorm:"column:pre_config"`            // 在 Caddyfile 中，比所有服务器配置都靠前的部分，用于指引基础选项（例如全局配置）
	IsManualMode bool      `gorm:"column:is_manual_mode"`        // 是否为手动管理模式：不会通过 worker 应用服务器信息，不记录最后一次心跳状态
	// LastSeen time.Time // 最后一次心跳，用于确认状态是否在线，还是离线（失联） // 这个存到 redis 里

	AdditionalFileIDs []uint `gorm:"column:additional_file_ids;index"` // 使用到的额外文件
	SiteIDs           []uint `gorm:"column:site_ids;index"`            // 部署在实例上的站点
}
