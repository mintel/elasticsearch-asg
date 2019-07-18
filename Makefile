GO := go
GOTEST := $(GO) test -race -timeout 5m

# Run full test suite.
test:
	$(GOTEST) ./...

# Run tests sans Docker integration tests.
test-short:
	$(GOTEST) -short ./...
