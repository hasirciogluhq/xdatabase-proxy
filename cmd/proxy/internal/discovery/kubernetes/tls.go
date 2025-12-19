package kubernetes

import (
	"context"
	"crypto/tls"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type K8sTLSProvider struct {
	clientset  *kubernetes.Clientset
	namespace  string
	secretName string
}

func NewK8sTLSProvider(clientset *kubernetes.Clientset, namespace, secretName string) *K8sTLSProvider {
	return &K8sTLSProvider{
		clientset:  clientset,
		namespace:  namespace,
		secretName: secretName,
	}
}

func (p *K8sTLSProvider) GetCertificate(ctx context.Context) (*tls.Certificate, error) {
	secret, err := p.clientset.CoreV1().Secrets(p.namespace).Get(ctx, p.secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", p.namespace, p.secretName, err)
	}

	certBytes, ok := secret.Data[corev1.TLSCertKey]
	if !ok {
		return nil, fmt.Errorf("secret missing %s", corev1.TLSCertKey)
	}
	keyBytes, ok := secret.Data[corev1.TLSPrivateKeyKey]
	if !ok {
		return nil, fmt.Errorf("secret missing %s", corev1.TLSPrivateKeyKey)
	}

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse x509 key pair: %w", err)
	}

	return &cert, nil
}

func (p *K8sTLSProvider) Store(ctx context.Context, certPEM, keyPEM []byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.secretName,
			Namespace: p.namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       certPEM,
			corev1.TLSPrivateKeyKey: keyPEM,
		},
	}

	_, err := p.clientset.CoreV1().Secrets(p.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		// If it already exists, update it
		if _, updateErr := p.clientset.CoreV1().Secrets(p.namespace).Update(ctx, secret, metav1.UpdateOptions{}); updateErr != nil {
			return fmt.Errorf("failed to create or update secret %s/%s: %v (create err: %v)", p.namespace, p.secretName, updateErr, err)
		}
	}
	return nil
}
