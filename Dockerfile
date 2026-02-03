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
  git \
  jq \
  unzip \
  && rm -rf /var/lib/apt/lists/*
# Install Go 1.22 (go.mod requires 1.21+; Debian's apt golang is 1.19)
RUN curl -sL https://go.dev/dl/go1.22.4.linux-amd64.tar.gz | tar -C /usr/local -xz \
  && ln -sf /usr/local/go/bin/go /usr/bin/go \
  && ln -sf /usr/local/go/bin/gofmt /usr/bin/gofmt
# Install Bun (autohand-cli requires Bun; Node has ESM/fs-extra issues)
RUN curl -fsSL https://bun.sh/install | bash
ENV PATH="/usr/local/bin:/root/.bun/bin:${PATH}"

# Install Autohand CLI via Bun (curl binary has react-devtools-core bundling bug)
RUN bun install -g autohand-cli
# autohand npm package uses node shebang; override with bun wrapper so it runs correctly
RUN echo '#!/bin/sh\nexec /root/.bun/bin/bun /root/.bun/bin/autohand "$@"' > /usr/local/bin/autohand && chmod +x /usr/local/bin/autohand
WORKDIR /workspace
# Default mount points: -v for data, config, and workspace
# /data: hattiebot.db and config
# /workspace: agent working directory (tools/, bin/, docs/)
ENV HATTIEBOT_CONFIG_DIR=/data CONFIG_DIR=/data
EXPOSE 8080
COPY --from=builder /hattiebot /usr/local/bin/hattiebot
COPY --from=builder /register-tool /usr/local/bin/register-tool
ENTRYPOINT ["/usr/local/bin/hattiebot"]
