#!/bin/sh
set -e

# Install the authorized key from the shared volume
cp /ssh-keys/id_ed25519.pub /home/git/.ssh/authorized_keys
chown git:git /home/git/.ssh/authorized_keys
chmod 600 /home/git/.ssh/authorized_keys

# Create and seed the bare repo
git init --bare /home/git/repo.git
git -C /home/git/repo.git symbolic-ref HEAD refs/heads/main

TMPDIR=$(mktemp -d)
cd "$TMPDIR"
git clone /home/git/repo.git work
cd work
git checkout -b main
git config user.email "test@test.local"
git config user.name "Test Setup"

# Create a test post for post-existence validation
mkdir -p _posts
cat > _posts/2024-01-01-test-post.md << 'EOF'
---
title: Test Post
---
Hello world
EOF

# Create comments directory
mkdir -p _data/comments
touch _data/comments/.gitkeep

git add -A
git commit -m "Initial seed"
git push origin main

cd /
rm -rf "$TMPDIR"

# Fix ownership
chown -R git:git /home/git/repo.git

# Start sshd in foreground
exec /usr/sbin/sshd -D -e
