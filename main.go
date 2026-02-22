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
		log.Printf("  posts path: %s (slug validation enabled)", cfg.PostsPath)
	}
	log.Printf("  allowed origins: %v", cfg.AllowedOrigins)

	repo := NewGitRepo(cfg)
	if err := repo.Clone(); err != nil {
		log.Fatalf("git clone failed: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.Handle("POST /comment", NewCommentHandler(cfg, repo))

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
