# Introduction

This repo is the Marble backend implementation:

- 1 single Go app
- Postgres DB

## Getting Started

### Requirements

[Install Go](https://go.dev/doc/install) on your laptop (see the version in go.mod).

> NB: To handle different versions, you can look at [Managing Go installations](https://go.dev/doc/manage-install) or use a version manager tool like [asdf](https://github.com/kennyp/asdf-golang)

You may also need to [install the gcloud CLI](https://cloud.google.com/sdk/docs/install) in order to interact with deployed environments.

> NB: the staging GCP project is `tokyo-country-381508`- you may need to access it to test some features that depend on cloud infrastructure (you may need to ask for permissions).

### Deployment

#### Prerequisites

Install firebase-tools (`npm install -g firebase-tools`)

```sh
firebase login
firebase init
```

### Firebase emulator suite for local development

[Install the emulator suite](https://firebase.google.com/docs/emulator-suite)

Then start it using:

```sh
firebase --project staging emulators:start --import=./firebase-local-data
```

Connect in the backoffice using: `admin@checkmarble.com` (marble admin user created by default)
Connect in the frontend using: `jbe@zorg.com` (admin of an organization created by default)

#### How to add data to ./firebase-local-data

- Run firebase emulator with paramater: `--export-on-exit`
  `firebase --project staging emulators:start --import=./firebase-local-data --export-on-exit`
- Add user, change options...
- Exit the emulator
- commit

### Lauch the project

#### Setup the DB

You should first start the local DB to run the backend server:

`docker compose up -d` : launch the postgres DB used by the backend.
Creates a `marble-backend_postgres-db` volume to store PG data.

`docker volume rm marble-backend_postgres-db` deletes the PG volume, useful to reset the app to a known state

In practice, this single-line will delete the stack and create a new one:
`docker compose down && docker volume rm marble-backend_postgres-db && docker compose up -d`

#### Local (VS Code)

The recommended way to run the backend is to run `Migrate and Launch (.env.local)` in the VSCode "Run and debug" tab (especially usefull to dev, as the task launch a debugger):

- Start a DB using docker compose
- Lauch the debug task `Migrate and Launch (.env.local)` (VS Code) that will migrate the DB and start the server.
- Three other debug tasks exist that execute the actions running as batches on the cloud: batch data ingestion, scheduling of scenarios, execution of scheduled scenarios.

You can also run the go service directly in the terminal.

- Create your local `.env` using the provided `.env.local`.
  - To create a `AUTHENTICATION_JWT_SIGNING_KEY` run `openssl genrsa -out signing.pem 2048` and save the value as a one liner using `\n` for line breaks, or take the one from `.env.local` - you may need to work out how to export multi-line env variables.
- export your `.env` file
- run `go run .` from the root folder

### DB Seed and migrations

- execute the program with flags -migrations to run migrations, -server to start the server
- in development environment, -server additionally runs the SeedZorgOrganization usecase script from usecases/seed_usecase.
- in the cloud staging environment, a dedicated Cloud Run jobs exists that runs the migrations on every new deployment

## API

The rooting of the application is defined inside `api/routes.go`

See [our API docs](https://docs.checkmarble.com/reference/introduction-1) for public facing reference or the Open API Specification for internal endpoints on Postman.
