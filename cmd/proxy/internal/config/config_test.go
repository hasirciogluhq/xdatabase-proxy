package config

import "testing"

func TestDatabaseType(t *testing.T) {
	for _, c := range []struct {
		in, want string
		tls      bool
		wantErr  bool
	}{
		{"", "postgresql", true, false},
		{"postgresql", "postgresql", true, false},
		{"CockroachDB", "postgresql", true, false},
		{"yugabytedb", "postgresql", true, false},
		{"timescaledb", "postgresql", true, false},
		{"citus", "postgresql", true, false},
		{"greenplum", "postgresql", true, false},
		{"questdb", "postgresql", true, false},
		{"cratedb", "postgresql", true, false},
		{"mongodb", "mongodb", false, false},
		{"redis", "redis", true, false},
		{"clickhouse", "clickhouse", true, false},
		{"tls", "tls", false, false},
		{"mysql", "", false, true},
	} {
		t.Setenv("DATABASE_TYPE", c.in)
		t.Setenv("STATIC_BACKENDS", "a=b:1")
		t.Setenv("TLS_MODE", "memory")
		cfg, err := Load()
		if c.wantErr {
			if err == nil {
				t.Errorf("%q: expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if cfg.DatabaseType != c.want || cfg.TLSEnabled != c.tls {
			t.Errorf("%q: got type=%s tls=%v", c.in, cfg.DatabaseType, cfg.TLSEnabled)
		}
	}
}

func TestDefaultsAndValidation(t *testing.T) {
	t.Setenv("STATIC_BACKENDS", "")
	t.Setenv("TLS_MODE", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProxyPort != "5432" || cfg.Discovery != "kubernetes" || cfg.TLSMode != "memory" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}

	t.Setenv("TLS_MODE", "file")
	if _, err := Load(); err == nil {
		t.Error("file mode without cert paths should fail")
	}
	t.Setenv("TLS_MODE", "kubernetes")
	if _, err := Load(); err == nil {
		t.Error("kubernetes mode without secret name should fail")
	}
	t.Setenv("TLS_MODE", "bogus")
	if _, err := Load(); err == nil {
		t.Error("unknown mode should fail")
	}
}

func TestSNITLSMode(t *testing.T) {
	for _, c := range []struct {
		db, mode string
		tls      string
		wantTLS  bool
		term     bool
		wantErr  bool
	}{
		{"mongodb", "", "", false, false, false},              // passthrough: no certificate needed
		{"mongodb", "terminate", "", true, true, false},       // proxy terminates TLS
		{"mongodb", "terminate", "false", false, false, true}, // terminate needs TLS
		{"redis", "terminate", "", true, true, false},
		{"redis", "", "", true, false, false},
		{"postgresql", "terminate", "", false, false, true}, // always terminates, setting does not apply
		{"mongodb", "bogus", "", false, false, true},
	} {
		t.Setenv("DATABASE_TYPE", c.db)
		t.Setenv("SNI_TLS_MODE", c.mode)
		t.Setenv("TLS_ENABLED", c.tls)
		t.Setenv("STATIC_BACKENDS", "a=b:1")
		t.Setenv("TLS_MODE", "memory")
		cfg, err := Load()
		if c.wantErr {
			if err == nil {
				t.Errorf("%+v: expected error", c)
			}
			continue
		}
		if err != nil {
			t.Errorf("%+v: %v", c, err)
			continue
		}
		if cfg.TLSEnabled != c.wantTLS || cfg.SNITerminate != c.term {
			t.Errorf("%+v: got tls=%v terminate=%v", c, cfg.TLSEnabled, cfg.SNITerminate)
		}
	}
}
