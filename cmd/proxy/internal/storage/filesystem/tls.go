package filesystem

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
)

type FileTLSProvider struct {
	CertFile string
	KeyFile  string
}

func NewFileTLSProvider(certFile, keyFile string) *FileTLSProvider {
	return &FileTLSProvider{
		CertFile: certFile,
		KeyFile:  keyFile,
	}
}

func (p *FileTLSProvider) GetCertificate(ctx context.Context) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(p.CertFile, p.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load key pair from %s, %s: %w", p.CertFile, p.KeyFile, err)
	}
	return &cert, nil
}

func (p *FileTLSProvider) Store(ctx context.Context, certPEM, keyPEM []byte) error {
	if err := os.WriteFile(p.CertFile, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to write cert file: %w", err)
	}
	if err := os.WriteFile(p.KeyFile, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}
	return nil
}
