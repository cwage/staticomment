#!/bin/sh
# staticomment integration tests
# Runs against a fully containerized environment with git-server and staticomment

STATICOMMENT_URL="${STATICOMMENT_URL:-http://staticomment:8080}"
GIT_SERVER="${GIT_SERVER:-git-server}"
ALLOWED_ORIGIN="${ALLOWED_ORIGIN:-http://testsite.local}"
REDIRECT_URL="${ALLOWED_ORIGIN}/blog/test-post"

PASS=0
FAIL=0
TOTAL=0

pass() {
    PASS=$((PASS + 1))
    TOTAL=$((TOTAL + 1))
    printf "  PASS: %s\n" "$1"
}

fail() {
    FAIL=$((FAIL + 1))
    TOTAL=$((TOTAL + 1))
    printf "  FAIL: %s — %s\n" "$1" "$2"
}

assert_status() {
    if [ "$2" = "$3" ]; then
        pass "$1"
    else
        fail "$1" "expected status $2, got $3"
    fi
}

assert_contains() {
    if printf '%s' "$2" | grep -qF "$3"; then
        pass "$1"
    else
        fail "$1" "expected to contain '$3'"
    fi
}

echo "=== staticomment integration tests ==="

# ── 1. Health check ──────────────────────────────────────────
echo ""
echo "--- Health check ---"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$STATICOMMENT_URL/health")
assert_status "GET /health returns 200" "200" "$STATUS"

BODY=$(curl -s "$STATICOMMENT_URL/health")
if [ "$BODY" = "ok" ]; then
    pass "GET /health body is 'ok'"
else
    fail "GET /health body is 'ok'" "got '$BODY'"
fi

# ── 2. Method enforcement ────────────────────────────────────
echo ""
echo "--- Method enforcement ---"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$STATICOMMENT_URL/health")
assert_status "POST /health returns 405" "405" "$STATUS"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$STATICOMMENT_URL/comment")
assert_status "GET /comment returns 405" "405" "$STATUS"

# ── 3. Invalid origin ────────────────────────────────────────
echo ""
echo "--- Origin validation ---"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Origin: http://evil.example.com" \
    -d "name=Test&body=Hello&slug=test-post&url=$REDIRECT_URL" \
    "$STATICOMMENT_URL/comment")
assert_status "Invalid origin returns 403" "403" "$STATUS"

# ── 4. No origin or referer ──────────────────────────────────

STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST \
    -d "name=Test&body=Hello&slug=test-post&url=$REDIRECT_URL" \
    "$STATICOMMENT_URL/comment")
assert_status "No origin or referer returns 403" "403" "$STATUS"

# ── 5. Redirect URL bad origin ───────────────────────────────

STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "name=Test&body=Hello&slug=test-post&url=http://evil.example.com/page" \
    "$STATICOMMENT_URL/comment")
assert_status "Redirect URL bad origin returns 403" "403" "$STATUS"

# ── 6. Referer fallback ──────────────────────────────────────
echo ""
echo "--- Referer fallback ---"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Referer: ${ALLOWED_ORIGIN}/blog/test-post" \
    -d "name=RefererTest&body=Referer+comment&slug=test-post&url=$REDIRECT_URL" \
    "$STATICOMMENT_URL/comment")
assert_status "Referer fallback accepted (303)" "303" "$STATUS"

# ── 7. Missing required fields ───────────────────────────────
echo ""
echo "--- Field validation ---"

# Missing name — has url so gets 303 error redirect
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "body=Hello&slug=test-post&url=$REDIRECT_URL" \
    "$STATICOMMENT_URL/comment")
assert_status "Missing name returns 303 error redirect" "303" "$STATUS"

# Missing url — no redirect target so gets 400
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "name=Test&body=Hello&slug=test-post" \
    "$STATICOMMENT_URL/comment")
assert_status "Missing url returns 400" "400" "$STATUS"

# ── 8. Invalid slug ──────────────────────────────────────────
echo ""
echo "--- Slug validation ---"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "name=Test&body=Hello&url=$REDIRECT_URL" \
    --data-urlencode "slug=../../../etc/passwd" \
    "$STATICOMMENT_URL/comment")
assert_status "Path traversal slug rejected (303)" "303" "$STATUS"

REDIR=$(curl -s -o /dev/null -w "%{redirect_url}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "name=Test&body=Hello&url=$REDIRECT_URL" \
    --data-urlencode "slug=..sneaky" \
    "$STATICOMMENT_URL/comment")
assert_contains "Invalid slug error in redirect" "$REDIR" "comment_error"

# ── 9. Invalid reply_to ──────────────────────────────────────
echo ""
echo "--- reply_to validation ---"

REDIR=$(curl -s -o /dev/null -w "%{redirect_url}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "name=Test&body=Hello&slug=test-post&url=$REDIRECT_URL" \
    --data-urlencode "reply_to=../evil" \
    "$STATICOMMENT_URL/comment")
assert_contains "Invalid reply_to rejected" "$REDIR" "comment_error"

# ── 10. Body too long ────────────────────────────────────────
echo ""
echo "--- Body length validation ---"

LONG_BODY=$(dd if=/dev/zero bs=10001 count=1 2>/dev/null | tr '\0' 'x')
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "name=Test&slug=test-post&url=$REDIRECT_URL" \
    --data-urlencode "body=$LONG_BODY" \
    "$STATICOMMENT_URL/comment")
assert_status "Body too long rejected (303)" "303" "$STATUS"

# ── 11. Successful comment ────────────────────────────────────
echo ""
echo "--- Successful comment ---"

RESULT=$(curl -s -o /dev/null -w "%{http_code}\n%{redirect_url}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "name=Integration+Test&body=This+is+a+test+comment&slug=test-post&url=$REDIRECT_URL" \
    "$STATICOMMENT_URL/comment")
STATUS=$(echo "$RESULT" | sed -n '1p')
REDIR=$(echo "$RESULT" | sed -n '2p')
assert_status "Successful comment returns 303" "303" "$STATUS"
assert_contains "Redirect contains #comment-submitted" "$REDIR" "#comment-submitted"

# ── 13. Comment with reply_to ─────────────────────────────────
echo ""
echo "--- Comment with reply_to ---"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "name=Reply+Test&body=This+is+a+reply&slug=test-post&url=$REDIRECT_URL&reply_to=some-comment-id" \
    "$STATICOMMENT_URL/comment")
assert_status "Comment with reply_to returns 303" "303" "$STATUS"

# ── 14. Comment with email ────────────────────────────────────
echo ""
echo "--- Comment with email ---"

STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "name=Email+Test&body=Comment+with+email&slug=test-post&url=$REDIRECT_URL&email=test@example.com" \
    "$STATICOMMENT_URL/comment")
assert_status "Comment with email returns 303" "303" "$STATUS"

# ── 16. Post existence (invalid slug) ─────────────────────────
echo ""
echo "--- Post existence validation ---"

REDIR=$(curl -s -o /dev/null -w "%{redirect_url}" \
    -X POST -H "Origin: $ALLOWED_ORIGIN" \
    -d "name=Test&body=Hello&slug=nonexistent-post&url=$REDIRECT_URL" \
    "$STATICOMMENT_URL/comment")
assert_contains "Nonexistent post rejected" "$REDIR" "Post+not+found"

# ── 12, 15, 17, 18. Git verification ─────────────────────────
echo ""
echo "--- Git verification ---"

export GIT_SSH_COMMAND="ssh -i /ssh-keys/id_ed25519 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"

CLONE_DIR=$(mktemp -d)
if ! git clone "git@${GIT_SERVER}:/home/git/repo.git" "$CLONE_DIR/repo" 2>/dev/null; then
    fail "Clone repo from git-server" "git clone failed"
    echo ""
    echo "==========================="
    printf "Results: %d passed, %d failed, %d total\n" "$PASS" "$FAIL" "$TOTAL"
    echo "==========================="
    exit 1
fi

COMMENT_DIR="$CLONE_DIR/repo/_data/comments/test-post"

# 12. Verify comment YAML files exist
COMMENT_COUNT=$(ls "$COMMENT_DIR"/*.yml 2>/dev/null | wc -l)
if [ "$COMMENT_COUNT" -gt 0 ]; then
    pass "Comment YAML files exist in repo ($COMMENT_COUNT found)"
else
    fail "Comment YAML files exist in repo" "no .yml files found"
fi

# Verify fields in a comment file
FIRST_COMMENT=$(ls "$COMMENT_DIR"/*.yml 2>/dev/null | head -1)
if [ -n "$FIRST_COMMENT" ]; then
    CONTENT=$(cat "$FIRST_COMMENT")
    assert_contains "Comment has name field" "$CONTENT" "name:"
    assert_contains "Comment has body field" "$CONTENT" "body:"
    assert_contains "Comment has date field" "$CONTENT" "date:"
    assert_contains "Comment has slug field" "$CONTENT" "slug:"
fi

# 13. Verify reply_to field
REPLY_COMMENT=$(grep -l "reply_to:" "$COMMENT_DIR"/*.yml 2>/dev/null | head -1 || true)
if [ -n "$REPLY_COMMENT" ]; then
    pass "Comment with reply_to field found"
    assert_contains "reply_to has correct value" "$(cat "$REPLY_COMMENT")" "some-comment-id"
else
    fail "Comment with reply_to field found" "no comment has reply_to"
fi

# 14. Verify email field
EMAIL_COMMENT=$(grep -l "email:" "$COMMENT_DIR"/*.yml 2>/dev/null | head -1 || true)
if [ -n "$EMAIL_COMMENT" ]; then
    pass "Comment with email field found"
    assert_contains "email has correct value" "$(cat "$EMAIL_COMMENT")" "test@example.com"
else
    fail "Comment with email field found" "no comment has email"
fi

# 15. Post existence (valid) — already proven by successful comment above
pass "Post existence validation (valid slug accepted)"

# 17. Git commit message format
COMMIT_MSG=$(cd "$CLONE_DIR/repo" && git log --oneline -1)
assert_contains "Commit message format" "$COMMIT_MSG" "Add comment on test-post"

# 18. Git commit author
COMMIT_AUTHOR=$(cd "$CLONE_DIR/repo" && git log --format="%an" -1)
assert_contains "Commit author is staticomment" "$COMMIT_AUTHOR" "staticomment"

rm -rf "$CLONE_DIR"

# ── Summary ───────────────────────────────────────────────────
echo ""
echo "==========================="
printf "Results: %d passed, %d failed, %d total\n" "$PASS" "$FAIL" "$TOTAL"
echo "==========================="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
