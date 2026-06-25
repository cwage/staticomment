# staticomment

A lightweight comment server for static websites (e.g. jekyll). Comments are stored as YAML files and committed directly to your site's git repository.

## How it works

staticomment clones your static site's git repo on startup. When a visitor submits a comment via an HTML form, the server:

1. Validates the origin and input fields
2. Writes a YAML file to `_data/comments/<slug>/<timestamp>-<random>.yml`
3. Commits and pushes to the repo

Your static site generator reads the YAML data files at build time to render comments.

## Configuration

All configuration is via environment variables:

| Variable | Required | Default | Description |
|---|---|---|---|
| `STATICOMMENT_GIT_REPO` | Yes | | Git remote URL (SSH format) |
| `STATICOMMENT_BRANCH` | No | `main` | Branch to clone and push to |
| `STATICOMMENT_COMMENTS_PATH` | No | `_data/comments` | Path within repo for comment files |
| `STATICOMMENT_PORT` | No | `8080` | HTTP listen port |
| `STATICOMMENT_ALLOWED_ORIGINS` | Yes | | Comma-separated allowed origins (e.g. `https://example.com`) |
| `STATICOMMENT_SSH_KEY_PATH` | No | `/app/.ssh/id_ed25519` | Path to SSH deploy key |
| `STATICOMMENT_SSH_INSECURE` | No | `0` | Set to `1` to disable strict host key checking |

### Spam mitigation

All of these are optional and layer on top of origin validation.

| Variable | Default | Description |
|---|---|---|
| `STATICOMMENT_HONEYPOT_FIELD` | `website` | Name of a hidden form field; submissions that fill it are silently discarded. |
| `STATICOMMENT_RATE_LIMIT_WINDOW` | `60` | Rate-limit window in seconds. |
| `STATICOMMENT_RATE_LIMIT_MAX` | `5` | Max submissions per IP per window (`0` disables). |
| `STATICOMMENT_MAX_LINKS` | `3` | Reject a comment body with more than this many links (`0` disables). |
| `STATICOMMENT_BLOCKED_PATTERNS` | | Comma-separated case-insensitive regexes; a body matching any is rejected. |
| `STATICOMMENT_MIN_SUBMIT_TIME` | `5` | Reject submissions faster than this many seconds, measured from a hidden `_timestamp` field (`0` disables). |
| `STATICOMMENT_TURNSTILE_SECRET` | | Cloudflare Turnstile secret key. When set, every comment must carry a valid Turnstile token, verified server-side with Cloudflare. Unset = disabled. |
| `STATICOMMENT_TURNSTILE_VERIFY_URL` | Cloudflare siteverify | Override the verification endpoint (mainly for testing). |

Honeypot, `_timestamp`, and Turnstile all depend on the *form* rendering the
corresponding fields/widget. Turnstile is the only one of these that defends
against bots POSTing directly to the endpoint without rendering the form, since
its token cannot be forged without solving the challenge in a real browser.

## Deployment

### Docker

```bash
docker run -d \
  -e STATICOMMENT_GIT_REPO=git@github.com:you/your-site.git \
  -e STATICOMMENT_ALLOWED_ORIGINS=https://your-site.com \
  -v /path/to/deploy-key:/app/.ssh/id_ed25519 \
  -p 8080:8080 \
  ghcr.io/cwage/staticomment:latest
```

### Docker Compose

```yaml
staticomment:
  image: ghcr.io/cwage/staticomment:latest
  restart: unless-stopped
  environment:
    - STATICOMMENT_GIT_REPO=git@github.com:you/your-site.git
    - STATICOMMENT_ALLOWED_ORIGINS=https://your-site.com
    - STATICOMMENT_SSH_KEY_PATH=/app/.ssh/id_ed25519
  volumes:
    - ./ssh-key:/app/.ssh
  ports:
    - "8080:8080"
```

## API

### `GET /health`

Returns `200 OK` with body `ok`.

### `POST /comment`

Accepts `application/x-www-form-urlencoded` with the following fields:

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Commenter's name |
| `body` | Yes | Comment text (max 10,000 characters) |
| `slug` | Yes | Post identifier (alphanumeric, hyphens, underscores) |
| `url` | Yes | Redirect URL after submission |
| `email` | No | Commenter's email |
| `reply_to` | No | ID of the comment this one replies to |
| `cf-turnstile-response` | If Turnstile enabled | Token emitted by the Turnstile widget |

On success, redirects to `url#comment-submitted`. On error, redirects to `url?comment_error=<message>`.

The `Origin` or `Referer` header must match one of the configured allowed origins.

## Jekyll integration

Add a comment form to your post layout that POSTs to your staticomment instance. The `slug` field should uniquely identify the post. In your template, read comments from `site.data.comments[slug]`. Each comment YAML file contains `name`, `email` (if provided), `body`, `date`, and `slug`.


## Limitations

- **Requires an SSH deploy key** with write access to the site repo. The key must be configured as a deploy key on the repo (not a personal SSH key).
- **Pushes directly to the configured branch** (default `main`). There is no PR-based workflow or moderation queue — comments go live on the next site build.
- **Single repo only.** One staticomment instance serves one git repository.
- **Synchronous git operations.** Each comment submission blocks until the commit is pushed. A global mutex serializes all git operations, so concurrent submissions are queued.
- **Spam protection is heuristic by default.** Origin validation, honeypot, rate limiting, link limits, blocked patterns, and a submit-time gate are built in (see [Spam mitigation](#spam-mitigation)). For bots that POST directly to the endpoint, enable Cloudflare Turnstile, which is server-side verified and unforgeable.
- **No notification system.** There are no webhooks or email alerts when comments are submitted.

## License

MIT
