package handlers

import (
	"caddy-delivery-network/app/server/gen/oapi/admin"
	"caddy-delivery-network/app/server/models"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
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
		// 忽略错误
		_ = json.Unmarshal([]byte(*req.Provider), &cert.Provider)
	}

	// 详细信息
	if req.Certificate != nil {
		cert.Certificate = *req.Certificate
		cert.ExpiresAt = a.certCalcExpiresAt(*req.Certificate)
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
		cert.IntermediateCertificate = req.IntermediateCertificate
	}
	if req.Csr != nil {
		cert.CSR = *req.Csr
	}
}

func (a *App) certCalcExpiresAt(certificate string) time.Time {
	block, _ := pem.Decode([]byte(certificate))
	if block == nil {
		a.l.Error("failed to parse certificate", zap.String("certificate", certificate))
		return time.Time{}
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		a.l.Error("failed to parse certificate", zap.String("certificate", certificate))
		return time.Time{}
	}

	return cert.NotAfter
}

func (a *App) CertCreate(c echo.Context) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.CertCreateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}

	// 创建
	var cert models.Cert
	a.certMapFields(&req, &cert)

	if err := a.db.WithContext(rctx).Create(&cert).Error; err != nil {
		a.l.Error("failed to create cert", zap.Any("cert", cert), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	certExpiresAtTs := cert.ExpiresAt.Unix()
	return c.JSON(http.StatusCreated, &admin.CertInfoWithID{
		Id:        &cert.ID,
		Name:      &cert.Name,
		Domains:   (*[]string)(&cert.Domains),
		ExpiresAt: &certExpiresAtTs,
		// 其他字段不开放
	})
}

func (a *App) CertList(c echo.Context, params admin.CertListParams) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, false, nil)
	if err != nil {
		a.l.Error("failed to auth", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	var (
		certs      []models.Cert
		certsCount int64
	)

	page, limit := a.parsePagination(params.Page, params.Limit)

	if err := a.db.WithContext(rctx).Model(&models.Cert{}).Limit(limit).Offset(page * limit).Find(&certs).Error; err != nil {
		a.l.Error("failed to get cert list", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}
	if err := a.db.WithContext(rctx).Model(&models.Cert{}).Count(&certsCount).Error; err != nil {
		a.l.Error("failed to count cert", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	pageMax := certsCount / int64(limit)
	if (certsCount % int64(limit)) != 0 {
		pageMax++
	}

	var resCerts []admin.CertInfoWithID
	for _, cert := range certs {
		resCerts = append(resCerts, admin.CertInfoWithID{
			Id:      &cert.ID,
			Name:    &cert.Name,
			Domains: (*[]string)(&cert.Domains),
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
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var cert models.Cert
	if err := a.db.WithContext(rctx).First(&cert, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get cert", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	certExpiresAtTs := cert.ExpiresAt.Unix()
	return c.JSON(http.StatusCreated, &admin.CertInfoWithID{
		Id:        &cert.ID,
		Name:      &cert.Name,
		Domains:   (*[]string)(&cert.Domains),
		ExpiresAt: &certExpiresAtTs,
	})
}

func (a *App) CertInfoUpdate(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 绑定请求体
	var req admin.CertInfoUpdateJSONRequestBody
	if err = c.Bind(&req); err != nil {
		a.l.Error("failed to bind request", zap.Error(err))
		return c.NoContent(http.StatusBadRequest)
	}

	// 从数据库中获得
	var cert models.Cert
	if err := a.db.WithContext(rctx).First(&cert, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get cert", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// 更新
	a.certMapFields(&req, &cert)

	// 更新信息
	if err := a.db.WithContext(rctx).Updates(&cert).Error; err != nil {
		a.l.Error("failed to update cert", zap.Any("cert", cert), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	certExpiresAtTs := cert.ExpiresAt.Unix()
	return c.JSON(http.StatusCreated, &admin.CertInfoWithID{
		Id:        &cert.ID,
		Name:      &cert.Name,
		Domains:   (*[]string)(&cert.Domains),
		ExpiresAt: &certExpiresAtTs,
		// 其他字段不开放
	})
}

func (a *App) CertRenew(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 从数据库中获得
	var cert models.Cert
	if err := a.db.WithContext(rctx).First(&cert, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.NoContent(http.StatusNotFound)
		} else {
			a.l.Error("failed to get cert", zap.Uint("id", id), zap.Error(err))
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	if cert.Provider == nil {
		return c.NoContent(http.StatusNotImplemented)
	}

	// todo: 调用 provider 处理

	// 更新信息
	//if err := a.db.WithContext(rctx).Model(&cert).Update("content", newContentBytes).Error; err != nil {
	//	a.l.Error("failed to update cert", zap.Any("cert", cert), zap.Error(err))
	//	return c.NoContent(http.StatusInternalServerError)
	//}

	certExpiresAtTs := cert.ExpiresAt.Unix()
	return c.JSON(http.StatusCreated, &admin.CertInfoWithID{
		Id:        &cert.ID,
		Name:      &cert.Name,
		Domains:   (*[]string)(&cert.Domains),
		ExpiresAt: &certExpiresAtTs,
		// 其他字段不开放
	})
}

func (a *App) CertDelete(c echo.Context, id uint) error {
	// 抓取 user 信息（认证）
	err, statusCode := a.authAdmin(c, true, nil)
	if err != nil {
		a.l.Error("failed to get user", zap.Error(err))
		return c.NoContent(statusCode)
	}

	rctx := c.Request().Context()

	// 删除
	if err := a.db.WithContext(rctx).Delete(&models.Cert{}, id).Error; err != nil {
		a.l.Error("failed to delete cert", zap.Uint("id", id), zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
