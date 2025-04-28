# Introduction

This repo is the Marble backend implementation:

- 1 single go app
- Postgres DB

## Getting Started

### Setup your environment

> **Disclaimer**
>
> This repository’s README file is intended for internal use by our development team. The documentation provided here is specifically designed for setting up and running the project on macOS.
>
> While external contributions and interest are appreciated, please note that we do not officially support setups on other operating systems. If you encounter issues outside of the macOS environment, support may be limited.
> 
> For general documentation and user-facing guides, please refer to our main repository: [Marble Documentation](https://github.com/checkmarble/marble/blob/main/README.md).

[Install mise-en-place](https://mise.jdx.dev/getting-started.html) or alternatively [install Go](https://go.dev/doc/install) manually on your laptop (see the version in go.mod).

Create your own `.env` file based on `.env.example`. You can customize it with your own values, but it should work locally with the default values (except for third-party service functionalities).

#### Setup the Database and Services

Launch the Postgres DB and other services used by the backend:

```sh
docker compose up -d
```

> NB: It creates a `marble-backend_postgres-db` volume to store PG data.

#### Firebase emulator suite for local development

Install the [Firebase tools](https://firebase.google.com/docs/emulator-suite):

```sh
# Install the Firebase CLI
curl -sL https://firebase.tools | bash

# Check the version is at least 13.16.0
firebase --version

# Login to firebase cli
firebase login
```

Then copy the `./contrib/firebase-local-data.example` folder to `./firebase-local-data`. This folder will be used to store the local data of the Firebase emulator. It is ignored by git.
```
cp -r ./contrib/firebase-local-data.example ./firebase-local-data
```

Then start it using (replace `[GOOGLE_CLOUD_PROJECT]` with the value from your `.env` file):

```sh
firebase --project [GOOGLE_CLOUD_PROJECT] emulators:start --import=./firebase-local-data --export-on-exit
```

> NB: The `--import` flag is used to import the local data into the emulator. The `--export-on-exit` flag is used to export the data when the emulator is stopped so you don't lose your changes.

### Launch the project

 or [mise](https://mise.jdx.dev/)) and run the root of the project:

The backend project is made of five discrete components:

 - The API server
 - The background worker
 - The scheduled executor
 - The data ingestion worker
 - The pending webhook handler

Depending on which feature you need while developing, you should run one or more of those services. The last three are one-off commands that are usually run in cron and do not need to run in the background. The worker, though, handles all asynchronous background tasks the API needs (such as index creation) and might be requireed for some of the API functionnality to work properly.

The `docker compose` of this repository only contains the _dependencies_ required to run the backend service, but does not start the services themselves. It is assumed the developer will run them themselves.

```sh
mise exec -- go run . --migrations --server
mise exec -- go run . --worker
```

Alternatively without mise, export your `.env` file (e.g. using [direnv](https://direnv.net/) :
```sh
go run . --migrations --server # To start the API
go run . --worker # To start the worker
```

If you need to run the one-off components (for example if you are working on background data ingestion or on scheduled scenario execution), run them directly from your editor or the terminal when required:

```sh
mise exec -- go run . --scheduled-executer
mise exec -- go run . --send-pending-webhook-events
mise exec -- go run . --data-ingestion
```

Alternatively, without mise :
```sh
go run . --scheduled-executer
go run . --send-pending-webhook-events
go run . --data-ingestion
```

> Using VSCode, you can also run the `Migrate and Launch (.env)` task in the "Run and debug" tab. This will load your env file, migrate the DB and start the server. Other components can also be started with the appropriately-named tasks.

## Application flags

The application can be run with the following flags:

- `--migrations`: run the migrations
- `--server`: run the server
- `--scheduled-executer`: execute scheduled scenario job
- `--data-ingestion`: run data ingestion job
- `--cron-scheduler`: background job that automatically runs the cron jobs
- `--send-pending-webhook-events`: retry sending failed webhooks

> NB: `.vscode/launch.json` contains the configuration to run the app with these flags.

## API

The routing of the application is defined inside `api/routes.go`.

For further information on the API, you can also refer to the following resources:

- [our API docs](https://docs.checkmarble.com/reference/introduction-1) for public facing reference
- the Open API Specification defined in the frontend repository [here](https://github.com/checkmarble/marble-frontend/blob/main/packages/marble-api/scripts/openapi.yaml).

## DB Seed and migrations

The application uses [goose](https://github.com/pressly/goose) to manage migrations.

Migrations are located in the `repositories/migrations` folder.

Execute the program with flags `-migrations` to run migrations.

To create a new migration, you can use the following command from within the `repositories/migrations` folder

```sh
goose create add_some_column sql
```

It happens that the migrations end up being misordered. This happens if two people pushed new migrations A and B, B having a timestamp greated than A, but B B is commited first to main. The issue can occur when pushing to the main branch, or when pulling changes from remote main to the local branch. In this case, you may need to roll back a few local migrations before you can migrate up again.

The easiest way of doings this is by installing the goose cli with brew (`brew install goose`), configuring the goose environment variables (typically `export GOOSE_DRIVER=postgres` and `export GOOSE_DBSTRING="user=postgres dbname=marble host=localhost password=marble"` should work), and then running `goose down` as many times as needed from the `repositories/migrations` folder. See also [the goose doc](https://github.com/pressly/goose).

##### (VSCode) Install recommended VSCode extensions

There is a recommended extensions list in the `.vscode/extensions.json` file.

All required configuration settings are already included inside the `.vscode/settings.json` file.
Recommended settings are in the `.vscode/.user-settings.sample.json` file. Cherry-pick them to your user config file.

## FAQ

### How to update firebase local data ?

- Run firebase emulator with paramater: `--export-on-exit`
- Add user, change options...
- Exit the emulator

> NB: The data will be saved in the `./firebase-local-data` folder. If you want to share the data, you can copy it to `./contrib/firebase-local-data.example` and commit it.

### How to reset the DB ?

`docker volume rm marble-backend_postgres-db` deletes the PG volume, useful to reset the app to a known state

In practice, this single-line will delete the stack and create a new one:
`docker compose down && docker volume rm marble-backend_postgres-db && docker compose up -d`
