# ── Stage 1 : build the React frontend ──────────────────────────────────────
FROM node:22-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# ── Stage 2 : build the Go binary ────────────────────────────────────────────
FROM golang:1.26-alpine AS go-builder

WORKDIR /app

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source (excluding web/ – handled below)
COPY . .

# Inject the frontend build produced by stage 1
COPY --from=frontend-builder /app/web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o karasu .

# ── Stage 3 : minimal runtime image ──────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=go-builder /app/karasu ./karasu

EXPOSE 8080

ENTRYPOINT ["/app/karasu"]
