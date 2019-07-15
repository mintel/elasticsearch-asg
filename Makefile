COMPOSE := docker-compose
GO := go
GOTEST := $(GO) test -race -timeout 10s

# Run full test suite.
test:
	@$(COMPOSE) up -d elasticsearch localstack && $(COMPOSE) run wait && $(GOTEST) ./...; ret=$$?; $(COMPOSE) down -v && exit $$ret

# Run tests sans Docker integration tests.
test-short:
	$(GOTEST) -short ./...
