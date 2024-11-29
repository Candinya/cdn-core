package models

import "gorm.io/gorm"

type User struct {
	gorm.Model

	// 基础信息
	Username string `gorm:"column:username;uniqueIndex"` // 用户名，全局唯一
	Name     string `gorm:"column:name"`                 // 显示名称
	IsAdmin  bool   `gorm:"column:is_admin"`             // 是否为管理员：管理员可以写入（更改），非管理员只能读取（浏览）

	// 登录与授权认证相关
	Password   string `gorm:"column:password"`     // 密码，使用 argon2id 储存
	JWTSignKey string `gorm:"column:jwt_sign_key"` // 用于签发 JWT Token 的密钥
}
