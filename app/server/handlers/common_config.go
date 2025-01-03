package handlers

import (
	"bytes"
	"caddy-delivery-network/app/server/constants"
	"caddy-delivery-network/app/server/models"
	"fmt"
	"go.uber.org/zap"
	"strings"
	"text/template"
)

func (a *App) buildInstanceConfig(instanceID uint) (string, error) {
	// 寻找 instance
	var instance models.Instance
	if err := a.db.First(&instance, "id = ?", instanceID).Error; err != nil {
		a.l.Error("failed to find instance with id", zap.Uint("instanceID", instanceID), zap.Error(err))
		return "", fmt.Errorf("failed to find instance with id %d: %w", instanceID, err)
	}

	// 添加 preconfig 内容
	configSections := []string{instance.PreConfig}

	// 依次添加站点
	for _, siteID := range instance.SiteIDs {
		siteConfig, err := a.buildSiteConfig(siteID)
		if err != nil {
			a.l.Error("failed to build site config", zap.Uint("siteID", siteID), zap.Error(err))
			return "", fmt.Errorf("failed to build site config %d: %w", siteID, err)
		}
		configSections = append(configSections, siteConfig)
	}

	// 连接所有内容
	return strings.Join(configSections, "\n\n"), nil
}

func (a *App) buildSiteConfig(siteID uint) (string, error) {
	// 寻找 site
	var site models.Site
	if err := a.db.Model(&models.Site{}).
		Preload("Cert").
		Preload("Template").
		First(&site, "id = ?", siteID).Error; err != nil {
		a.l.Error("failed to find site with id", zap.Uint("siteID", siteID), zap.Error(err))
		return "", fmt.Errorf("failed to find site with id %d: %w", siteID, err)
	}

	// 准备模板
	siteTemplate, err := template.New(fmt.Sprintf("site-%d", siteID)).Parse(site.Template.Content)
	if err != nil {
		a.l.Error("failed to parse site template", zap.Uint("siteID", siteID), zap.Error(err))
		return "", fmt.Errorf("failed to parse site template %d: %w", siteID, err)
	}

	// 准备数据
	data := make(map[string]string)

	// 添加保留字段
	data["Origin"] = site.Origin
	if site.Cert != nil {
		certPathPrefix := fmt.Sprintf(constants.CertPathDir, site.CertID)

		// 添加基础信息
		tlsConfig := fmt.Sprintf(
			"tls %s %s",
			certPathPrefix+constants.CertPathCertName,
			certPathPrefix+constants.CertPathKeyName,
		)

		// 添加中间证书
		if site.Cert.IntermediateCertificate != nil {
			tlsConfig += fmt.Sprintf(
				" {\n        ca_root %s\n    }",
				certPathPrefix+constants.CertPathIntermediateName,
			)
		}

		data["Cert"] = tlsConfig
	}

	// 添加自定义字段
	for index, fieldName := range site.Template.Variables {
		data[fieldName] = site.TemplateValues[index]
	}

	// 应用模板
	var buf bytes.Buffer
	if err := siteTemplate.Execute(&buf, data); err != nil {
		a.l.Error("failed to execute site template", zap.Uint("siteID", siteID), zap.Any("data", data), zap.Error(err))
		return "", fmt.Errorf("failed to execute site template %d: %w", siteID, err)
	}

	// 成功返回
	return buf.String(), nil
}
