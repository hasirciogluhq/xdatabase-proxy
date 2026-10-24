package route

import "testing"

func TestParseLogin(t *testing.T) {
	for _, c := range []struct {
		in, name string
		key      Key
	}{
		{"alice.db1", "alice", Key{"db1", false}},
		{"alice.db1.pool", "alice", Key{"db1", true}},
		{"a.b.db1.pool", "a.b", Key{"db1", true}},
		{"a.b.db1", "a.b", Key{"db1", false}},
		{"alice", "alice", Key{}},
		{"alice.pool", "alice.pool", Key{}},
	} {
		if n, k := ParseLogin(c.in); n != c.name || k != c.key {
			t.Errorf("%q: got (%q,%+v)", c.in, n, k)
		}
	}
}

func TestParseHost(t *testing.T) {
	for _, c := range []struct {
		in  string
		key Key
	}{
		{"db1.example.com", Key{"db1", false}},
		{"db1.pool.example.com", Key{"db1", true}},
		{"DB1.Pool.Example.com.", Key{"db1", true}},
		{"db1", Key{"db1", false}},
		{"db1.pool", Key{"db1", false}},
	} {
		if k := ParseHost(c.in); k != c.key {
			t.Errorf("%q: got %+v", c.in, k)
		}
	}
}
