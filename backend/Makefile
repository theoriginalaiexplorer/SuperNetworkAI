.PHONY: dev build swagger migrate-up migrate-down sqlc test

dev:
	air -c .air.toml

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o api .

swagger:
	swag init -g main.go --output docs/

migrate-up:
	migrate -path db/migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path db/migrations -database "$(DATABASE_URL)" down 1

sqlc:
	sqlc generate

test:
	go test ./...
