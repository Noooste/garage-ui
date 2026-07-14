.PHONY: help build up down restart logs clean ps health dev-build dev-up dev-down dev-restart dev-logs prod-up prod-down prod-restart prod-logs push stop

# Variables
DOCKER_COMPOSE = docker compose
DOCKER_COMPOSE_DEV = docker compose -f docker-compose.dev.yml
DOCKER_COMPOSE_PROD = docker compose -f docker-compose.yml
IMAGE_NAME = noooste/garage-ui
IMAGE_TAG = latest

## help: Show this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: Build the Docker image locally
build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

## build-no-cache: Build the Docker image without cache
build-no-cache:
	docker build --no-cache -t $(IMAGE_NAME):$(IMAGE_TAG) .

## push: Push the Docker image to registry
push: build
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

## dev-build: Build and start development environment
dev-build:
	$(DOCKER_COMPOSE_DEV) build

## dev-up: Start development environment
dev-up:
	$(DOCKER_COMPOSE_DEV) up -d

## dev-down: Stop development environment
dev-down:
	$(DOCKER_COMPOSE_DEV) down

## dev-restart: Restart development environment
dev-restart: dev-down dev-up

## dev-logs: Show logs for development environment
dev-logs:
	$(DOCKER_COMPOSE_DEV) logs -f

## dev-logs-ui: Show logs for garage-ui in development
dev-logs-ui:
	$(DOCKER_COMPOSE_DEV) logs -f garage-ui

## dev-logs-garage: Show logs for garage in development
dev-logs-garage:
	$(DOCKER_COMPOSE_DEV) logs -f garage

## dev-shell: Open shell in garage-ui container (development)
dev-shell:
	$(DOCKER_COMPOSE_DEV) exec garage-ui sh

## prod-up: Start production environment
prod-up:
	$(DOCKER_COMPOSE_PROD) up -d

## prod-down: Stop production environment
prod-down:
	$(DOCKER_COMPOSE_PROD) down

## prod-restart: Restart production environment
prod-restart: prod-down prod-up

## prod-logs: Show logs for production environment
prod-logs:
	$(DOCKER_COMPOSE_PROD) logs -f

## prod-logs-ui: Show logs for garage-ui in production
prod-logs-ui:
	$(DOCKER_COMPOSE_PROD) logs -f garage-ui

## prod-logs-garage: Show logs for garage in production
prod-logs-garage:
	$(DOCKER_COMPOSE_PROD) logs -f garage

## prod-pull: Pull latest images for production
prod-pull:
	$(DOCKER_COMPOSE_PROD) pull

## up: Start default (production) environment
up: prod-up

## down: Stop all environments
down:
	$(DOCKER_COMPOSE_PROD) down
	$(DOCKER_COMPOSE_DEV) down

## stop: Stop all containers
stop:
	$(DOCKER_COMPOSE_PROD) stop
	$(DOCKER_COMPOSE_DEV) stop

## restart: Restart default (production) environment
restart: prod-restart

## logs: Show logs for default (production) environment
logs: prod-logs

## ps: Show running containers
ps:
	docker ps -a --filter "name=garage"

## health: Check health status of containers
health:
	@echo "=== Container Status ==="
	@docker ps --filter "name=garage" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

## clean: Remove all containers, volumes, and images
clean:
	$(DOCKER_COMPOSE_PROD) down -v --remove-orphans
	$(DOCKER_COMPOSE_DEV) down -v --remove-orphans
	docker system prune -f

## clean-volumes: Remove all volumes (WARNING: deletes data)
clean-volumes:
	$(DOCKER_COMPOSE_PROD) down -v
	$(DOCKER_COMPOSE_DEV) down -v
	rm -rf ./meta ./data

## rebuild: Clean rebuild of development environment
rebuild: dev-down dev-build dev-up

## install: Initial setup - create necessary directories
install:
	@echo "Creating necessary directories..."
	@mkdir -p meta data
	@echo "Setup complete. Edit garage.toml and config.yaml before starting."

## update: Pull latest changes and rebuild
update: prod-pull prod-restart

.DEFAULT_GOAL := help

.PHONY: test test-race test-cover test-smoke

## test: Run backend unit tests
test:
	cd backend && go test ./...

## test-race: Run backend unit tests with the race detector
test-race:
	cd backend && go test -race ./...

## test-cover: Run backend unit tests with coverage and enforce the coverage gate
test-cover:
	cd backend && go test -coverprofile=../coverage.out -coverpkg=./... ./...
	bash scripts/coverage-gate.sh coverage.out

## test-smoke: Run the docker compose smoke test (requires Docker + compose v2)
test-smoke:
	cd backend && go test -tags=smoke -timeout 10m ./tests/smoke/...

docs:
	@echo "Generating documentation..."
	@mkdir -p docs
	@echo "Documentation generated in the 'docs' directory."
	swag init -g backend/cmd/garage-ui/main.go -o docs --parseDependency --parseInternal