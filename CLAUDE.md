# staticomment

Lightweight comment server for static sites. Receives form POSTs, writes YAML comment files, and commits/pushes them to a git repo over SSH.

## Design Goals

- **Platform-agnostic**: Must work equally well with GitHub, GitLab, Gitea, Forgejo, Bitbucket, or any self-hosted git server accessible over SSH. Never assume GitHub as the only target. Features that benefit specific platforms (e.g. baked-in SSH host keys) are fine as optimizations, but the core flow must not depend on them.
- **Static-site-generator-agnostic**: The YAML comment file format and directory layout should work with Jekyll, Hugo, Eleventy, or any SSG that can read data files. Don't couple to Jekyll-specific features.
- **Simple and single-purpose**: This is a small service. Flat file structure, no unnecessary abstractions. Restructure only if complexity demands it.
- **Docker-first**: All development and deployment via Docker. No local Go toolchain required.
- **Secure by default**: Strict SSH host key checking, origin validation, input sanitization. Insecure options are opt-in, not default.

## Architecture

- `config.go` — env var parsing and validation
- `git.go` — git clone/pull/commit/push via os/exec, mutex-locked
- `handler.go` — HTTP handler for POST /comment
- `main.go` — entry point, config, server setup

## Build & Run

```
make build    # docker build
make run      # docker compose up -d
make shell    # docker compose exec staticomment sh
make stop     # docker compose down
```

## Config (env vars)

| Variable | Required | Default | Description |
|---|---|---|---|
| `STATICOMMENT_GIT_REPO` | yes | — | Git remote URL (SSH or HTTPS) |
| `STATICOMMENT_BRANCH` | no | `main` | Branch to clone and push to |
| `STATICOMMENT_COMMENTS_PATH` | no | `_data/comments` | Path within repo for comment files |
| `STATICOMMENT_PORT` | no | `8080` | HTTP listen port |
| `STATICOMMENT_ALLOWED_ORIGINS` | yes | — | Comma-separated allowed origins |
| `STATICOMMENT_SSH_KEY_PATH` | no | `/app/.ssh/id_ed25519` | Path to SSH deploy key |
| `STATICOMMENT_SSH_INSECURE` | no | `0` | Set to `1` to disable SSH host key checking |
