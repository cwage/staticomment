# Build stage
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /staticomment .

# Runtime stage
FROM alpine:3.21
RUN apk add --no-cache git openssh-client
# Pinned SSH host keys for common git hosting providers.
# The application will ssh-keyscan the configured host at startup for any host
# not already present, and refresh on failure for key rotation.
RUN mkdir -p /app/.ssh && cat > /app/.ssh/known_hosts <<'EOF'
github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
EOF
COPY --from=build /staticomment /app/staticomment
WORKDIR /app
ENTRYPOINT ["/app/staticomment"]
