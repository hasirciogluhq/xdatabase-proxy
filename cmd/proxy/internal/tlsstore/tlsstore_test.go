package tlsstore

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

var log = slog.New(slog.DiscardHandler)

func TestEnsureGeneratesOnceAndReuses(t *testing.T) {
	m := &Memory{}
	c1, err := Ensure(context.Background(), m, true, time.Hour, log)
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := Ensure(context.Background(), m, true, time.Hour, log)
	if c1 != c2 {
		t.Fatal("certificate regenerated although still valid")
	}
}

func TestEnsureRenewsExpiringSelfSigned(t *testing.T) {
	m := &Memory{}
	c1, _ := Ensure(context.Background(), m, true, time.Hour, log)
	c2, err := Ensure(context.Background(), m, true, 400*24*time.Hour, log)
	if err != nil || c1 == c2 {
		t.Fatalf("expected renewal, err=%v", err)
	}
}

func TestEnsureWithoutAutoGenerateFails(t *testing.T) {
	if _, err := Ensure(context.Background(), &Memory{}, false, time.Hour, log); err == nil {
		t.Fatal("expected error")
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	d := t.TempDir()
	f := File{d + "/tls.crt", d + "/tls.key"}
	if _, err := Ensure(context.Background(), f, true, time.Hour, log); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
}
