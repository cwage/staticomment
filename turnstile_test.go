package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewTurnstileVerifier_DisabledWhenNoSecret(t *testing.T) {
	if v := NewTurnstileVerifier("", ""); v != nil {
		t.Fatalf("expected nil verifier when secret is empty, got %#v", v)
	}
	if v := NewTurnstileVerifier("secret", ""); v == nil {
		t.Fatal("expected non-nil verifier when secret is set")
	} else if v.verifyURL != defaultTurnstileVerifyURL {
		t.Fatalf("expected default verify URL, got %q", v.verifyURL)
	}
}

func TestTurnstileVerify_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.PostFormValue("secret"); got != "test-secret" {
			t.Errorf("secret = %q, want test-secret", got)
		}
		if got := r.PostFormValue("response"); got != "good-token" {
			t.Errorf("response = %q, want good-token", got)
		}
		if got := r.PostFormValue("remoteip"); got != "203.0.113.7" {
			t.Errorf("remoteip = %q, want 203.0.113.7", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	v := NewTurnstileVerifier("test-secret", srv.URL)
	if err := v.Verify(context.Background(), "good-token", "203.0.113.7"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestTurnstileVerify_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	}))
	defer srv.Close()

	v := NewTurnstileVerifier("test-secret", srv.URL)
	if err := v.Verify(context.Background(), "bad-token", ""); err == nil {
		t.Fatal("expected verification to be rejected, got nil error")
	}
}

func TestTurnstileVerify_NonOKStatusFailsClosed(t *testing.T) {
	// Even with a success body, a non-200 status must not be trusted.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	v := NewTurnstileVerifier("test-secret", srv.URL)
	if err := v.Verify(context.Background(), "any-token", ""); err == nil {
		t.Fatal("expected non-200 response to fail closed, got nil")
	}
}

func TestTurnstileVerify_MissingToken(t *testing.T) {
	// No server should be contacted when the token is empty.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("verify endpoint should not be called for an empty token")
	}))
	defer srv.Close()

	v := NewTurnstileVerifier("test-secret", srv.URL)
	if err := v.Verify(context.Background(), "   ", "1.2.3.4"); err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
}

func TestTurnstileVerify_NetworkErrorFailsClosed(t *testing.T) {
	// Point at a closed server to simulate a network failure.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	v := NewTurnstileVerifier("test-secret", url)
	if err := v.Verify(context.Background(), "any-token", ""); err == nil {
		t.Fatal("expected network error to fail closed, got nil")
	}
}
