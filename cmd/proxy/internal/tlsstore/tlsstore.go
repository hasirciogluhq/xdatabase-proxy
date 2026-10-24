// Package tlsstore loads and persists the proxy's server certificate.
package tlsstore

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/pkg/tlsutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Store is a certificate backend.
type Store interface {
	Load(ctx context.Context) (*tls.Certificate, error)
	// Save persists a certificate. With replace=false it must not overwrite
	// an existing one (so concurrent replicas converge on the first writer).
	Save(ctx context.Context, certPEM, keyPEM []byte, replace bool) error
}

// Ensure returns a usable certificate, generating (and storing) a self-signed
// one when none exists, or when our own self-signed certificate is about to expire.
func Ensure(ctx context.Context, s Store, autoGenerate bool, renewWindow time.Duration, log *slog.Logger) (*tls.Certificate, error) {
	cert, err := s.Load(ctx)
	switch {
	case err == nil && !(autoGenerate && tlsutil.IsOwnSelfSigned(cert) && tlsutil.ExpiresWithin(cert, renewWindow)):
		return cert, nil
	case err != nil && !autoGenerate:
		return nil, fmt.Errorf("load certificate (TLS_AUTO_GENERATE=false): %w", err)
	}

	log.Info("generating self-signed certificate", "renewal", err == nil)
	certPEM, keyPEM, genErr := tlsutil.SelfSigned()
	if genErr != nil {
		return nil, genErr
	}
	if saveErr := s.Save(ctx, certPEM, keyPEM, err == nil); saveErr != nil {
		log.Warn("storing certificate failed, reloading", "error", saveErr)
	}
	return s.Load(ctx)
}

// File keeps the certificate on disk.
type File struct{ CertFile, KeyFile string }

func (f File) Load(context.Context) (*tls.Certificate, error) {
	c, err := tls.LoadX509KeyPair(f.CertFile, f.KeyFile)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (f File) Save(_ context.Context, certPEM, keyPEM []byte, _ bool) error {
	if err := os.WriteFile(f.CertFile, certPEM, 0o644); err != nil {
		return err
	}
	return os.WriteFile(f.KeyFile, keyPEM, 0o600)
}

// Memory keeps the certificate for the life of the process.
type Memory struct {
	mu   sync.Mutex
	cert *tls.Certificate
}

func (m *Memory) Load(context.Context) (*tls.Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cert == nil {
		return nil, os.ErrNotExist
	}
	return m.cert, nil
}

func (m *Memory) Save(_ context.Context, certPEM, keyPEM []byte, _ bool) error {
	c, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.cert = &c
	m.mu.Unlock()
	return nil
}

// Secret keeps the certificate in a kubernetes.io/tls Secret.
type Secret struct {
	Client          kubernetes.Interface
	Namespace, Name string
}

func (s Secret) Load(ctx context.Context) (*tls.Certificate, error) {
	sec, err := s.Client.CoreV1().Secrets(s.Namespace).Get(ctx, s.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	c, err := tls.X509KeyPair(sec.Data[corev1.TLSCertKey], sec.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s Secret) Save(ctx context.Context, certPEM, keyPEM []byte, replace bool) error {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: s.Name, Namespace: s.Namespace},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{corev1.TLSCertKey: certPEM, corev1.TLSPrivateKeyKey: keyPEM},
	}
	api := s.Client.CoreV1().Secrets(s.Namespace)
	var err error
	if replace {
		_, err = api.Update(ctx, sec, metav1.UpdateOptions{})
	} else {
		_, err = api.Create(ctx, sec, metav1.CreateOptions{})
	}
	return err
}
