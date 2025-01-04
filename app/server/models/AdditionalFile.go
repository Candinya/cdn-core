package models

import "gorm.io/gorm"

type AdditionalFile struct {
	gorm.Model

	Name     string `gorm:"column:name"`               // 文件标记
	Filename string `gorm:"column:filename"`           // 文件名
	Content  []byte `gorm:"column:content;type:bytea"` // 文件内容（二进制）
}
