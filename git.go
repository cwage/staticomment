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

// ensureHostKeys checks whether the configured git host is already in known_hosts.
// If not, it runs ssh-keyscan to fetch the host keys. This runs once at startup
// so that any git host (GitHub, GitLab, Gitea, self-hosted, etc.) works without
// manual known_hosts configuration.
func (g *GitRepo) ensureHostKeys() error {
	if g.cfg.SSHInsecure {
		return nil
	}
	host := extractHost(g.cfg.GitRepo)
	if host == "" {
		return fmt.Errorf("could not extract host from repo URL: %s", g.cfg.GitRepo)
	}
	if hostInKnownHosts(host) {
		log.Printf("git: host key for %s already in known_hosts", host)
		return nil
	}
	log.Printf("git: host key for %s not found, running ssh-keyscan", host)
	return scanAndAppendHostKeys(host)
}

// refreshHostKeys replaces the host keys for the configured git host.
// Used as a fallback when a git operation fails due to stale keys.
func (g *GitRepo) refreshHostKeys() error {
	host := extractHost(g.cfg.GitRepo)
	if host == "" {
		return fmt.Errorf("could not extract host from repo URL: %s", g.cfg.GitRepo)
	}
	log.Printf("git: refreshing SSH host keys for %s", host)
	// Overwrite rather than append to replace potentially stale keys
	return scanAndWriteHostKeys(host)
}

func hostInKnownHosts(host string) bool {
	data, err := os.ReadFile(knownHostsPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, host+" ") || strings.HasPrefix(line, host+",") {
			return true
		}
	}
	return false
}

func scanHostKeys(host string) ([]byte, error) {
	cmd := exec.Command("ssh-keyscan", host)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ssh-keyscan %s: %w", host, err)
	}
	return out, nil
}

func scanAndAppendHostKeys(host string) error {
	out, err := scanHostKeys(host)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0700); err != nil {
		return fmt.Errorf("creating .ssh dir: %w", err)
	}
	f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("opening known_hosts: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(out); err != nil {
		return fmt.Errorf("appending to known_hosts: %w", err)
	}
	return nil
}

func scanAndWriteHostKeys(host string) error {
	out, err := scanHostKeys(host)
	if err != nil {
		return err
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

// sanitizeArgs redacts credentials from URL-like arguments for safe logging.
func sanitizeArgs(args []string) []string {
	safe := make([]string, len(args))
	for i, arg := range args {
		if strings.Contains(arg, "://") {
			if u, err := url.Parse(arg); err == nil && u.User != nil {
				u.User = nil
				safe[i] = u.String()
				continue
			}
		}
		safe[i] = arg
	}
	return safe
}

func (g *GitRepo) run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+g.sshCommand())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("git: running %s %v in %s", name, sanitizeArgs(args), dir)
	return cmd.Run()
}

func (g *GitRepo) Clone() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Ensure the configured git host is in known_hosts before any SSH operation.
	// For hosts baked into the image (GitHub, GitLab), this is a no-op.
	// For self-hosted or other providers, this runs ssh-keyscan automatically.
	if err := g.ensureHostKeys(); err != nil {
		log.Printf("warning: could not ensure host keys: %v", err)
	}

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
		// Clone failed — possibly stale host keys. Refresh and retry once.
		log.Printf("git clone failed, refreshing SSH host keys and retrying")
		if scanErr := g.refreshHostKeys(); scanErr != nil {
			log.Printf("ssh-keyscan failed: %v", scanErr)
			return fmt.Errorf("git clone: %w", err)
		}
		if rmErr := os.RemoveAll(repoDir); rmErr != nil {
			return fmt.Errorf("removing repo dir before retry: %w", rmErr)
		}
		if mkErr := os.MkdirAll(repoDir, 0755); mkErr != nil {
			return fmt.Errorf("creating repo dir before retry: %w", mkErr)
		}
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
			// Rebase may have left a conflicted state — abort it
			g.run(repoDir, "git", "rebase", "--abort")
			return fmt.Errorf("git pull during push retry: %w", pullErr)
		}
	}
	return fmt.Errorf("git push failed after %d attempts", pushMaxRetries)
}

// FullPath returns the absolute path for a file relative to the repo root.
func (g *GitRepo) FullPath(relPath string) string {
	return filepath.Join(repoDir, relPath)
}
