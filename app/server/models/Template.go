package models

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Template struct {
	gorm.Model

	Name        string         `gorm:"column:name"`                  // 模板名字
	Description string         `gorm:"column:description"`           // 模板描述（介绍）
	Content     string         `gorm:"column:content"`               // 模板内容
	Variables   pq.StringArray `gorm:"column:variables;type:text[]"` // 模板变量
}
