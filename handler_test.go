package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newTurnstileTestHandler builds a handler with Turnstile enabled, pointed at a
// mock siteverify endpoint. The git repo is never reached on the paths tested
// here (verification happens before any git operation), so a zero repo is fine.
func newTurnstileTestHandler(verifyURL string) *CommentHandler {
	cfg := &Config{
		AllowedOrigins:     []string{"http://test.local"},
		TurnstileSecret:    "test-secret",
		TurnstileVerifyURL: verifyURL,
	}
	return NewCommentHandler(cfg, &GitRepo{}, NewRateLimiter(60, 0))
}

func postComment(h *CommentHandler, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/comment", strings.NewReader(form.Encode()))
	req.Header.Set("Origin", "http://test.local")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestHandler_TurnstileRejectsBadToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	}))
	defer srv.Close()

	h := newTurnstileTestHandler(srv.URL)
	rr := postComment(h, url.Values{
		"name":              {"Bot"},
		"body":              {"spam"},
		"slug":              {"test-post"},
		"url":               {"http://test.local/blog/test-post"},
		turnstileTokenField: {"bad-token"},
	})

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "comment_error=") {
		t.Fatalf("expected error redirect, got Location=%q", loc)
	}
}

func TestHandler_TurnstileRejectsMissingToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("siteverify must not be called when no token is supplied")
	}))
	defer srv.Close()

	h := newTurnstileTestHandler(srv.URL)
	rr := postComment(h, url.Values{
		"name": {"Bot"},
		"body": {"spam"},
		"slug": {"test-post"},
		"url":  {"http://test.local/blog/test-post"},
		// no cf-turnstile-response field at all (direct-to-endpoint bot)
	})

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "comment_error=") {
		t.Fatalf("expected error redirect, got Location=%q", loc)
	}
}
