COMPOSE := docker-compose
GO := go
GOTEST := $(GO) test -race -timeout 10s

test:
	@$(COMPOSE) up -d elasticsearch localstack && $(COMPOSE) run wait && $(GOTEST) ./...; ret=$$?; $(COMPOSE) down -v && exit $$ret
