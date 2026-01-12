# Build stage
FROM golang:1.23.4-alpine as builder

WORKDIR /app

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main cmd/proxy/main.go

# Runtime stage
FROM alpine:latest as runner

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/main /app/main

EXPOSE 5432

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD test -f /app/main || exit 1

CMD ["./main"]
