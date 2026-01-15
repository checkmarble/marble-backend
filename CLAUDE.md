# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Setup
```bash
# Install Go (or use mise: https://mise.jdx.dev/)
# Create .env file from .env.example and configure values
cp .env.example .env

# Start dependencies (PostgreSQL and other services)
docker compose up -d

# Setup Firebase emulator (for local development)
firebase --project [GOOGLE_CLOUD_PROJECT] emulators:start --import=./firebase-local-data --export-on-exit
```

### Running the Application

The application has four main components that run from the same binary:

```bash
# API Server (most common during development)
go run . --migrations --server

# Background Worker (handles async jobs)
go run . --worker

# Analytics Server (separate analytics API)
go run . --analytics

# Migrations only
go run . --migrations
```

With mise:
```bash
mise exec -- go run . --migrations --server
mise exec -- go run . --worker
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./usecases
go test ./repositories

# Run a specific test
go test ./usecases -run TestScenarioUsecase

# Run integration tests (requires Docker)
go test ./integration_test

# Run with verbose output
go test -v ./...
```

### Database Migrations

Migrations use **goose** and are located in `repositories/migrations/` (192+ SQL files).

```bash
# Create a new migration (from repositories/migrations/ directory)
cd repositories/migrations
goose create add_some_column sql

# Migrations run automatically with --migrations flag
go run . --migrations --server
```

**Common issue**: Migrations can become misordered when two PRs add migrations simultaneously. If this happens, install goose CLI (`brew install goose`), configure environment variables, and roll back:
```bash
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="user=postgres dbname=marble host=localhost password=marble"
export GOOSE_MIGRATION_DIR="./repositories/migrations"
goose down
```

**Reset database completely**:
```bash
docker compose down && docker volume rm marble-backend_postgres-db && docker compose up -d
```

### Code Generation
```bash
# Generate API clients
make generate_api_clients
```

## Architecture Overview

### Component Structure

This is a **monolithic Go backend** split into four discrete runtime components:
- **API Server** (Gin HTTP framework) - handles REST API requests
- **Worker** (River job queue) - processes async background jobs
- **Analytics Server** - separate analytics API endpoints
- **Migrations** - database schema management

All components share the same codebase and use the same PostgreSQL database.

### Package Organization (Clean Architecture)

```
api/           → HTTP layer: handlers, routes, middleware (internal/web app API)
pubapi/        → Public API: separate handlers and DTOs for client-facing API
  ├── v1/      → V1 public API handlers
  └── openapi/ → OpenAPI specification
usecases/      → Business logic layer (90+ usecase implementations)
repositories/  → Data access layer (100+ repository implementations)
models/        → Domain models and types (91 files)
dto/           → Data transfer objects for internal API contracts (64 files)
jobs/          → Worker job implementations
infra/         → Infrastructure setup (DB, Firebase, GCP, tracing)
cmd/           → Component bootstrap code (server, worker, migrations, analytics)
```

**Important**: The `pubapi/` folder contains the **public client-facing API** with its own handlers and DTOs, separate from the internal web app API in `api/`.

### Dependency Flow

```
HTTP Request → Handler (api/ or pubapi/)
    → Usecase (usecases/)
    → Repository (repositories/)
    → Database (PostgreSQL)
```

Key principles:
- **Handlers** are thin - they parse requests, call usecases, format responses
- **Usecases** contain business logic - they orchestrate repositories and domain rules
- **Repositories** handle data access - they build queries and map to domain models
- **Models** are shared across all layers

### Dependency Injection Pattern

The codebase uses **functional options pattern** for dependency injection:

1. **Repositories** are initialized first with database pool:
   ```go
   repos := repositories.NewRepositories(dbPool, gcpConfig,
       repositories.WithMetabase(...),
       repositories.WithRiverClient(...),
   )
   ```

2. **Usecases** are initialized with repositories:
   ```go
   uc := usecases.NewUsecases(repos,
       usecases.WithAppName(...),
       usecases.WithLicense(...),
       usecases.WithFirebaseAdmin(...),
   )
   ```

3. **Feature-specific usecases** are created via factory methods:
   ```go
   caseUsecase := usecases.NewCaseUsecase()
   decisionUsecase := usecases.NewDecisionUsecase()
   ```

### Multi-Organization (Multi-Tenancy)

- Each organization has its own **client database** for customer data
- The main **Marble database** stores metadata, scenarios, users, and configuration
- `organization_id` is used throughout for tenant isolation
- Per-organization River job queues ensure workload isolation

### Database Structure

Two-level database system:
- **Marble DB**: Core platform data (organizations, users, scenarios, decisions metadata)
- **Client DBs**: Per-organization customer data (ingested tables, custom data models)

Query execution uses the **Executor pattern** to abstract transactions:
- `Executor` - regular database connection
- `TxExecutor` - transactional context
- Both implement the same interface, allowing seamless switching

### Worker & Async Jobs

**Job Queue**: River (PostgreSQL-backed, replaces the need for Redis/RabbitMQ)

**Common job types**:
- `AsyncDecisionArgs` - evaluate decisions asynchronously
- `IndexCreationArgs` - create database indexes for ingested data
- `CaseReviewArgs` - AI-powered case analysis
- `ContinuousScreeningDoScreeningArgs` - run screening checks
- `DecisionWorkflowArgs` - automated decision workflows

Jobs are defined in `models/river_job.go` and implemented in `jobs/` and various usecase files.

**Adding a new job**:
1. Define args struct in `models/river_job.go`
2. Implement worker in appropriate usecase or `jobs/` package
3. Register worker in `cmd/worker.go`
4. Enqueue jobs from usecases using `riverClient.InsertTx()` or `riverClient.Insert()`

### Error Handling

**Custom error types** in `models/errors.go` map to HTTP status codes:
- `BadParameterError` → 400
- `NotFoundError` → 404
- `ForbiddenError` → 403
- `ConflictError` → 409
- etc.

**Error wrapping**: Uses `github.com/cockroachdb/errors` for enhanced error context:
```go
return errors.Wrap(err, "failed to create scenario")
```

**Centralized error presentation**: `api/present_error.go` formats all errors consistently and sends them to Sentry.

### Routing & Authentication

**Main route file**: `api/routes.go`

**Authentication methods**:
- **JWT tokens** (issued by backend) - for web app users
- **API keys** - for public API (`/v1/*` endpoints in `pubapi/`)
- **Firebase Auth** or **OIDC** - identity providers

**Route organization**:
- Public client API: `/v1/*` - API key authenticated (handlers in `pubapi/v1/`)
- Internal web app API: organized by feature (`/decisions/*`, `/scenarios/*`, etc.) - handlers in `api/`
- Health checks: `/health`, `/liveness` - unauthenticated

**Middleware stack** (defined in `api/router.go`):
- Panic recovery
- Sentry error tracking
- CORS
- Request logging
- Context injection (logger, segment client)
- OpenTelemetry tracing

### Key Domain Concepts

**Scenarios**: Business rules that evaluate decisions
- **Scenario Iterations**: Versions of a scenario (draft → published)
- **Scenario Executions**: Running scenarios on ingested data
- **Decision Workflows**: Automated actions based on decision outcomes

**Decisions**: Results of scenario evaluations with risk scores

**Cases**: Investigations triggered by risky decisions

**Ingestion**: Importing customer data into organization-specific databases

**Screenings**: Sanctions/watchlist checks (via OpenSanctions integration)

**Continuous Screening**: Ongoing monitoring of entities for sanctions list changes

## Important Files & Patterns

### Main Entry Point
- `main.go` - Parses flags and routes to appropriate component (server/worker/migration/analytics)

### Routing & Handlers
- `api/routes.go` - Internal web app route definitions
- `api/handle_*.go` - Internal endpoint handlers (60+ files)
- `pubapi/v1/` - Public client API handlers
- `dto/` - DTOs for internal API
- `pubapi/` - DTOs and handlers for public client API

### Core Business Logic
- `usecases/usecases.go` - Main usecase factory and dependency container
- `usecases/*_usecase.go` - Feature-specific business logic

### Data Access
- `repositories/repositories.go` - Main repository factory
- `repositories/*_repository.go` - Individual data access implementations
- `repositories/migrations/` - SQL migration files (192+ migrations)

### Configuration
- `.env.example` - All environment variables with documentation
- `infra/config.go` - Configuration structures
- `cmd/config.go` - Command-specific configuration

### Testing
- `integration_test/` - End-to-end integration tests
- `*_test.go` - Unit tests throughout the codebase
- `mocks/` - Generated mocks (via mockery)
- Integration tests use dockertest to spin up PostgreSQL containers

## Development Guidelines

### Adding a New Feature

1. **Define the model** in `models/`
2. **Create repository** in `repositories/` with interface at top of file
3. **Implement usecase** in `usecases/`
4. **Add handler** in `api/handle_*.go` (or `pubapi/v1/` for public API)
5. **Register routes** in `api/routes.go`
6. **Create migration** if schema changes needed
7. **Add DTOs** in `dto/` (or in `pubapi/` for public API)

### Error Handling Best Practices

```go
// Wrap errors with context
if err != nil {
    return errors.Wrap(err, "failed to create scenario")
}

// Use domain error types
if notFound {
    return models.NotFoundError
}

// Check error types
if errors.Is(err, models.ForbiddenError) {
    // handle forbidden
}
```

### Transaction Management

```go
// Execute in transaction
tx, err := executor.Transaction(ctx, func(tx repositories.Transaction) error {
    // All operations in this function use the same transaction
    if err := repo.CreateThing(ctx, tx, thing); err != nil {
        return err  // Automatic rollback
    }
    return nil  // Automatic commit
})
```

### Organization Context

Most usecases require organization context:
```go
// Credentials carry organization_id and user permissions
creds := credentials.Get(ctx)
orgId := creds.OrganizationId

// Validate permissions
if err := security.CanRead(ctx, creds, resource); err != nil {
    return models.ForbiddenError
}
```

### Logging

Uses `log/slog` structured logging:
```go
logger := utils.GetLoggerFromContext(ctx)
logger.InfoContext(ctx, "processing decision",
    "decision_id", decisionId,
    "scenario_id", scenarioId,
)
```

### Testing Patterns

**Integration tests** (`integration_test/`):
- Use dockertest to spin up real PostgreSQL
- Run full migrations
- Test via HTTP using httptest
- Use mocked Firebase authentication

**Unit tests**:
- Mock repositories using generated mocks in `mocks/`
- Test business logic in isolation
- Use `testify/assert` and `testify/require`

## External Integrations

### Firebase (Authentication)
- Used as identity provider for user authentication
- Emulator available for local development
- Client SDK in `repositories/idp/firebase_client.go`

### OpenSanctions (Sanctions Screening)
- External API for sanctions/watchlist checks
- Can be self-hosted or use SaaS
- Client in `infra/opensanctions.go`

### Convoy (Webhooks)
- Webhook delivery service
- Handles retries and delivery guarantees
- Client in `infra/convoy.go`

### Google Cloud Platform
- BigQuery for analytics
- Cloud Storage for file uploads
- Cloud Trace for distributed tracing
- Service account authentication via `GOOGLE_APPLICATION_CREDENTIALS`

### Metabase (Analytics Dashboards)
- Embedded analytics dashboards
- JWT-based authentication
- Configuration in `.env`

### Segment (Product Analytics)
- Event tracking for product analytics
- Can be disabled with `DISABLE_SEGMENT=true`

## API Documentation

- Public API docs: https://docs.checkmarble.com/reference/introduction-1
- OpenAPI spec: Referenced in frontend repository and `pubapi/openapi/`
- Internal routes: See `api/routes.go` for complete list

## License & Feature Flags

The application supports multiple license tiers:
- `LICENSE_KEY` environment variable
- License validation in `usecases/license.go`
- Feature gating based on license entitlements
- Premium features include AI agent, continuous screening, advanced analytics
