package postgresql_proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/core"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/logger"
)

const (
	sslRequestCode = 80877103
)

// ErrorResponse represents a PostgreSQL error response
type ErrorResponse struct {
	Severity string
	Code     string
	Message  string
}

type PostgresProxy struct {
	TLSConfig *tls.Config
	Resolver  core.BackendResolver
}

func (p *PostgresProxy) sendErrorResponse(conn net.Conn, errResp *ErrorResponse) error {
	var msgData []byte
	msgData = append(msgData, 'S')
	msgData = append(msgData, []byte(errResp.Severity)...)
	msgData = append(msgData, 0)
	msgData = append(msgData, 'C')
	msgData = append(msgData, []byte(errResp.Code)...)
	msgData = append(msgData, 0)
	msgData = append(msgData, 'M')
	msgData = append(msgData, []byte(errResp.Message)...)
	msgData = append(msgData, 0)
	msgData = append(msgData, 0) // Final null terminator

	msg := make([]byte, 1+4+len(msgData))
	msg[0] = 'E'
	binary.BigEndian.PutUint32(msg[1:5], uint32(4+len(msgData)))
	copy(msg[5:], msgData)

	_, writeErr := conn.Write(msg)
	if writeErr != nil {
		logger.Error("Error sending error response", "remote_addr", conn.RemoteAddr(), "error", writeErr)
	} else {
		logger.Info("Sent error response", "remote_addr", conn.RemoteAddr(), "severity", errResp.Severity, "code", errResp.Code, "message", errResp.Message)
	}
	return writeErr
}

// HandleConnection implements core.ConnectionHandler.
// It takes full ownership of the connection lifecycle.
func (p *PostgresProxy) HandleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	// 1. Handshake & Protocol Parsing
	metadata, clientConn, rawStartupMsg, err := p.handshake(clientConn)
	if err != nil {
		logger.Error("Handshake failed", "error", err, "remote_addr", clientConn.RemoteAddr())
		// Try to send error response if possible, but handshake error might mean we can't speak protocol
		return
	}

	// 2. Resolve Backend
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	backendAddr, err := p.Resolver.Resolve(ctx, metadata, core.DatabaseTypePostgresql)
	if err != nil {
		logger.Error("Resolution failed", "error", err, "remote_addr", clientConn.RemoteAddr())
		_ = p.sendErrorResponse(clientConn, &ErrorResponse{
			Severity: "FATAL",
			Code:     "08001", // sqlclient_unable_to_establish_sqlconnection
			Message:  fmt.Sprintf("resolution failed: %v", err),
		})
		return
	}

	// 3. Dial Backend
	backendConn, err := net.Dial("tcp", backendAddr)
	if err != nil {
		logger.Error("Dial failed", "backend_addr", backendAddr, "error", err, "remote_addr", clientConn.RemoteAddr())
		_ = p.sendErrorResponse(clientConn, &ErrorResponse{
			Severity: "FATAL",
			Code:     "08001",
			Message:  fmt.Sprintf("failed to connect to backend %s: %v", backendAddr, err),
		})
		return
	}
	defer backendConn.Close()

	// 4. Forward Startup Message
	if _, err := backendConn.Write(rawStartupMsg); err != nil {
		logger.Error("Failed to forward startup message", "error", err, "remote_addr", clientConn.RemoteAddr())
		return
	}

	// 5. Pipe Data
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(backendConn, clientConn)
	}()

	go func() {
		defer wg.Done()
		io.Copy(clientConn, backendConn)
	}()

	wg.Wait()
}

// handshake performs the initial protocol handshake and returns metadata, the (potentially wrapped) connection, and the raw startup message bytes.
func (p *PostgresProxy) handshake(conn net.Conn) (core.RoutingMetadata, net.Conn, []byte, error) {
	// Read message length (4 bytes)
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read message length: %w", err)
	}

	length := int32(binary.BigEndian.Uint32(header))
	if length < 4 {
		return nil, nil, nil, fmt.Errorf("invalid message length: %d", length)
	}

	// Read message body
	payload := make([]byte, length-4)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read message body: %w", err)
	}

	// Check for SSLRequest
	if len(payload) >= 4 {
		code := int32(binary.BigEndian.Uint32(payload[0:4]))
		if code == sslRequestCode {
			// Check if TLS is configured
			if p.TLSConfig == nil {
				// Send 'N' to reject SSL (TLS disabled)
				if _, err := conn.Write([]byte{'N'}); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to write SSL rejection response: %w", err)
				}
				logger.Info("SSL request rejected - TLS is disabled", "remote_addr", conn.RemoteAddr())
				// Continue reading the next message (StartupMessage without SSL)
				return p.handshake(conn)
			}

			// Send 'S' to accept SSL
			if _, err := conn.Write([]byte{'S'}); err != nil {
				return nil, nil, nil, fmt.Errorf("failed to write SSL response: %w", err)
			}

			// Upgrade connection
			tlsConn := tls.Server(conn, p.TLSConfig)
			if err := tlsConn.Handshake(); err != nil {
				_ = p.sendErrorResponse(conn, &ErrorResponse{
					Severity: "FATAL",
					Code:     "08006",
					Message:  fmt.Sprintf("TLS handshake failed: %v", err),
				})
				return nil, nil, nil, fmt.Errorf("tls handshake failed: %w", err)
			}

			state := tlsConn.ConnectionState()
			logger.Info("TLS Handshake successful",
				"protocol", tlsVersionName(state.Version),
				"cipher_suite", tls.CipherSuiteName(state.CipherSuite),
				"remote_addr", conn.RemoteAddr())

			// Recursively parse the StartupMessage from the encrypted stream
			return p.handshake(tlsConn)
		}
	}

	// Parse StartupMessage
	if len(payload) < 4 {
		return nil, nil, nil, fmt.Errorf("payload too short")
	}

	params := make(map[string]string)
	buf := bytes.NewBuffer(payload[4:]) // Skip protocol version

	for {
		key, err := buf.ReadString(0)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}
		key = key[:len(key)-1] // Trim null byte

		if key == "" {
			break
		}

		value, err := buf.ReadString(0)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("malformed startup message")
		}
		value = value[:len(value)-1] // Trim null byte

		params[key] = value
		logger.Info("StartupMessage param", "key", key, "value", value, "remote_addr", conn.RemoteAddr())
	}

	// Parse username to extract deployment_id and pool status
	// Format: username.deployment_id[.pool]
	// Examples:
	//   alice.db-prod.pool     → username=alice, deployment_id=db-prod, pooled=true
	//   bob.team-1992252154561 → username=bob, deployment_id=team-1992252154561, pooled=false
	if user, ok := params["user"]; ok {
		logger.Info("Connection requested", "user", user, "remote_addr", conn.RemoteAddr())
		parts := strings.Split(user, ".")
		if len(parts) >= 2 {
			if parts[len(parts)-1] == "pool" {
				params["pooled"] = "true"
				if len(parts) >= 3 {
					params["deployment_id"] = parts[len(parts)-2]
					params["username"] = strings.Join(parts[:len(parts)-2], ".")
				}
			} else {
				params["pooled"] = "false"
				params["deployment_id"] = parts[len(parts)-1]
				params["username"] = strings.Join(parts[:len(parts)-1], ".")
			}
		} else {
			params["pooled"] = "false"
		}
	}

	// Default database to postgres if not provided OR if it equals the original user
	// Some PostgreSQL clients (like psql) automatically use username as database when not specified
	// This causes issues when username is "postgres.team-1992252154561" and gets used as database name
	// We detect this case and default to "postgres" database instead
	originalUser := params["user"]
	if dbName, ok := params["database"]; !ok || dbName == "" || dbName == originalUser {
		params["database"] = "postgres"
		logger.Info("Database defaulted to postgres", "original_db", dbName, "remote_addr", conn.RemoteAddr())
	}

	// Always rebuild startup message with parsed params
	// Every PostgreSQL connection performs a fresh handshake, so we rebuild the StartupMessage
	// to send the correct username (without deployment_id/pool suffix) and database to the backend
	protocolVersion := binary.BigEndian.Uint32(payload[0:4])
	buildParams := make(map[string]string)

	// Copy all params except internal metadata (user will be set separately)
	// Exclude: deployment_id, pooled, username (internal routing metadata)
	// Include: database, client_encoding, application_name, etc.
	for k, v := range params {
		if k != "deployment_id" && k != "pooled" && k != "username" && k != "user" {
			buildParams[k] = v
		}
	}

	// Use parsed username (without deployment_id suffix) or fallback to original
	// Backend expects: "alice" not "alice.db-prod.pool"
	if username, ok := params["username"]; ok && username != "" {
		buildParams["user"] = username
		logger.Info("Using parsed username", "username", username, "database", buildParams["database"], "remote_addr", conn.RemoteAddr())
	} else if originalUser, ok := params["user"]; ok {
		buildParams["user"] = originalUser
		logger.Info("Using original username", "user", originalUser, "database", buildParams["database"], "remote_addr", conn.RemoteAddr())
	}

	// Rebuild the binary StartupMessage packet with modified parameters
	rawStartupMsg := rebuildStartupMessage(protocolVersion, buildParams)
	return core.RoutingMetadata(params), conn, rawStartupMsg, nil
}

func rebuildStartupMessage(protocolVersion uint32, params map[string]string) []byte {
	// Calculate total length needed
	totalLength := 4 + 4 // Length field + protocol version
	for key, value := range params {
		totalLength += len(key) + 1 + len(value) + 1
	}
	totalLength++ // Final null byte

	newMessage := make([]byte, totalLength)
	binary.BigEndian.PutUint32(newMessage[0:4], uint32(totalLength))
	binary.BigEndian.PutUint32(newMessage[4:8], protocolVersion)

	offset := 8
	for key, value := range params {
		copy(newMessage[offset:], key)
		offset += len(key)
		newMessage[offset] = 0
		offset++
		copy(newMessage[offset:], value)
		offset += len(value)
		newMessage[offset] = 0
		offset++
	}
	newMessage[offset] = 0
	return newMessage
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLSv1.0"
	case tls.VersionTLS11:
		return "TLSv1.1"
	case tls.VersionTLS12:
		return "TLSv1.2"
	case tls.VersionTLS13:
		return "TLSv1.3"
	default:
		return fmt.Sprintf("Unknown (%x)", version)
	}
}
