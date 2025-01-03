package constants

const (
	CertPathPrefix           = "/data/cdn/certs/"
	CertPathDir              = CertPathPrefix + "%d/" // %d -> cert id
	CertPathCertName         = "cert.pem"
	CertPathIntermediateName = "ca.pem"
	CertPathKeyName          = "key.pem"
)
