.PHONY: all lint test vet check mdns-publisher integration-test

BINARY := mdns-publisher

all: $(BINARY)

$(BINARY):
	go build -o $(BINARY) ./...

lint:
	golangci-lint run ./...

test: mocks_test.go
	go test -timeout=5s -shuffle=on -race -coverprofile=coverage.out ./...

coverage:
	go tool cover -html=coverage.out

vet:
	go vet ./...

check: lint vet test

# Integration tests with testcontainers (requires Docker)
integration-test:
	INTEGRATION_TEST=1 go test -timeout=5m -tags=integration -v ./...

# Quick integration test (single run)
integration-test-quick:
	INTEGRATION_TEST=1 go test -timeout=5m -tags=integration -run TestIntegration_GetHostnamesFromRealDocker -v ./...

mocks_test.go: types.go
	go tool go.uber.org/mock/mockgen -source=types.go -destination=mocks_test.go -package=main

clean:
	rm -f $(BINARY) 