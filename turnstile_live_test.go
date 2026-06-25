//go:build manual

// Live round-trip tests against Cloudflare's real siteverify endpoint using the
// documented test keys and dummy token. Network-dependent, so excluded from the
// normal suite — run with: go test -tags manual -run Live -v
package main

import (
	"context"
	"testing"
)

const dummyToken = "XXXX.DUMMY.TOKEN.XXXX"

func TestLiveTurnstile_AlwaysPassSecret(t *testing.T) {
	v := NewTurnstileVerifier("1x0000000000000000000000000000000AA", "")
	if err := v.Verify(context.Background(), dummyToken, ""); err != nil {
		t.Fatalf("always-pass secret should accept dummy token, got: %v", err)
	}
}

func TestLiveTurnstile_AlwaysFailSecret(t *testing.T) {
	v := NewTurnstileVerifier("2x0000000000000000000000000000000AA", "")
	if err := v.Verify(context.Background(), dummyToken, ""); err == nil {
		t.Fatal("always-fail secret should reject dummy token, got nil error")
	}
}

func TestLiveTurnstile_AlreadySpentSecret(t *testing.T) {
	v := NewTurnstileVerifier("3x0000000000000000000000000000000AA", "")
	if err := v.Verify(context.Background(), dummyToken, ""); err == nil {
		t.Fatal("token-already-spent secret should reject, got nil error")
	}
}
