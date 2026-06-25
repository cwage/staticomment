package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// defaultTurnstileVerifyURL is Cloudflare's server-side token verification endpoint.
const defaultTurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// turnstileTokenField is the form field the Turnstile widget populates client-side.
const turnstileTokenField = "cf-turnstile-response"

// TurnstileVerifier validates Cloudflare Turnstile tokens server-side.
//
// The widget runs a challenge in the visitor's browser and emits a token. The
// server then exchanges that token plus its secret key with Cloudflare, which
// is what makes the check unforgeable: a bot POSTing directly to the endpoint
// has no valid token and cannot mint one without solving the challenge.
type TurnstileVerifier struct {
	secret    string
	verifyURL string
	client    *http.Client
}

// NewTurnstileVerifier returns a verifier, or nil if secret is empty. A nil
// verifier means the feature is disabled and verification is skipped entirely,
// keeping Turnstile strictly opt-in.
func NewTurnstileVerifier(secret, verifyURL string) *TurnstileVerifier {
	if secret == "" {
		return nil
	}
	if verifyURL == "" {
		verifyURL = defaultTurnstileVerifyURL
	}
	return &TurnstileVerifier{
		secret:    secret,
		verifyURL: verifyURL,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// turnstileResponse is the subset of the siteverify response we care about.
type turnstileResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify checks a token (and optional remote IP) with Cloudflare. It returns
// nil when the token is valid and a descriptive error otherwise. A network or
// decode failure is treated as a verification failure (fail closed).
func (t *TurnstileVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("missing turnstile token")
	}

	form := url.Values{}
	form.Set("secret", t.secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("building turnstile verify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("turnstile verify request failed: %w", err)
	}
	defer resp.Body.Close()

	// A non-200 indicates an endpoint/proxy problem, not a verdict. Treat it as
	// a hard failure rather than risk trusting whatever body accompanies it.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("turnstile verify returned HTTP %d", resp.StatusCode)
	}

	var tr turnstileResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return fmt.Errorf("decoding turnstile response: %w", err)
	}
	if !tr.Success {
		return fmt.Errorf("turnstile verification rejected: %v", tr.ErrorCodes)
	}
	return nil
}
