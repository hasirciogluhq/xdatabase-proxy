// Package discovery maps a routing key to a backend address.
package discovery

import (
	"context"
	"fmt"
	"strings"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/route"
)

// Static resolves routes from a fixed table. It allows every strategy.
type Static map[string]string

// NewStatic parses "deployment[.pool]=host:port,...".
func NewStatic(spec string) (Static, error) {
	s := Static{}
	for _, pair := range strings.Split(spec, ",") {
		if strings.TrimSpace(pair) == "" {
			continue
		}
		k, addr, ok := strings.Cut(pair, "=")
		k, addr = strings.TrimSpace(k), strings.TrimSpace(addr)
		if !ok || k == "" || addr == "" {
			return nil, fmt.Errorf("invalid STATIC_BACKENDS entry %q (want deployment[.pool]=host:port)", pair)
		}
		s[k] = addr
	}
	return s, nil
}

func (s Static) Resolve(_ context.Context, k route.Key, _ route.Strategy) (string, error) {
	name := k.Deployment
	if k.Pooled {
		name += ".pool"
	}
	if addr, ok := s[name]; ok {
		return addr, nil
	}
	return "", fmt.Errorf("no backend for %q", name)
}
