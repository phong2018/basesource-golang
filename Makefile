.PHONY: build vet test lint check

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./internal/... -v -race -count=1

lint:
	golangci-lint run ./...

check: vet test lint
