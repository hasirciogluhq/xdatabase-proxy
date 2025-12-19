package memory

import (
	"context"
	"crypto/tls"
	"os"
	"sync"
)

// MemoryTLSProvider is a simple in-memory implementation for development
type MemoryTLSProvider struct {
	cert *tls.Certificate
	mu   sync.RWMutex
}

func NewMemoryTLSProvider() *MemoryTLSProvider {
	return &MemoryTLSProvider{}
}

func (p *MemoryTLSProvider) GetCertificate(ctx context.Context) (*tls.Certificate, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cert == nil {
		return nil, os.ErrNotExist
	}
	return p.cert, nil
}

func (p *MemoryTLSProvider) Store(ctx context.Context, certPEM, keyPEM []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	p.cert = &cert
	return nil
}
