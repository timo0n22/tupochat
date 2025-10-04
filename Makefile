.PHONY: help build run stop clean test fmt vet docker-build docker-up docker-down logs

# Variables
APP_NAME=tupochat
DOCKER_IMAGE=ghcr.io/$(shell git config user.name | tr '[:upper:]' '[:lower:]')/$(APP_NAME)
VERSION=$(shell git describe --tags --always --dirty)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build binary
	@echo "Building $(APP_NAME)..."
	@go build -ldflags="-w -s" -o $(APP_NAME) .
	@echo "Build complete!"

run: ## Run locally
	@echo "Starting $(APP_NAME)..."
	@go run main.go

stop: ## Stop all Docker containers
	@echo "Stopping containers..."
	@docker compose down

clean: ## Clean builds and containers
	@echo "Cleaning..."
	@rm -f $(APP_NAME)
	@docker compose down -v
	@docker system prune -f
	@echo "Clean complete!"

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete!"

vet: ## Vet code
	@echo "Vetting code..."
	@go vet ./...
	@echo "Vet complete!"

lint: fmt vet ## Full code check

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .
	@echo "Docker image built!"

docker-up: ## Start with docker-compose
	@echo "Starting with docker-compose..."
	@docker compose up -d
	@echo "Services started!"
	@echo "Check logs: make logs"

docker-down: ## Stop docker-compose
	@echo "Stopping docker-compose..."
	@docker compose down
	@echo "Services stopped!"

docker-restart: docker-down docker-up ## Restart docker-compose

logs: ## Show logs
	@docker compose logs -f chat

db-shell: ## Connect to PostgreSQL
	@docker compose exec db psql -U postgres -d tupochatdb

db-reset: ## Reset database
	@echo "Resetting database..."
	@docker compose exec db psql -U postgres -d tupochatdb -f /docker-entrypoint-initdb.d/init.sql
	@echo "Database reset!"

dev: docker-up logs ## Run in development mode

prod-build: ## Build production image
	@echo "Building production image..."
	@docker build \
		--platform linux/amd64,linux/arm64 \
		-t $(DOCKER_IMAGE):$(VERSION) \
		-t $(DOCKER_IMAGE):latest \
		.
	@echo "Production image built!"

prod-push: prod-build ## Push to registry
	@echo "Pushing to registry..."
	@docker push $(DOCKER_IMAGE):$(VERSION)
	@docker push $(DOCKER_IMAGE):latest
	@echo "Pushed to registry!"

install-deps: ## Install dependencies
	@echo "Installing dependencies..."
	@g
