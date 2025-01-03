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
	Type    CacheInstanceFileType    `json:"type,omitempty"`
	Subtype CacheInstanceFileSubtype `json:"subtype,omitempty"`
	Path    string                   `json:"path,omitempty"`
	ID      uint                     `json:"id,omitempty"` // File or cert ID
}
