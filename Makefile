.PHONY: all lint test vet check mdns-publisher

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

mocks_test.go: types.go
	go tool go.uber.org/mock/mockgen -source=types.go -destination=mocks_test.go -package=main

clean:
	rm -f $(BINARY) 