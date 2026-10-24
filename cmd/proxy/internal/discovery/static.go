// Package discovery maps a (deployment, pooled) route to a backend address.
package discovery

import (
	"context"
	"fmt"
	"strings"
)

// Static resolves routes from a fixed table.
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

func (s Static) Resolve(_ context.Context, deployment string, pooled bool) (string, error) {
	key := deployment
	if pooled {
		key += ".pool"
	}
	if addr, ok := s[key]; ok {
		return addr, nil
	}
	return "", fmt.Errorf("no backend for %q", key)
}
