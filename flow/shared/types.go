package shared

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	MirrorNameKey ContextKey = "mirror_name"
	FlowNameKey   ContextKey = "flow_name"
)

// CatalogPool wraps a pgxpool.Pool for catalog database access
type CatalogPool struct {
	Pool *pgxpool.Pool
}

// BeginTx starts a new transaction
func (c *CatalogPool) BeginTx(ctx context.Context, opts interface{}) (interface{}, error) {
	return c.Pool.Begin(ctx)
}

// PGVersion represents PostgreSQL version
type PGVersion int

const (
	POSTGRES_12 PGVersion = 120000
	POSTGRES_13 PGVersion = 130000
	POSTGRES_14 PGVersion = 140000
	POSTGRES_15 PGVersion = 150000
	POSTGRES_16 PGVersion = 160000
)

// CustomDataType represents a custom PostgreSQL data type
type CustomDataType struct {
	OID   uint32
	Name  string
	Delim string
}

// CreateTlsConfig creates a TLS configuration
func CreateTlsConfig(minVersion uint16, rootCA []byte, serverName string, tlsHost string, skipVerify bool) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion:         minVersion,
		InsecureSkipVerify: skipVerify,
	}

	if serverName != "" {
		tlsConfig.ServerName = serverName
	}
	if tlsHost != "" {
		tlsConfig.ServerName = tlsHost
	}

	if rootCA != nil {
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(rootCA) {
			return nil, fmt.Errorf("failed to add CA certificate")
		}
		tlsConfig.RootCAs = certPool
	}

	return tlsConfig, nil
}

// GetMajorVersion returns the major PostgreSQL version
func GetMajorVersion(ctx context.Context, conn interface{}) (PGVersion, error) {
	// Implementation would query server_version_num
	return POSTGRES_16, nil
}

// Ptr returns a pointer to the given value
func Ptr[T any](v T) *T {
	return &v
}

// ReplaceIllegalCharactersWithUnderscores replaces characters that are not allowed in identifiers
func ReplaceIllegalCharactersWithUnderscores(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result[i] = c
		} else {
			result[i] = '_'
		}
	}
	return string(result)
}
