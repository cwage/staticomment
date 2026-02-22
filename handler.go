package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultMaxBodyLen = 10000

type Comment struct {
	Name    string `yaml:"name"`
	Email   string `yaml:"email,omitempty"`
	Body    string `yaml:"body"`
	Date    string `yaml:"date"`
	Slug    string `yaml:"slug"`
	ReplyTo string `yaml:"reply_to,omitempty"`
}

type CommentHandler struct {
	cfg  *Config
	repo *GitRepo
}

func NewCommentHandler(cfg *Config, repo *GitRepo) *CommentHandler {
	return &CommentHandler{cfg: cfg, repo: repo}
}

func (h *CommentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.checkOrigin(r) {
		http.Error(w, "Forbidden: origin not allowed", http.StatusForbidden)
		return
	}

	// Limit request body to prevent resource exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	body := strings.TrimSpace(r.FormValue("body"))
	slug := strings.TrimSpace(r.FormValue("slug"))
	replyTo := strings.TrimSpace(r.FormValue("reply_to"))
	redirectURL := strings.TrimSpace(r.FormValue("url"))

	// Validate redirect URL against allowed origins before using it in any redirect
	if redirectURL != "" && !h.isAllowedRedirect(redirectURL) {
		http.Error(w, "Forbidden: redirect URL origin not allowed", http.StatusForbidden)
		return
	}

	// Validate required fields
	if name == "" || body == "" || slug == "" || redirectURL == "" {
		h.errorRedirect(w, r, redirectURL, "Missing required fields (name, body, slug, url)")
		return
	}

	// Validate body length
	if len(body) > defaultMaxBodyLen {
		h.errorRedirect(w, r, redirectURL, "Comment body too long")
		return
	}

	// Sanitize slug — reject path traversal
	if !isValidSlug(slug) {
		h.errorRedirect(w, r, redirectURL, "Invalid slug")
		return
	}

	// Validate reply_to format if provided
	if replyTo != "" && !isValidSlug(replyTo) {
		h.errorRedirect(w, r, redirectURL, "Invalid reply_to")
		return
	}

	// Validate that a post matching this slug exists in the repo
	if h.cfg.PostsPath != "" {
		if !h.postExists(slug) {
			h.errorRedirect(w, r, redirectURL, "Post not found")
			return
		}
	}

	// Build comment
	comment := Comment{
		Name:    name,
		Email:   email,
		Body:    body,
		Date:    time.Now().UTC().Format(time.RFC3339),
		Slug:    slug,
		ReplyTo: replyTo,
	}

	// Write YAML file
	relPath, err := h.writeComment(comment)
	if err != nil {
		log.Printf("error writing comment: %v", err)
		h.errorRedirect(w, r, redirectURL, "Failed to save comment")
		return
	}

	// Git commit and push
	if err := h.repo.CommitAndPush(relPath, slug); err != nil {
		log.Printf("error committing comment: %v", err)
		h.errorRedirect(w, r, redirectURL, "Failed to publish comment")
		return
	}

	log.Printf("comment saved and pushed: %s", relPath)

	// Redirect back to the post
	u, err := url.Parse(redirectURL)
	if err != nil {
		http.Error(w, "Bad redirect URL", http.StatusBadRequest)
		return
	}
	u.Fragment = "comment-submitted"
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

func (h *CommentHandler) writeComment(c Comment) (string, error) {
	// Build the directory path: <comments_path>/<slug>/
	dir := filepath.Join(h.cfg.CommentsPath, c.Slug)
	fullDir := h.repo.FullPath(dir)
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		return "", fmt.Errorf("creating comment dir: %w", err)
	}

	// Generate filename: <timestamp>-<random>.yml
	ts := time.Now().UTC().Format("20060102150405")
	rnd, err := randomHex(4)
	if err != nil {
		return "", fmt.Errorf("generating random id: %w", err)
	}
	filename := fmt.Sprintf("%s-%s.yml", ts, rnd)
	relPath := filepath.Join(dir, filename)
	fullPath := h.repo.FullPath(relPath)

	data, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshaling comment: %w", err)
	}

	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("writing comment file: %w", err)
	}

	return relPath, nil
}

func (h *CommentHandler) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Fall back to Referer
		ref := r.Header.Get("Referer")
		if ref == "" {
			return false
		}
		u, err := url.Parse(ref)
		if err != nil {
			return false
		}
		origin = u.Scheme + "://" + u.Host
	}

	for _, allowed := range h.cfg.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func (h *CommentHandler) errorRedirect(w http.ResponseWriter, r *http.Request, redirectURL, msg string) {
	if redirectURL != "" {
		u, err := url.Parse(redirectURL)
		if err == nil {
			q := u.Query()
			q.Set("comment_error", msg)
			u.RawQuery = q.Encode()
			http.Redirect(w, r, u.String(), http.StatusSeeOther)
			return
		}
	}
	http.Error(w, msg, http.StatusBadRequest)
}

func (h *CommentHandler) isAllowedRedirect(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	origin := u.Scheme + "://" + u.Host
	for _, allowed := range h.cfg.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func (h *CommentHandler) postExists(slug string) bool {
	pattern := filepath.Join(h.repo.FullPath(h.cfg.PostsPath), slug+".*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		log.Printf("error globbing for post %s: %v", slug, err)
		return false
	}
	return len(matches) > 0
}

func isValidSlug(slug string) bool {
	if slug == "" {
		return false
	}
	if strings.Contains(slug, "..") {
		return false
	}
	if strings.ContainsAny(slug, "/\\") {
		return false
	}
	// Only allow alphanumeric, hyphens, underscores
	for _, c := range slug {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
