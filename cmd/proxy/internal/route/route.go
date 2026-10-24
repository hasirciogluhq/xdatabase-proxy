// Package route defines how a client connection is mapped to a backend:
// the routing key, the strategies that can produce it and the resolver contract.
package route

import (
	"context"
	"strings"
)

// Strategy says where the routing key came from.
type Strategy string

const (
	// Username: the login carries the deployment, "user.deployment[.pool]".
	Username Strategy = "username"
	// SNI: the TLS server name carries it, "deployment[.pool].example.com".
	SNI Strategy = "sni"
)

// Key identifies a backend: a deployment and whether the pooled endpoint is wanted.
type Key struct {
	Deployment string
	Pooled     bool
}

// Resolver finds the backend address for a key. via is the strategy that produced
// the key, so a resolver can refuse strategies the operator did not enable.
type Resolver interface {
	Resolve(ctx context.Context, k Key, via Strategy) (string, error)
}

// ParseLogin splits "name.deployment[.pool]". Without a deployment part it
// returns the whole login as name and an empty key.
func ParseLogin(user string) (name string, k Key) {
	parts := strings.Split(user, ".")
	if n := len(parts); n >= 3 && parts[n-1] == "pool" {
		return strings.Join(parts[:n-2], "."), Key{parts[n-2], true}
	} else if n >= 2 && parts[n-1] != "pool" {
		return strings.Join(parts[:n-1], "."), Key{parts[n-1], false}
	}
	return user, Key{}
}

// ParseHost maps "db1.example.com" to db1 and "db1.pool.example.com" to db1 pooled.
func ParseHost(host string) Key {
	labels := strings.Split(strings.ToLower(strings.TrimSuffix(host, ".")), ".")
	return Key{labels[0], len(labels) > 2 && labels[1] == "pool"}
}
