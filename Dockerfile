# ── Stage 1: build zk-serve ──────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache curl

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Download embedded JS assets and compile a static, stripped binary.
RUN curl -fsSL https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js \
      -o internal/server/static/htmx.min.js && \
    curl -fsSL https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js \
      -o internal/server/static/mermaid.min.js

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/zk-serve ./cmd/zk-serve

# ── Stage 2: fetch zk CLI ─────────────────────────────────────────────────────
FROM alpine:3.21 AS zk-dl

ARG ZK_VERSION=v0.15.2
ARG TARGETARCH

RUN apk add --no-cache curl tar && \
    curl -fsSL "https://github.com/zk-org/zk/releases/download/${ZK_VERSION}/zk-${ZK_VERSION}-alpine-${TARGETARCH}.tar.gz" \
      | tar -xz -C /usr/local/bin zk && \
    chmod +x /usr/local/bin/zk

# ── Stage 3: final minimal image ──────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates git && \
    adduser -D -u 1000 zk

COPY --from=builder   /bin/zk-serve        /usr/local/bin/zk-serve
COPY --from=zk-dl     /usr/local/bin/zk    /usr/local/bin/zk

# Notebook is expected to be mounted at /notebook.
VOLUME ["/notebook"]
EXPOSE 8080

USER zk
ENTRYPOINT ["zk-serve"]
CMD ["--addr", ":8080", "--notebook", "/notebook"]
