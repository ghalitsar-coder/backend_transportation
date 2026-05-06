FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build binary statis agar bisa berjalan di scratch/alpine tanpa libc
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-w -s" -o main ./cmd/server/main.go

FROM alpine:3.20 AS runtime
WORKDIR /app

# ca-certificates diperlukan untuk HTTPS calls ke TomTom API
RUN apk add --no-cache ca-certificates tzdata

# Copy binary dari builder
COPY --from=builder /app/main .

# Buat direktori uploads (akan di-mount jika pakai volume)
RUN mkdir -p ./uploads/reports

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
	CMD wget -q --spider http://localhost:8080/health || exit 1

CMD ["./main"]