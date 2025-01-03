package models

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Site struct {
	gorm.Model

	// 站点基础信息
	Name   string `gorm:"column:name"`   // 站点名称，只在系统里标记使用
	Origin string `gorm:"column:origin"` // 来源，可以是 http://localhost:8080 这种，也可以只是 localhost ，也可以是多个

	// 站点使用的配置模板
	TemplateID     uint           `gorm:"column:template_id;index"`           // 使用的模板 ID
	TemplateValues pq.StringArray `gorm:"column:template_values;type:text[]"` // 模板内变量的值（与模板变量一一对应）

	// 站点使用的证书
	CertID *uint `gorm:"column:cert_id;index"` // 使用的证书 ID ， NULL 表示由目标 Caddy 自行申请管理（ HTTPS 模式下）

	// 连接模型时使用
	Template Template `gorm:"foreignKey:TemplateID"` // 模板
	Cert     *Cert    `gorm:"foreignKey:CertID"`     // 证书
}
