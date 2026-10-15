# Multi-stage Dockerfile for Dormitory Management System

# Stage 1: Build binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o dormitory-server .

# Stage 2: Minimal runtime
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Bangkok

# Copy binary and assets from builder
COPY --from=builder /app/dormitory-server /app/
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/static /app/static
COPY --from=builder /app/locales /app/locales
COPY --from=builder /app/migrations /app/migrations

# Create uploads directory
RUN mkdir -p /app/uploads

EXPOSE 8080

CMD ["/app/dormitory-server"]
