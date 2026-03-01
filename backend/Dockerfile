# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o api .

# Final stage — distroless for minimal image with CA certs
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/api /api

EXPOSE 3001

ENTRYPOINT ["/api"]
