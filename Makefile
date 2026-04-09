BINARY    := kluster-compare
GO        := go
GOFLAGS   :=
LDFLAGS   := -s -w

.PHONY: build run test clean install fmt vet lint

build:
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) .

run: build
	./$(BINARY) $(ARGS)

test:
	$(GO) test ./...

clean:
	rm -f $(BINARY)
	rm -rf dist/

install: build
	cp $(BINARY) $(GOPATH)/bin/ 2>/dev/null || cp $(BINARY) ~/go/bin/

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint: vet
	@which golangci-lint > /dev/null 2>&1 || echo "install golangci-lint: https://golangci-lint.run/welcome/install/"
	golangci-lint run ./...
