package handlers

import (
	"caddy-delivery-network/app/server/constants"
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/models"
	"caddy-delivery-network/app/server/utils"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
	"time"
)

func (a *App) certMapFields(req *admin.CertInfoInput, cert *models.Cert) {
	// 基础信息
	if req.Name != nil {
		cert.Name = *req.Name
	}
	if req.Domains != nil {
		cert.Domains = *req.Domains
	}
	if req.Provider != nil {
		if err := json.Unmarshal([]byte(*req.Provider), &cert.Provider); err != nil {
			a.l.Error("provider unmarshal failed", zap.Error(err))
		}
	}

	// 详细信息
	if req.Certificate != nil {
		cert.Certificate = *req.Certificate
		cert.ExpiresAt, cert.Domains = a.certParseMeta(*req.Certificate) // 优先级更高，会覆盖即使提交了域名的证书
	}
	if req.PrivateKey != nil {
		encryptedKey, err := a.aesEncrypt([]byte(*req.PrivateKey))
		if err != nil {
			a.l.Error("failed to encrypt private key", zap.Error(err))
		} else {
			cert.PrivateKey = encryptedKey
		}
	}
	if req.IntermediateCertificate != nil {
		cert.IntermediateCertificate = *req.IntermediateCertificate
	}
	if req.Csr != nil {
		cert.CSR = *req.Csr
	}
}

func (a *App) certParseMeta(certificate string) (time.Time, []string) {
	block, _ := pem.Decode([]byte(certificate))
	if block == nil {
		a.l.Error("failed to parse certificate", zap.String("certificate", certificate))
		return time.Time{}, nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		a.l.Error("failed to parse certificate", zap.String("certificate", certificate))
		return time.Time{}, nil
	}

	return cert.NotAfter, cert.DNSNames
}

func (a *App) certUpdateClearCache(ctx context.Context, id uint, isCACertStatusChanged bool) error {
	// 寻找使用了这张证书的站点
	var sites []models.Site
	if err := a.db.WithContext(ctx).Find(&sites, "cert_id = ?", id).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			a.l.Error("failed to get sites", zap.Error(err))
			return fmt.Errorf("failed to get sites: %w", err)
		}
	}

	// 再对于每个站点寻找部署了这个站点的实例
	for _, site := range sites {
		var instances []models.Instance
		if err := a.db.WithContext(ctx).
			Find(&instances, "? = ANY(site_ids)", site.ID).
			Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				// 出问题了
				a.l.Error("failed to get instances", zap.Error(err))
				return fmt.Errorf("failed to get instances: %w", err)
			}
		}
		for _, instance := range instances {
			// 清理心跳数据缓存（这里包含了文件对应的更新时间）
			heartbeatCacheKey := fmt.Sprintf(constants.CacheKeyInstanceHeartbeat, instance.ID)
			a.rdb.Del(ctx, heartbeatCacheKey)

			// 如果出现中间证书状态的更新，需要清理文件列表缓存
			if isCACertStatusChanged {
				filesCacheKey := fmt.Sprintf(constants.CacheKeyInstanceFiles, instance.ID)
				a.rdb.Del(ctx, filesCacheKey)
			}
		}
	}

	return nil
}

func (a *App) certCheckAbleToDelete(ctx context.Context, id uint) (bool, error) {
	var siteCount int64
	if err := a.db.WithContext(ctx).
		Model(&models.Site{}).
		Where("cert_id = ?", id).
		Count(&siteCount).
		Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			// 出问题了
			a.l.Error("failed to get sites", zap.Error(err))
			return false, fmt.Errorf("failed to get sites: %w", err)
		}
	}

	return siteCount == 0, nil
}

func (a *App) CertCreate(c echo.Context) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.CertCreateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 创建
	var cert models.Cert
	a.certMapFields(&req, &cert)

	if err := a.db.WithContext(rctx).Create(&cert).Error; err != nil {
		a.l.Error("failed to create cert", zap.Any("cert", cert), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, &admin.CertInfoWithID{
		Id:        &cert.ID,
		Name:      &cert.Name,
		Domains:   (*[]string)(&cert.Domains),
		ExpiresAt: utils.P(cert.ExpiresAt.Unix()),
		// 其他字段不开放
	})
}

func (a *App) CertList(c echo.Context, params admin.CertListParams) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	var (
		certs      []models.Cert
		certsCount int64
	)

	showAll, page, limit := a.parsePagination(params.Page, params.Limit)
	queryBase := a.db.WithContext(rctx).Model(&models.Cert{}).Order("id ASC")
	if !showAll {
		queryBase = queryBase.Limit(limit).Offset(page * limit)
	}

	if err := queryBase.Find(&certs).Error; err != nil {
		a.l.Error("failed to get cert list", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}
	if err := a.db.WithContext(rctx).Model(&models.Cert{}).Count(&certsCount).Error; err != nil {
		a.l.Error("failed to count cert", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	pageMax := certsCount / int64(limit)
	if (certsCount % int64(limit)) != 0 {
		pageMax++
	}

	resCerts := []admin.CertInfoWithID{}
	for _, cert := range certs {
		resCerts = append(resCerts, admin.CertInfoWithID{
			Id:           &cert.ID,
			Name:         &cert.Name,
			Domains:      (*[]string)(&cert.Domains),
			ExpiresAt:    utils.P(cert.ExpiresAt.Unix()),
			IsManualMode: utils.P(cert.Provider == nil),
		})
	}

	return c.JSON(http.StatusOK, &admin.CertListResponse{
		Limit:   &limit,
		PageMax: &pageMax,
		List:    &resCerts,
	})
}

func (a *App) CertInfoGet(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var cert models.Cert
	if err := a.db.WithContext(rctx).First(&cert, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get cert", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	return c.JSON(http.StatusOK, &admin.CertInfoWithID{
		Id:           &cert.ID,
		Name:         &cert.Name,
		Domains:      (*[]string)(&cert.Domains),
		ExpiresAt:    utils.P(cert.ExpiresAt.Unix()),
		IsManualMode: utils.P(cert.Provider == nil),
	})
}

func (a *App) CertInfoUpdate(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.CertInfoUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return a.er(c, http.StatusBadRequest)
	}

	// 从数据库中获得
	var cert models.Cert
	if err := a.db.WithContext(rctx).First(&cert, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get cert", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 如果证书部分发生变更，需要清理旧缓存（已知私钥会随着证书变化，所以没必要单独验证）
	if req.Certificate != nil && *req.Certificate != cert.Certificate ||
		req.IntermediateCertificate != nil && *req.IntermediateCertificate != cert.IntermediateCertificate {
		if err := a.certUpdateClearCache(rctx, cert.ID,
			(req.IntermediateCertificate == nil || *req.IntermediateCertificate == "") != // 新 CA 证书为空
				(cert.IntermediateCertificate == ""), // 旧 CA 证书为空
		); err != nil {
			a.l.Error("failed to clear cache", zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	// 更新
	a.certMapFields(&req, &cert)

	// 更新信息
	if err := a.db.WithContext(rctx).Updates(&cert).Error; err != nil {
		a.l.Error("failed to update cert", zap.Any("cert", cert), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	if req.IsManualMode != nil {
		// 检查是否存在模式变更
		if *req.IsManualMode && cert.Provider != nil {
			// 切换成手动模式，即清理 provider 选项
			if err := a.db.WithContext(rctx).Model(&cert).Update("provider", nil).Error; err != nil {
				a.l.Error("failed to update cert mode", zap.Any("cert", cert), zap.Error(err))
				return a.er(c, http.StatusInternalServerError)
			}
			cert.Provider = nil
		} // 如果是手动模式变自动，会设置 provider 参数，就不用特判
	}

	return c.JSON(http.StatusOK, &admin.CertInfoWithID{
		Id:           &cert.ID,
		Name:         &cert.Name,
		Domains:      (*[]string)(&cert.Domains),
		ExpiresAt:    utils.P(cert.ExpiresAt.Unix()),
		IsManualMode: utils.P(cert.Provider == nil),
		// 其他字段不开放
	})
}

func (a *App) CertRenew(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var cert models.Cert
	if err := a.db.WithContext(rctx).First(&cert, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a.er(c, http.StatusNotFound)
		} else {
			a.l.Error("failed to get cert", zap.Uint("id", id), zap.Error(err))
			return a.er(c, http.StatusInternalServerError)
		}
	}

	if cert.Provider == nil {
		return a.er(c, http.StatusNotImplemented)
	}

	// todo: 调用 provider 处理

	// 清理缓存
	if err := a.certUpdateClearCache(rctx, cert.ID, false); err != nil {
		a.l.Error("failed to clear cache", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	// 更新信息
	//if err := a.db.WithContext(rctx).Model(&cert).Update("content", newContentBytes).Error; err != nil {
	//	a.l.Error("failed to update cert", zap.Any("cert", cert), zap.Error(err))
	//	return a.er(c,http.StatusInternalServerError)
	//}

	return c.JSON(http.StatusOK, &admin.CertInfoWithID{
		Id:        &cert.ID,
		Name:      &cert.Name,
		Domains:   (*[]string)(&cert.Domains),
		ExpiresAt: utils.P(cert.ExpiresAt.Unix()),
		// 其他字段不开放
	})
}

func (a *App) CertDelete(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return a.er(c, statusCode)
	}

	rctx := c.Request().Context()

	// 检查是否可以被删除
	if ableToDelete, err := a.certCheckAbleToDelete(rctx, id); err != nil {
		a.l.Error("failed to check able-to-delete", zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	} else if !ableToDelete {
		return a.er(c, http.StatusPreconditionFailed)
	}

	// 删除
	if err := a.db.WithContext(rctx).Delete(&models.Cert{}, id).Error; err != nil {
		a.l.Error("failed to delete cert", zap.Uint("id", id), zap.Error(err))
		return a.er(c, http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
