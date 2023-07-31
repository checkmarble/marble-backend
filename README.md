# Introduction

This repo is the Marble backend implementation:

- 1 single Go app
- Postgres DB
- PGAdmin (to view the DB content)

## Getting Started

### Requirements

[Install Go](https://go.dev/doc/install) on your laptop. For now, there is no fixed version in the project, but according to `go.mod` we all use a `1.20` version.

> NB: To handle different version, you can look at [Managing Go installations](https://go.dev/doc/manage-install) or use a version manager tool like [asdf](https://github.com/kennyp/asdf-golang)

You may also need to [install the gcloud CLI](https://cloud.google.com/sdk/docs/install) in order to interact with deployed environments.

> NB: the GCP project is `tokyo-country-381508` (you may need to ask for permissions)

### Deployment


#### Prerequisites

Install firebase-tools (`npm install -g firebase-tools`)

```
firebase login
firebase init
```

### Firebase emulator suite for local development

Installation: https://firebase.google.com/docs/emulator-suite

Then start the emulator suite:

```
firebase --project staging emulators:start --import=./firebase-local-data
```

Connect in the backoffice using: `admin@checkmarble.com`
Connect in the frontend using: `jbe@zorg.com`

#### How to add data to ./firebase-local-data

- Run firebase emulator with paramater: `--export-on-exit`
  `firebase --project staging emulators:start --import=./firebase-local-data --export-on-exit`
- Add user, change options...
- Exit the emulator
- commit


#### Export local data


### Lauch the project

#### Docker

`docker compose up -d --build` : build the app container, and launches the stack (in deamon mode)
Creates a `marble-backend_postgres-db` volume to store PG data.

`docker compose logs -f -t marble-backend db` shows the logs for the app and PG. useful to filter out annoying PGAdmin logs

`docker volume rm marble-backend_postgres-db` deletes the PG volume, useful to reset the app to a known state

In practice, this single-line will delete the stack and create a new one:
`docker compose down && docker volume rm marble-backend_postgres-db && docker compose up -d --build && docker compose logs -f -t marble-backend db`
`ctrl-C` to detach from the logs output

#### Local (VS Code)

You can choose to launch the application locally, using the provided debug task (especially usefull to dev, as the task launch a debugger):

- Start a DB using docker (you can inspire from the existing docker file)
- Create your local `.env` using the provided `.env.tmpl`.
  - To create a `AUTHENTICATION_JWT_SIGNING_KEY` run `openssl genrsa -out signing.pem 2048` and save the value as a one liner using `\n` for line breaks
- Lauch the debug task (VS Code)

### DB Seed and reset

- execute the program with flags -migrations to run migrations, -wipe to reset the DB to an empty state, -server to start the server
- in dev or staging environments, -server additionally runs the seed script from pg_repository/seed.go.
- in the cloud staging environment, two Cloud Run jobs exist to take the migrations/wipe actions

## API

The rooting of the application is defined inside `api/routes.go`

See [our API docs](https://docs.checkmarble.com/reference/introduction-1) for public facing reference or the Open API Specification for internal endpoints on Postman.

## curl calls

`POST` a decision. Get TokenID and ScenarioID from startup log (cf `seed.go`).
Token value is hardcoded to `token12345` for convenience.

```sh
// Initialise variables in your shell
SCENARIO_ID=...
REFRESH_TOKEN="token12345"
```

Get an access token by calling

```sh
TOKEN=$(curl -XPOST -H "Content-type: application/json" -H "X-API-Key: token12345" http://localhost:8080/token)
```

Beware that the implementation of getting the different types of access tokens is not finished yet, and you may encounter authorization errors on the various endpoints.

```sh
curl -XPOST -H "Content-type: application/json" -H "Authorization: Bearer $TOKEN" -d "$(jq -n --arg scenario_id "$SCENARIO_ID" '{"scenario_id": $scenario_id, "trigger_object":{"type": "tx", "amount": 5.0} }')" 'http://localhost:8080/decisions'
```

display result, store created id in .last_id file

```sh
curl -XPOST -H "Content-type: application/json" -H "Authorization: Bearer $TOKEN" -d "$(jq -n --arg scenario_id "$SCENARIO_ID" '{"scenario_id": $scenario_id, "trigger_object":{"type": "tx", "amount": 5.0} }')" 'http://localhost:8080/decisions' | tee >(jq) | jq -r '.id' > .last_id
```

`GET` a decision. Replace the ID by one you created.

```sh
curl -XGET -H "Content-type: application/json" -H "Authorization: Bearer $TOKEN" 'http://localhost:8080/decisions/9a2b5c9d-ac12-45b3-8f52-0eda979d5853'
```

use .last_id file to find id just created

```sh
curl -XGET -H "Content-type: application/json" -H "Authorization: Bearer $TOKEN" "http://localhost:8080/decisions/$(cat .last_id)" | jq
```
