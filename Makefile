BIN := ~/.local/bin/todoist-cli

.PHONY: build install test vet

build:
	go build -o todoist-cli ./cmd/todoist-cli

install:
	go build -o $(BIN) ./cmd/todoist-cli

test:
	go test ./...

vet:
	go vet ./...
