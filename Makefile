BIN      := ~/.local/bin/todoist-cli
COV_OUT  := coverage.out
COV_MIN  := 80

.PHONY: build install test test-live vet cover cover-html

build:
	go build -o todoist-cli ./cmd/todoist-cli

install:
	go build -o $(BIN) ./cmd/todoist-cli

test:
	go test ./...

# Needs TODOIST_TOKEN_A and TODOIST_TOKEN_B for two throwaway accounts.
test-live:
	go test -tags live -count=1 -v -run Live ./cmd/todoist-cli

vet:
	go vet ./...

cover:
	go test ./... -coverpkg=./... -coverprofile=$(COV_OUT) -covermode=atomic
	@go tool cover -func=$(COV_OUT) | tail -1
	@total=$$(go tool cover -func=$(COV_OUT) | tail -1 | awk '{print $$3}' | tr -d '%'); \
	 echo "Coverage: $${total}% (minimum: $(COV_MIN)%)"; \
	 if [ "$$(echo "$${total} < $(COV_MIN)" | bc)" = "1" ]; then \
	   echo "FAIL: coverage below minimum"; exit 1; \
	 fi

cover-html: cover
	go tool cover -html=$(COV_OUT)
