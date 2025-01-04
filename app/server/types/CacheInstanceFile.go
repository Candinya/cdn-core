package types

type CacheInstanceFileType int

const (
	CacheInstanceFileAdditionalFile CacheInstanceFileType = iota
	CacheInstanceFileCert
)

type CacheInstanceFileSubtype int

const (
	CacheInstanceFileSubtypeCertCertificate CacheInstanceFileSubtype = iota
	CacheInstanceFileSubtypeCertPrivateKey
	CacheInstanceFileSubtypeCertIntermediate
)

type CacheInstanceFile struct {
	Type    CacheInstanceFileType    `json:"type"`
	Subtype CacheInstanceFileSubtype `json:"subtype,omitempty"`
	ID      uint                     `json:"id"` // File or cert ID
}
