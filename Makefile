BIN      := ~/.local/bin/todoist-cli
COV_OUT  := coverage.out
COV_MIN  := 70

# Packages that have tests and meaningful coverage targets.
# internal/config and internal/todoist wrap keychain/HTTP — excluded.
COV_PKGS := ./internal/db/... ./internal/state/... ./internal/tasks/...

.PHONY: build install test vet cover cover-html

build:
	go build -o todoist-cli ./cmd/todoist-cli

install:
	go build -o $(BIN) ./cmd/todoist-cli

test:
	go test ./...

vet:
	go vet ./...

cover:
	go test $(COV_PKGS) -coverprofile=$(COV_OUT) -covermode=atomic
	@go tool cover -func=$(COV_OUT) | tail -1
	@total=$$(go tool cover -func=$(COV_OUT) | tail -1 | awk '{print $$3}' | tr -d '%'); \
	 echo "Coverage: $${total}% (minimum: $(COV_MIN)%)"; \
	 if [ "$$(echo "$${total} < $(COV_MIN)" | bc)" = "1" ]; then \
	   echo "FAIL: coverage below minimum"; exit 1; \
	 fi

cover-html: cover
	go tool cover -html=$(COV_OUT)
