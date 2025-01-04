package constants

// 证书文件
const (
	CertPathPrefix           = "/data/cdn/certs/"
	CertPathDir              = CertPathPrefix + "%d/" // %d -> cert id
	CertPathCertName         = "cert.pem"
	CertPathKeyName          = "key.pem"
	CertPathIntermediateName = "ca.pem"
)

// 额外文件
const (
	AFilePathPrefix = "/data/cdn/afiles/" // Additional File
)
