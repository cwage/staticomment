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
RUN mkdir -p /app/.ssh && ssh-keyscan github.com >> /app/.ssh/known_hosts 2>/dev/null
COPY --from=build /staticomment /app/staticomment
WORKDIR /app
ENTRYPOINT ["/app/staticomment"]
