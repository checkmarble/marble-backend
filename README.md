This repo is a proof of concept re-implementation of our python backend in Go

### Goal: implement an MVP to de-risk our absence of backend

Features:

- Simple monolith, follows hexagonal architecture (API / App / repo layers)
- Dynamic data model per organization
  - Tables & fields
  - 1:1 relationships
- AST (abstract syntax tree) to represent rules
  - Simplistic operators with limited safety checks
- Scenarios = a set of rules and thresholds
- Simplistic authentication
- Connection to a Postgres DB
  - Migrations included at start of app by default
  - see `migrations` folder for DB structure
- Preloading of organizations, datamodels, scenarios, tokens in memory on app startup
  - hardcoded datamodels & scenarios

### API

- Create a decision: `POST /decisions`
- View a decision `GET /decisions/id`

See [our API docs](https://docs.checkmarble.com/reference/introduction-1) for reference

### Stack

- 1 single Go app
- Postgres DB
- PGAdmin (to view the DB content)

### Usage

Requires: `docker` to run & `go` to develop

`docker compose up -d --build` : build the app container, and launches the stack (in deamon mode)
Creates a `marble-backend_postgres-db` volume to store PG data.

`docker compose logs -f -t marble-backend db` shows the logs for the app and PG. useful to filter out annoying PGAdmin logs

`docker volume rm marble-backend_postgres-db` deletes the PG volume, useful to reset the app to a known state

In practice, this single-line will delete the stack and create a new one:
`docker compose down && docker volume rm marble-backend_postgres-db && docker compose up -d --build && docker compose logs -f -t marble-backend db`
`ctrl-C` to detach from the logs output

### curl calls

`POST` a decision. Get TokenID and ScenarioID from startup log (cf `seed.go`).
Token value is hardcoded to `token12345` for convenience.

```sh
// Initialise variables in your shell
SCENARIO_ID=...
REFRESH_TOKEN="token12345"
```

Get an access token by calling

```sh
TOKEN=$(curl -XPOST -H "Content-type: application/json" -d "{\"refresh_token\": \"$REFRESH_TOKEN\"}" 'http://localhost:8080/token')
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
