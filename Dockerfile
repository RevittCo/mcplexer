# ── Stage 1: Build web UI ──
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ── Stage 2: Build Go binary ──
FROM golang:1.25rc1-alpine AS go-builder
RUN apk add --no-cache git
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /mcplexer ./cmd/mcplexer

# ── Stage 3: Runtime ──
FROM alpine:3.21
RUN apk add --no-cache ca-certificates nodejs npm
WORKDIR /app
COPY --from=go-builder /mcplexer /usr/local/bin/mcplexer

# Default config location
VOLUME ["/app/data"]
ENV MCPLEXER_DB_DSN=/app/data/mcplexer.db
ENV MCPLEXER_CONFIG=/app/data/mcplexer.yaml

EXPOSE 3333

ENTRYPOINT ["mcplexer"]
CMD ["serve", "--mode=http", "--addr=:3333"]
