package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	log.Printf("staticomment starting on :%s", cfg.Port)
	log.Printf("  repo: %s (branch: %s)", cfg.GitRepo, cfg.Branch)
	log.Printf("  comments path: %s", cfg.CommentsPath)
	if cfg.PostsPath != "" {
		log.Printf("  posts path: %s (post existence validation enabled)", cfg.PostsPath)
	}
	log.Printf("  allowed origins: %v", cfg.AllowedOrigins)
	if cfg.HoneypotField != "" {
		log.Printf("  honeypot field: %s", cfg.HoneypotField)
	}
	if cfg.RateLimitMax > 0 {
		log.Printf("  rate limit: %d requests per %d seconds", cfg.RateLimitMax, cfg.RateLimitWindow)
	}
	if cfg.MaxLinks > 0 {
		log.Printf("  max links: %d", cfg.MaxLinks)
	}
	if len(cfg.BlockedPatterns) > 0 {
		log.Printf("  blocked patterns: %d", len(cfg.BlockedPatterns))
	}
	if cfg.MinSubmitTime > 0 {
		log.Printf("  min submit time: %ds", cfg.MinSubmitTime)
	}

	repo := NewGitRepo(cfg)
	if err := repo.Clone(); err != nil {
		log.Fatalf("git clone failed: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	rateLimiter := NewRateLimiter(cfg.RateLimitWindow, cfg.RateLimitMax)
	mux.Handle("POST /comment", NewCommentHandler(cfg, repo, rateLimiter))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
