.PHONY: build test run clean help

BINARY_NAME=spotseek
GO=go

help: ## Show this help message
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@egrep '^(.+)\:\ ##\ (.+)' ${MAKEFILE_LIST} | column -t -c 2 -s ':#'

build: ## Build the application
	$(GO) build -o $(BINARY_NAME) cmd/main.go

run: build ## Run the application in server mode
	./$(BINARY_NAME) serve

test: ## Run tests
	$(GO) test -v ./...

clean: ## Remove binary and test cache
	rm -f $(BINARY_NAME)
	$(GO) clean -testcache
