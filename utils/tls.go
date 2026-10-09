package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// ArrayTLSConfig is shared by REST and bulk clients so they cannot silently use
// different trust policies. Verification is enabled unless explicitly disabled.
func ArrayTLSConfig(storage Storage) (*tls.Config, error) {
	config := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: storage.TLSInsecureSkipVerify}
	if storage.TLSCAFile != "" {
		pem, err := os.ReadFile(storage.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("read array CA file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("array CA file contains no valid certificates")
		}
		config.RootCAs = pool
	}
	return config, nil
}
