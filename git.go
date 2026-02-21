package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const (
	repoDir        = "/app/repo"
	knownHostsPath = "/app/.ssh/known_hosts"
)

type GitRepo struct {
	cfg *Config
	mu  sync.Mutex
}

func NewGitRepo(cfg *Config) *GitRepo {
	return &GitRepo{cfg: cfg}
}

func (g *GitRepo) sshCommand() string {
	if g.cfg.SSHInsecure {
		return fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", g.cfg.SSHKeyPath)
	}
	return fmt.Sprintf("ssh -i %s -o UserKnownHostsFile=%s", g.cfg.SSHKeyPath, knownHostsPath)
}

// refreshHostKeys extracts the git host from the repo URL and runs ssh-keyscan
// to update the known_hosts file. This handles the case where baked-in host keys
// have been rotated.
func (g *GitRepo) refreshHostKeys() error {
	host := extractHost(g.cfg.GitRepo)
	if host == "" {
		return fmt.Errorf("could not extract host from repo URL: %s", g.cfg.GitRepo)
	}
	log.Printf("git: refreshing SSH host keys for %s", host)
	cmd := exec.Command("ssh-keyscan", host)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ssh-keyscan %s: %w", host, err)
	}
	if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0700); err != nil {
		return fmt.Errorf("creating .ssh dir: %w", err)
	}
	if err := os.WriteFile(knownHostsPath, out, 0600); err != nil {
		return fmt.Errorf("writing known_hosts: %w", err)
	}
	return nil
}

// extractHost parses the hostname from a git remote URL.
// Handles both SSH (git@github.com:user/repo.git) and HTTPS formats.
func extractHost(repo string) string {
	// SSH format: git@host:path
	if strings.Contains(repo, "@") && strings.Contains(repo, ":") && !strings.Contains(repo, "://") {
		parts := strings.SplitN(repo, "@", 2)
		hostPort := strings.SplitN(parts[1], ":", 2)
		return hostPort[0]
	}
	// HTTPS format
	u, err := url.Parse(repo)
	if err != nil {
		return ""
	}
	return u.Hostname()
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

	cloneArgs := []string{"clone", "--branch", g.cfg.Branch, "--single-branch", g.cfg.GitRepo, repoDir}
	err := g.run("/app", "git", cloneArgs...)
	if err != nil && !g.cfg.SSHInsecure {
		log.Printf("git clone failed, refreshing SSH host keys and retrying")
		if scanErr := g.refreshHostKeys(); scanErr != nil {
			log.Printf("ssh-keyscan failed: %v", scanErr)
			return fmt.Errorf("git clone: %w", err)
		}
		// Clean up failed clone attempt
		os.RemoveAll(repoDir)
		os.MkdirAll(repoDir, 0755)
		err = g.run("/app", "git", cloneArgs...)
	}
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

const pushMaxRetries = 3

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

	// Retry push with rebase on failure (e.g. non-fast-forward rejection)
	for attempt := 0; attempt < pushMaxRetries; attempt++ {
		err := g.run(repoDir, "git", "push")
		if err == nil {
			return nil
		}
		log.Printf("git push attempt %d failed: %v, retrying after pull --rebase", attempt+1, err)
		if pullErr := g.pullLocked(); pullErr != nil {
			return fmt.Errorf("git pull during push retry: %w", pullErr)
		}
	}
	return fmt.Errorf("git push failed after %d attempts", pushMaxRetries)
}

// FullPath returns the absolute path for a file relative to the repo root.
func (g *GitRepo) FullPath(relPath string) string {
	return filepath.Join(repoDir, relPath)
}
