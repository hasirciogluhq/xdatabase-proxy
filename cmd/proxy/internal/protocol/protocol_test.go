package protocol

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func reader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

func TestRedisAuth(t *testing.T) {
	user, fwd, err := Redis{}.Read(reader("*3\r\n$4\r\nAUTH\r\n$14\r\nalice.db1.pool\r\n$6\r\nsecret\r\n"))
	if err != nil || user != "alice.db1.pool" {
		t.Fatalf("user=%q err=%v", user, err)
	}
	want := "*3\r\n$4\r\nAUTH\r\n$5\r\nalice\r\n$6\r\nsecret\r\n"
	if got := string(fwd("alice")); got != want {
		t.Fatalf("forward %q, want %q", got, want)
	}
}

func TestRedisHelloAuth(t *testing.T) {
	user, fwd, err := Redis{}.Read(reader(string(encodeRESP([]string{"hello", "3", "auth", "bob.d1", "pw", "SETNAME", "app"}))))
	if err != nil || user != "bob.d1" {
		t.Fatalf("user=%q err=%v", user, err)
	}
	want := string(encodeRESP([]string{"hello", "3", "auth", "bob", "pw", "SETNAME", "app"}))
	if got := string(fwd("bob")); got != want {
		t.Fatalf("forward %q, want %q", got, want)
	}
}

func TestRedisRejectsUnroutableFirstCommands(t *testing.T) {
	for name, in := range map[string]string{
		"ping":          string(encodeRESP([]string{"PING"})),
		"hello no auth": string(encodeRESP([]string{"HELLO", "3"})),
		"password only": string(encodeRESP([]string{"AUTH", "secret"})),
		"inline":        "PING\r\n",
		"zero args":     "*0\r\n",
		"too many args": "*33\r\n",
		"huge bulk":     "*1\r\n$99999\r\n",
		"bad bulk":      "*1\r\n$4\r\nPINGXX",
		"truncated":     "*2\r\n$4\r\nAUTH\r\n",
	} {
		if _, _, err := (Redis{}).Read(reader(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestRedisKeepsPipelinedBytesInReader(t *testing.T) {
	cmd := string(encodeRESP([]string{"AUTH", "a.d", "p"}))
	r := reader(cmd + "*1\r\n$4\r\nPING\r\n")
	if _, _, err := (Redis{}).Read(r); err != nil {
		t.Fatal(err)
	}
	rest := make([]byte, 14)
	if n, _ := r.Read(rest); string(rest[:n]) != "*1\r\n$4\r\nPING\r\n" {
		t.Fatalf("pipelined bytes lost: %q", rest[:n])
	}
}

func clickhouseHello(user string) []byte {
	return chHello{clientName: "ClickHouse client", major: 24, minor: 8, revision: 54471, database: "default", user: user, password: "pw"}.encode()
}

func TestClickHouseHello(t *testing.T) {
	in := clickhouseHello("alice.db1.pool")
	user, fwd, err := ClickHouse{}.Read(bufio.NewReader(bytes.NewReader(in)))
	if err != nil || user != "alice.db1.pool" {
		t.Fatalf("user=%q err=%v", user, err)
	}
	if got, want := fwd("alice"), clickhouseHello("alice"); !bytes.Equal(got, want) {
		t.Fatalf("forward % x, want % x", got, want)
	}
}

func TestClickHouseRejectsBadInput(t *testing.T) {
	long := binary.AppendUvarint([]byte{0}, 1<<20) // client name longer than the limit
	for name, in := range map[string][]byte{
		"not hello": {1, 0, 0},
		"empty":     {},
		"too long":  long,
		"truncated": clickhouseHello("alice")[:10],
	} {
		if _, _, err := (ClickHouse{}).Read(bufio.NewReader(bytes.NewReader(in))); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
