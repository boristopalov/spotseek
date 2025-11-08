.PHONY: build test run clean help

BINARY_NAME=spotseek
GO=go
ARGS?=

help: ## Show this help message
	@echo 'Usage:'
	@echo '  make <target> [ARGS="flags"]'
	@echo ''
	@echo 'Examples:'
	@echo '  make run'
	@echo '  make run ARGS="-username myuser -password mypass"'
	@echo '  make run-tui ARGS="-username myuser"'
	@echo ''
	@echo 'Targets:'
	@egrep '^(.+)\:\ ##\ (.+)' ${MAKEFILE_LIST} | column -t -c 2 -s ':#'

build: ## Build the application
	$(GO) build -o $(BINARY_NAME) cmd/main.go

build-race: ## Build the application with race detector
	$(GO) build -race -o $(BINARY_NAME) cmd/main.go

run: build ## Run the application in server mode (use ARGS="..." to pass flags)
	./$(BINARY_NAME) serve $(ARGS)

run-tui: build ## Run the application in TUI mode (use ARGS="..." to pass flags)
	./$(BINARY_NAME) tui $(ARGS)

run-race: build-race ## Run the application in server mode with race detector (use ARGS="..." to pass flags)
	./$(BINARY_NAME) tui $(ARGS)

test: ## Run tests
	$(GO) test -v ./...

clean: ## Remove binary and test cache
	rm -f $(BINARY_NAME)
	$(GO) clean -testcache
