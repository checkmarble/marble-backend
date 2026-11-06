# Introduction

> **Disclaimer**
>
> This repository’s README file is intended for internal use by our development team. The documentation provided here is specifically designed for setting up and running the project on macOS.
>
> While external contributions and interest are appreciated, please note that we do not officially support setups on other operating systems. If you encounter issues outside of the macOS environment, support may be limited.
> 
> For general documentation and user-facing guides, please refer to our main repository: [Marble Documentation](https://github.com/checkmarble/marble/blob/main/README.md).

This repo is the Marble backend implementation:

- 1 single go app
- Postgres DB

## Getting Started

### Setup your Environment

You will need the usual C/C++ toolchain to be installed on your system. E.g. on Fedora:

```sh
sudo dnf install @c-development
```

You will need also need `docker` and `docker-compose` to be installed.

Install [mise](https://mise.jdx.dev/getting-started.html), and [set up the integration with your shell](https://mise.jdx.dev/shell-setup.html) so that all dev tools are exposed in your PATH and all environment variables from your `.env` are exported. Alternatively, you can run `mise exec -- the_command` to run any tool without shell integration.
See also [the IDE extensions](https://mise.jdx.dev/ide-integration.html#ide-plugins) to have direct integration in your IDE.

Run `mise install` at the root of the repo to gather all dev dependencies.

Later, you can do:
- `mise upgrade` when deps are updated
- `mise prune` when deps are removed

Create your own `.env` file by copying `.env.example`. You can customize it with your own values, but it should work locally with the default values (except for third-party service functionalities).

### Setup the Database and Services

The `docker compose` of this repository only contains the _dependencies_ required to run the backend service, but does not start the services themselves. It is assumed the developer will run them themselves (see next section).

Launch the Postgres DB and other services used by the backend with `docker compose up -d`.
This also creates the volumes to store data for PG, ElasticSearch and Firebase local emulator.

### Build and Run the Project

The backend project is made of five discrete components:

 - The API server
 - The background worker
 - The scheduled executor
 - The data ingestion worker
 - The pending webhook handler

Depending on which feature you need while developing, you should run one or more of those services. The last three are one-off commands that are usually run in cron and do not need to run in the background. The worker, though, handles all asynchronous background tasks the API needs (such as index creation) and might be requireed for some of the API functionnality to work properly.

```sh
go run . --migrations --server # To start the API
go run . --worker # To start the worker
```

If you need to run the one-off components (for example if you are working on background data ingestion or on scheduled scenario execution), run them directly from your editor or the terminal when required:

```sh
go run . --scheduled-executer
go run . --send-pending-webhook-events
go run . --data-ingestion
```

> Using VSCode, you can also run the `Migrate and Launch (.env)` task in the "Run and debug" tab. This will load your env file, migrate the DB and start the server. Other components can also be started with the appropriately-named tasks.

You can also get live filewatching via `air`:

```sh
air -- --migrations --server # Run, then automatically rebuild and restart the server whenever a source file changes
air -- --worker # Same for the worker
```

## Application Flags

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

## DB Seed and Migrations

The application uses [goose](https://github.com/pressly/goose) to manage migrations.

Migrations are located in the `repositories/migrations` folder.

Execute the program with flags `-migrations` to run migrations.

To create a new migration, you can use the following command from within the `repositories/migrations` folder

```sh
goose create add_some_column sql
```

It happens that the migrations end up being misordered. This happens if two people pushed new migrations A and B, B having a timestamp greated than A, but B B is commited first to main. The issue can occur when pushing to the main branch, or when pulling changes from remote main to the local branch. In this case, you may need to roll back a few local migrations before you can migrate up again.

The easiest way of doings this is by installing the goose cli with brew (`brew install goose`), configuring the goose environment variables
(typically `export GOOSE_DRIVER=postgres GOOSE_DBSTRING="user=postgres dbname=marble host=localhost password=marble" GOOSE_MIGRATION_DIR="./repositories/migrations"` should work),
and then running `goose down` as many times as needed from the repository root folder. See also [the goose doc](https://github.com/pressly/goose).

##### (VSCode) Install recommended VSCode extensions

There is a recommended extensions list in the `.vscode/extensions.json` file.

All required configuration settings are already included inside the `.vscode/settings.json` file.
Recommended settings are in the `.vscode/.user-settings.sample.json` file. Cherry-pick them to your user config file.

## FAQ

### How to reset the DB ?

`docker volume rm marble-backend-postgres-db` deletes the PG volume, useful to reset the app to a known state

In practice, this single-line will delete the stack and create a new one:
`docker compose down && docker volume rm marble-backend-postgres-db && docker compose up -d`
