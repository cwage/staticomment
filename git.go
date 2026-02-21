package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

const repoDir = "/app/repo"

type GitRepo struct {
	cfg *Config
	mu  sync.Mutex
}

func NewGitRepo(cfg *Config) *GitRepo {
	return &GitRepo{cfg: cfg}
}

func (g *GitRepo) sshCommand() string {
	return fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", g.cfg.SSHKeyPath)
}

func (g *GitRepo) run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+g.sshCommand())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("git: running %s %v in %s", name, args, dir)
	return cmd.Run()
}

func (g *GitRepo) Clone() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		log.Println("git: repo already cloned, pulling instead")
		return g.pullLocked()
	}

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("creating repo dir: %w", err)
	}

	err := g.run("/app", "git", "clone",
		"--branch", g.cfg.Branch,
		"--single-branch",
		g.cfg.GitRepo,
		repoDir,
	)
	if err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	// Configure git user for commits
	if err := g.run(repoDir, "git", "config", "user.email", "staticomment@quietlife.net"); err != nil {
		return fmt.Errorf("git config email: %w", err)
	}
	if err := g.run(repoDir, "git", "config", "user.name", "staticomment"); err != nil {
		return fmt.Errorf("git config name: %w", err)
	}

	return nil
}

func (g *GitRepo) pullLocked() error {
	return g.run(repoDir, "git", "pull", "--rebase")
}

func (g *GitRepo) Pull() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.pullLocked()
}

func (g *GitRepo) CommitAndPush(filePath, slug string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.pullLocked(); err != nil {
		return fmt.Errorf("git pull before commit: %w", err)
	}

	if err := g.run(repoDir, "git", "add", filePath); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	msg := fmt.Sprintf("Add comment on %s", slug)
	if err := g.run(repoDir, "git", "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	if err := g.run(repoDir, "git", "push"); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

// FullPath returns the absolute path for a file relative to the repo root.
func (g *GitRepo) FullPath(relPath string) string {
	return filepath.Join(repoDir, relPath)
}
