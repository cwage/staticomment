package main

import "testing"

// setRequiredEnv sets the minimum env vars LoadConfig requires, so each test
// can focus on the field under test.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("STATICOMMENT_GIT_REPO", "git@example.com:owner/repo.git")
	t.Setenv("STATICOMMENT_ALLOWED_ORIGINS", "https://example.com")
}

func TestLoadConfig_RejectsMalformedTurnstileVerifyURL(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("STATICOMMENT_TURNSTILE_SECRET", "secret")
	t.Setenv("STATICOMMENT_TURNSTILE_VERIFY_URL", "not-a-url")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for malformed TURNSTILE_VERIFY_URL, got nil")
	}
}

func TestLoadConfig_AcceptsValidTurnstileVerifyURL(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("STATICOMMENT_TURNSTILE_SECRET", "secret")
	t.Setenv("STATICOMMENT_TURNSTILE_VERIFY_URL", "https://verify.example.com/siteverify")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
	if cfg.TurnstileVerifyURL != "https://verify.example.com/siteverify" {
		t.Fatalf("verify URL = %q, want the configured override", cfg.TurnstileVerifyURL)
	}
}

func TestLoadConfig_TurnstileDisabledByDefault(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TurnstileSecret != "" {
		t.Fatalf("expected Turnstile disabled by default, got secret %q", cfg.TurnstileSecret)
	}
}
