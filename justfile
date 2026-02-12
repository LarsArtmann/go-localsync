set shell := ["bash", "-c"]

default: build

build:
    go build -o bin/gh-sync ./cmd/gh-sync

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
