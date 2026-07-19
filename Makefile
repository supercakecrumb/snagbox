.PHONY: build run test lint docker tidy vet

build:
	go build -o bin/snagbox ./cmd/snagbox

run: build
	./bin/snagbox

test:
	go test ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

tidy:
	go mod tidy

docker:
	docker build -t snagbox:local .
