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

build-race: ## Build the application with race detector
	$(GO) build -race -o $(BINARY_NAME) cmd/main.go

run: build ## Run the application in server mode
	./$(BINARY_NAME) serve

run-tui: build ## Run the application in TUI mode
	./$(BINARY_NAME) tui

run-race: build-race ## Run the application in server mode with race detector
	./$(BINARY_NAME) tui

test: ## Run tests
	$(GO) test -v ./...

clean: ## Remove binary and test cache
	rm -f $(BINARY_NAME)
	$(GO) clean -testcache
