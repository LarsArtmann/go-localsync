set shell := ["bash", "-c"]

version := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`

default: build

build:
    go build -ldflags "-X main.version={{version}} -X main.commit={{commit}} -X main.date={{date}}" -o bin/gh-sync ./cmd/examples/github-sync

run *args: build
    ./bin/gh-sync {{args}}

test:
    go test -v ./...

test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

lint:
    go vet ./...
    go fmt ./...

clean:
    rm -rf bin/
    rm -f *.db

dev *args: build
    ./bin/gh-sync -verbose {{args}}

install: build
    cp bin/gh-sync ~/bin/

sqlc:
    sqlc generate

version:
    @echo "go-localsync"
    @go version

deps:
    go mod download
    go mod tidy
