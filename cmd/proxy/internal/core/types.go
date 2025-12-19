package core

import (
	"context"
	"crypto/tls"
	"net"
)

// RoutingMetadata contains information extracted from the protocol handshake
// used to determine the destination backend (e.g., "database": "finance").
type RoutingMetadata map[string]string

// BackendResolver defines how to find a backend address based on metadata.
// It is purely a lookup mechanism and knows nothing about the network.
type BackendResolver interface {
	Resolve(ctx context.Context, metadata RoutingMetadata) (string, error)
}

// ProtocolHandler defines how to interpret the initial connection handshake.
// It abstracts away the specific database wire protocol (Postgres, MySQL, etc).
type ProtocolHandler interface {
	// Handshake reads the initial bytes from the connection to extract metadata.
	// It returns:
	// - extracted metadata
	// - the net.Conn to be used for the rest of the session (potentially wrapped, e.g., TLS)
	// - an error if the handshake fails
	Handshake(conn net.Conn) (RoutingMetadata, net.Conn, error)
}

// TLSProvider defines how to retrieve the server certificate.
// It abstracts away the storage mechanism (K8s Secret, File, Vault, etc.).
type TLSProvider interface {
	GetCertificate(ctx context.Context) (*tls.Certificate, error)
	Store(ctx context.Context, certPEM, keyPEM []byte) error
}
