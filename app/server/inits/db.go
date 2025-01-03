package inits

import (
	"caddy-delivery-network/app/server/models"
	"fmt"
	"github.com/alexedwards/argon2id"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func DB(conn string) (db *gorm.DB, err error) {
	// 打开连接
	if db, err = gorm.Open(postgres.Open(conn), &gorm.Config{}); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 迁移
	if err = mig(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// 初始化启动数据
	if err = initData(db); err != nil {
		return nil, fmt.Errorf("failed to init data into database: %w", err)
	}

	// 返回
	return db, nil
}

func mig(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Template{},
		&models.Cert{},
		&models.Site{},
		&models.AdditionalFile{},
		&models.Instance{},
	)
}

func initData(db *gorm.DB) (err error) {
	// 查询现有记录数量
	var counter int64

	// 初始化用户
	if err = db.Model(&models.User{}).Count(&counter).Error; err != nil {
		return fmt.Errorf("failed to get user count: %w", err)
	} else if counter == 0 { // 没有任何用户，添加初始用户
		// 创建密码
		var password string
		if password, err = argon2id.CreateHash("password", argon2id.DefaultParams); err != nil {
			return fmt.Errorf("failed to generate password: %w", err)
		}

		// 插入记录
		if err = db.Create(&models.User{
			Username: "admin",
			Name:     "CDN Admin",
			IsAdmin:  true,
			Password: password,
		}).Error; err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}
	}

	// 初始化模板
	if err = db.Model(&models.Template{}).Count(&counter).Error; err != nil {
		return fmt.Errorf("failed to get template count: %w", err)
	} else if counter == 0 { // 没有任何模板，添加初始模板
		// 插入记录
		if err = db.Create([]*models.Template{
			{
				Name:        "空白模板",
				Description: "没有内置任何内容，一切自定义",
				Content:     "{{.Origin}} {\n    {{.Cert}}\n{{.Content}}\n}",
				Variables:   []string{"Content"},
			},
			{
				Name:        "简单反代",
				Description: "简单的反向代理，使用 Caddy 内置的证书管理",
				Content:     "{{.Origin}} {\n    {{.Cert}}\n    reverse_proxy {{.Source}}\n}",
				Variables:   []string{"Source"},
			},
			{
				Name:        "变源反代",
				Description: "使用源站（HTTPS）不知道的 SNI 做代理",
				Content:     "{{.Origin}} {\n    {{.Cert}}\n    reverse_proxy https://{{.Source}} {\n        header_up Host {{.Source}}\n        transport http {\n            tls\n            tls_server_name {{.Source}}\n        }\n    }\n}",
				Variables:   []string{"Source"},
			},
			{
				Name:        "自定义错误转换",
				Description: "使用自定义的 502 错误页面",
				Content:     "{{.Origin}} {\n    {{.Cert}}\n    reverse_proxy {{.Source}}\n    handle_errors {\n        @badgateway expression `{err.status_code} == 502`\n        handle @badgateway {\n            rewrite * /custom_502.html\n            file_server {\n                status 500\n            }\n        }\n    }\n}",
				Variables:   []string{"Source"},
			},
		}).Error; err != nil {
			return fmt.Errorf("failed to create initial templates: %w", err)
		}
	}

	// 已有数据或全部导入成功
	return nil
}
