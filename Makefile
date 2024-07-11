help: ## Display this help
	@egrep -h '\s##\s' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m  %-30s\033[0m %s\n", $$1, $$2}'

generate_migration:	## Generate a new migration
	@read -p "Enter migration name: " name;	\
	goose -dir repositories/migrations/ create $$name sql

generate_api_clients:	## Generate API clients
	go generate ./api-clients/convoy/generate.go
