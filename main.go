package main

import (
	"log"
	"net/http"
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

	log.Printf("listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
