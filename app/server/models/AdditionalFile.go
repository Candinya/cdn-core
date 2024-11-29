package models

import "gorm.io/gorm"

type AdditionalFile struct {
	gorm.Model

	Name    string `gorm:"column:name"`               // 文件标记
	Path    string `gorm:"column:path"`               // 文件路径
	Content []byte `gorm:"column:content;type:bytea"` // 文件内容（二进制）
}
