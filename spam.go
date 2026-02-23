package main

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimiter tracks request timestamps per IP for rate limiting.
type RateLimiter struct {
	window  time.Duration
	max     int
	mu      sync.Mutex
	entries map[string][]time.Time
}

// NewRateLimiter creates a rate limiter. If max is 0, limiting is disabled.
func NewRateLimiter(windowSeconds, max int) *RateLimiter {
	rl := &RateLimiter{
		window:  time.Duration(windowSeconds) * time.Second,
		max:     max,
		entries: make(map[string][]time.Time),
	}
	if max > 0 {
		go rl.cleanup()
	}
	return rl
}

// Allow checks whether the given IP is within the rate limit.
// Returns true if the request is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	if rl.max <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Filter out expired entries
	timestamps := rl.entries[ip]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.max {
		rl.entries[ip] = valid
		return false
	}

	rl.entries[ip] = append(valid, now)
	return true
}

// cleanup periodically removes expired entries to prevent memory growth.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-rl.window)
		for ip, timestamps := range rl.entries {
			valid := timestamps[:0]
			for _, t := range timestamps {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.entries, ip)
			} else {
				rl.entries[ip] = valid
			}
		}
		rl.mu.Unlock()
	}
}

// extractIP returns the IP portion of a RemoteAddr, stripping the port.
func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// checkHoneypot returns true if the honeypot field is filled (indicating a bot).
func checkHoneypot(r *http.Request, fieldName string) bool {
	if fieldName == "" {
		return false
	}
	return strings.TrimSpace(r.FormValue(fieldName)) != ""
}

var linkPattern = regexp.MustCompile(`https?://`)

// checkBodyContent checks the comment body for excessive links and blocked patterns.
// Returns an error message string, or empty string if the body is acceptable.
func checkBodyContent(body string, maxLinks int, blockedPatterns []*regexp.Regexp) string {
	if maxLinks > 0 {
		count := len(linkPattern.FindAllStringIndex(body, -1))
		if count > maxLinks {
			return fmt.Sprintf("Too many links (max %d)", maxLinks)
		}
	}

	for _, re := range blockedPatterns {
		if re.MatchString(body) {
			return "Comment contains blocked content"
		}
	}

	return ""
}

// checkTimestamp returns true if the submission was too fast (likely a bot).
// It parses the hidden _timestamp field (unix epoch seconds) and compares to now.
func checkTimestamp(r *http.Request, minSeconds int) bool {
	if minSeconds <= 0 {
		return false
	}
	tsStr := strings.TrimSpace(r.FormValue("_timestamp"))
	if tsStr == "" {
		// No timestamp field — skip check (form may not include it)
		return false
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return true // Invalid timestamp, treat as suspicious
	}
	elapsed := time.Now().Unix() - ts
	return elapsed < int64(minSeconds)
}
