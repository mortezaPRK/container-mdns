.PHONY: all lint test vet check mdns-publisher integration-test

BINARY := mdns-publisher

all: $(BINARY)

$(BINARY):
	go build -o $(BINARY) ./...

lint:
	golangci-lint run ./...

test: mocks_test.go
	go test -short -timeout=5s -shuffle=on -race -coverprofile=coverage.out ./...

coverage:
	go tool cover -html=coverage.out

vet:
	go vet ./...

check: lint vet test

# Integration tests with testcontainers
integration-test:
	go test -timeout=5m -v ./...

# Quick integration test (single run)
integration-test-quick:
	go test -timeout=5m -run TestIntegration_GetHostnamesFromRealDocker -v ./...

mocks_test.go: types.go
	go run go.uber.org/mock/mockgen@latest -source=types.go -destination=mocks_test.go -package=main

clean:
	rm -f $(BINARY) 