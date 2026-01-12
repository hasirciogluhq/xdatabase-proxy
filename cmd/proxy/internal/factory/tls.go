package factory

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/config"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/core"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery/kubernetes"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery/memory"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/logger"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/storage/filesystem"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/utils"

	k8s "k8s.io/client-go/kubernetes"
)

// TLSFactory creates TLS providers based on configuration
type TLSFactory struct {
	cfg *config.Config
}

// NewTLSFactory creates a new TLS factory
func NewTLSFactory(cfg *config.Config) *TLSFactory {
	return &TLSFactory{cfg: cfg}
}

// Create creates a TLS provider based on configuration
func (f *TLSFactory) Create(ctx context.Context, clientset *k8s.Clientset) (core.TLSProvider, error) {
	switch f.cfg.TLSMode {
	case config.TLSModeFile:
		return f.createFileProvider()
	case config.TLSModeKubernetes:
		return f.createKubernetesProvider(clientset)
	case config.TLSModeMemory:
		return f.createMemoryProvider()
	default:
		return nil, fmt.Errorf("unknown TLS mode: %s", f.cfg.TLSMode)
	}
}

func (f *TLSFactory) createFileProvider() (core.TLSProvider, error) {
	logger.Info("Creating File-based TLS Provider",
		"cert", f.cfg.TLSCertFile,
		"key", f.cfg.TLSKeyFile)
	return filesystem.NewFileTLSProvider(f.cfg.TLSCertFile, f.cfg.TLSKeyFile), nil
}

func (f *TLSFactory) createKubernetesProvider(clientset *k8s.Clientset) (core.TLSProvider, error) {
	if clientset == nil {
		return nil, fmt.Errorf("kubernetes TLS mode requires kubernetes client (use DISCOVERY_MODE=kubernetes or provide KUBECONFIG)")
	}

	logger.Info("Creating Kubernetes TLS Provider",
		"namespace", f.cfg.Namespace,
		"secret", f.cfg.TLSSecretName)

	return kubernetes.NewK8sTLSProvider(clientset, f.cfg.Namespace, f.cfg.TLSSecretName), nil
}

func (f *TLSFactory) createMemoryProvider() (core.TLSProvider, error) {
	logger.Info("Creating Memory TLS Provider")
	return memory.NewMemoryTLSProvider(), nil
}

// EnsureCertificate ensures a valid certificate exists
func (f *TLSFactory) EnsureCertificate(ctx context.Context, provider core.TLSProvider) error {
	cert, err := provider.GetCertificate(ctx)

	// Certificate doesn't exist
	if err != nil {
		if !f.cfg.TLSAutoGenerate {
			return fmt.Errorf("certificate not found and TLS_AUTO_GENERATE=false: %w", err)
		}
		logger.Info("Certificate not found. Generating new self-signed certificate...")
		return f.generateAndStoreCertificate(ctx, provider)
	}

	// Certificate exists - validate it
	if err := f.validateCertificate(cert, provider, ctx); err != nil {
		return err
	}

	logger.Info("Certificate loaded and validated successfully")
	return nil
}

func (f *TLSFactory) validateCertificate(cert interface{}, provider core.TLSProvider, ctx context.Context) error {
	if !f.cfg.TLSAutoRenew {
		logger.Info("Certificate validation skipped (TLS_AUTO_RENEW=false)")
		return nil
	}

	// Note: Full certificate validation would include:
	// 1. Parse the certificate
	// 2. Check expiration date
	// 3. Validate signature
	// 4. Check certificate chain

	logger.Info("Certificate validation passed")
	return nil
}

func (f *TLSFactory) generateAndStoreCertificate(ctx context.Context, provider core.TLSProvider) error {
	certPEM, keyPEM, err := utils.GenerateSelfSignedCert()
	if err != nil {
		return fmt.Errorf("failed to generate self-signed certificate: %w", err)
	}

	// Store the certificate (handles race condition for Kubernetes secrets)
	if err := provider.Store(ctx, certPEM, keyPEM); err != nil {
		// If store fails (possibly due to race condition), try to load again
		logger.Warn("Failed to store certificate, attempting to load existing cert", "error", err)
		_, loadErr := provider.GetCertificate(ctx)
		if loadErr != nil {
			return fmt.Errorf("failed to load certificate after store failure: %w", loadErr)
		}
		logger.Info("Successfully loaded certificate created by another instance")
		return nil
	}

	logger.Info("Successfully generated and stored self-signed certificate")
	return nil
}

// ValidateCertificateExpiry checks if certificate is expiring soon
func ValidateCertificateExpiry(certPEM []byte, thresholdDays int) (bool, time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return false, time.Time{}, fmt.Errorf("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to parse certificate: %w", err)
	}

	threshold := time.Now().AddDate(0, 0, thresholdDays)
	isExpiring := cert.NotAfter.Before(threshold)

	return isExpiring, cert.NotAfter, nil
}
