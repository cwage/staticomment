package main

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	GitRepo      string
	Branch       string
	CommentsPath string
	Port         string
	AllowedOrigins []string
	SSHKeyPath   string
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Branch:       envOrDefault("STATICOMMENT_BRANCH", "main"),
		CommentsPath: envOrDefault("STATICOMMENT_COMMENTS_PATH", "_data/comments"),
		Port:         envOrDefault("STATICOMMENT_PORT", "8080"),
		SSHKeyPath:   envOrDefault("STATICOMMENT_SSH_KEY_PATH", "/app/.ssh/id_ed25519"),
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
		if o != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
		}
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
