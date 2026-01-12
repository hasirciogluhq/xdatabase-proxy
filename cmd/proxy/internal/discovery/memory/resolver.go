package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/core"
)

type Resolver struct {
	backends map[string]string
	mu       sync.RWMutex
}

// NewResolver creates a new memory resolver from a comma-separated string
// Format: "deployment_id[.pool]=host:port,..."
// Example: "db1=localhost:5432,db1.pool=localhost:6432"
func NewResolver(mappingStr string) (*Resolver, error) {
	backends := make(map[string]string)
	if mappingStr == "" {
		return &Resolver{backends: backends}, nil
	}

	pairs := strings.Split(mappingStr, ",")
	for _, pair := range pairs {
		parts := strings.Split(strings.TrimSpace(pair), "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mapping format: %s", pair)
		}
		key := strings.TrimSpace(parts[0])
		addr := strings.TrimSpace(parts[1])
		backends[key] = addr
	}

	return &Resolver{backends: backends}, nil
}

func (r *Resolver) Resolve(ctx context.Context, metadata core.RoutingMetadata, databaseType core.DatabaseType) (string, error) {
	deploymentID, ok := metadata["deployment_id"]
	if !ok {
		return "", fmt.Errorf("metadata missing 'deployment_id'")
	}
	pooled := metadata["pooled"]

	// Construct lookup key: deployment_id or deployment_id.pool
	key := deploymentID
	if pooled == "true" {
		key = deploymentID + ".pool"
	}

	r.mu.RLock()
	addr, ok := r.backends[key]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("backend not found for key: %s", key)
	}

	fmt.Printf("MemoryResolver: Routing %s (pooled=%s) to %s\n", deploymentID, pooled, addr)
	return addr, nil
}
