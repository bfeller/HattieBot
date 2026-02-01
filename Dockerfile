# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /hattiebot ./cmd/hattiebot && go build -o /register-tool ./cmd/register-tool

# Runtime stage
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  curl \
  docker.io \
  golang \
  jq \
  && rm -rf /var/lib/apt/lists/*
# Install Autohand CLI (recommended method for Linux)
RUN curl -fsSL https://autohand.ai/install.sh | bash
WORKDIR /workspace
# Default mount points: -v for data, config, and workspace
# /data: hattiebot.db and config
# /workspace: agent working directory (tools/, bin/, docs/)
ENV HATTIEBOT_CONFIG_DIR=/data
EXPOSE 8080
COPY --from=builder /hattiebot /usr/local/bin/hattiebot
COPY --from=builder /register-tool /usr/local/bin/register-tool
ENTRYPOINT ["/usr/local/bin/hattiebot"]
