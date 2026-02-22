package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	GitRepo        string
	Branch         string
	CommentsPath   string
	PostsPath      string
	Port           string
	AllowedOrigins []string
	SSHKeyPath     string
	SSHInsecure    bool
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Branch:       envOrDefault("STATICOMMENT_BRANCH", "main"),
		CommentsPath: envOrDefault("STATICOMMENT_COMMENTS_PATH", "_data/comments"),
		PostsPath:    os.Getenv("STATICOMMENT_POSTS_PATH"),
		Port:         envOrDefault("STATICOMMENT_PORT", "8080"),
		SSHKeyPath:   envOrDefault("STATICOMMENT_SSH_KEY_PATH", "/app/.ssh/id_ed25519"),
	}

	cfg.SSHInsecure = os.Getenv("STATICOMMENT_SSH_INSECURE") == "1"

	// Validate CommentsPath is relative and clean
	if filepath.IsAbs(cfg.CommentsPath) {
		return nil, fmt.Errorf("STATICOMMENT_COMMENTS_PATH must be a relative path")
	}
	cfg.CommentsPath = filepath.Clean(cfg.CommentsPath)
	if strings.HasPrefix(cfg.CommentsPath, "..") {
		return nil, fmt.Errorf("STATICOMMENT_COMMENTS_PATH must not escape the repo directory")
	}

	// Validate PostsPath if set (empty disables post validation)
	if cfg.PostsPath != "" {
		if filepath.IsAbs(cfg.PostsPath) {
			return nil, fmt.Errorf("STATICOMMENT_POSTS_PATH must be a relative path")
		}
		cfg.PostsPath = filepath.Clean(cfg.PostsPath)
		if strings.HasPrefix(cfg.PostsPath, "..") {
			return nil, fmt.Errorf("STATICOMMENT_POSTS_PATH must not escape the repo directory")
		}
	}

	cfg.GitRepo = os.Getenv("STATICOMMENT_GIT_REPO")
	if cfg.GitRepo == "" {
		return nil, fmt.Errorf("STATICOMMENT_GIT_REPO is required")
	}

	origins := os.Getenv("STATICOMMENT_ALLOWED_ORIGINS")
	if origins == "" {
		return nil, fmt.Errorf("STATICOMMENT_ALLOWED_ORIGINS is required")
	}
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		u, err := url.Parse(o)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("STATICOMMENT_ALLOWED_ORIGINS: invalid origin %q (must include scheme and host, e.g. https://example.com)", o)
		}
		cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
	}
	if len(cfg.AllowedOrigins) == 0 {
		return nil, fmt.Errorf("STATICOMMENT_ALLOWED_ORIGINS must contain at least one origin")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
