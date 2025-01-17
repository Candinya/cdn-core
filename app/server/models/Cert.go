package models

import (
	"encoding/json"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"time"
)

type Cert struct {
	gorm.Model

	// 证书的基础信息
	Name      string          `gorm:"column:name"`                // 证书的名字，方便记忆
	Domains   pq.StringArray  `gorm:"column:domains;type:text[]"` // 证书的域名，可以为多个
	Provider  json.RawMessage `gorm:"column:provider;type:jsonb"` // 提供方信息（用 JSONB 存储方便扩展）， NULL 表示手动管理
	ExpiresAt time.Time       `gorm:"column:expires_at;index"`    // 证书的过期时间，如果是自动管理则会在过期前尝试自动续期（未来实现），也可以调用接口强制 renew

	// 证书的本体信息
	Certificate             string `gorm:"column:certificate"`              // 签发的证书
	PrivateKey              []byte `gorm:"column:private_key;type:bytea"`   // 私钥，使用来自环境变量的 secret key 加密 (AES-GCM)
	IntermediateCertificate string `gorm:"column:intermediate_certificate"` // 中间证书
	CSR                     string `gorm:"column:csr"`                      // 签发请求信息
}
