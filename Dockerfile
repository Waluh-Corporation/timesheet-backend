FROM golang:1.25-alpine AS builder

RUN apk update && apk add --no-cache git

WORKDIR /app

# Download dependencies.
COPY go.mod go.sum* ./
RUN go mod download || true

# Copy source code and templates.
COPY . .

# Build Go binary statically.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# Runtime stage.
FROM alpine:latest

# ca-certificates for outbound TLS, tzdata so Asia/Jakarta resolves for the cron.
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /app/templates ./templates

EXPOSE 8080

ENV PORT=8080
ENV GIN_MODE=release

CMD ["./main"]
