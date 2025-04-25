help: ## Display this help
	@egrep -h '\s##\s' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m  %-30s\033[0m %s\n", $$1, $$2}'

generate_migration:	## Generate a new migration
	@read -p "Enter migration name: " name;	\
	goose -dir repositories/migrations/ create $$name sql

generate_api_clients:	## Generate API clients
	go generate ./api-clients/convoy/generate.go
	
.PHONY: docker_services firebase_emulator launch_scenario ingest_data send_webhooks cron_scheduler workers reset_db

# Default environment file
ENV_FILE=.env

## Start docker services
docker_services:
	@set -e; \
	echo "Starting Docker services..."; \
	docker compose up -d

## Run Firebase emulator
firebase_emulator: 
	@echo "Starting Firebase emulator..."
	set -a && source $(ENV_FILE) && firebase --project "$$GOOGLE_CLOUD_PROJECT" emulators:start --import=./firebase-local-data --export-on-exit

## Run server with migrations
server:
	set -a && source $(ENV_FILE) && go run . --server --migrations

## Execute scheduled scenario job
launch_scenario:
	set -a && source $(ENV_FILE) && go run . --scheduled-executer

## Run data ingestion job
ingest_data:
	set -a && source $(ENV_FILE) && go run . --data-ingestion

## Send pending webhook events
send_webhooks:
	set -a && source $(ENV_FILE) && go run . --send-pending-webhook-events

## Launch cron job scheduler
cron_scheduler:
	set -a && source $(ENV_FILE) && go run . --cron-scheduler

## Launch task queue workers
workers:
	set -a && source $(ENV_FILE) && go run . --worker

## Reset database
reset_db:
	@set -e; \
	echo "Resetting database..."; \
	read -p "Are you sure you want to reset the database? This will delete all data. (y/N): " confirm; \
	if [ "$$confirm" != "y" ]; then \
		echo "Database reset canceled."; \
		exit 0; \
	fi; \
	echo "Stopping Docker services..."; \
	docker compose down; \
	echo "Removing Docker volumes..."; \
	docker volume rm marble-backend_postgres-db; \
	$(MAKE) docker_services
