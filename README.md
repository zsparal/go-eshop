# Go eshop

This project is a small example of implementing an event-sourced e-commerce web application using Go and Postgres. The different parts of the application are meant to be reused for later projects as building blocks.

## Local development 

### Sourcing env variables

Database credentials for local development are defined in the `.env.dev` file. To source these into your environment, use:

```bash
set -o allexport && source .env.dev && set +o allexport
```

### Starting up containers

For local development, you can use the included `docker-compose.yaml` file to start a Postgres server. The same server will be used to run integration tests, but with randomly generated database names to make it possible to run integration tests concurrently.

First, you'll want to start up the containers:

```sh
podman compose up -d
```

### Running migrations

To test the system in a live environment, you'll need to run migrations on the previously started database. The project uses [tern](https://github.com/jackc/tern) to manage migrations. To migrate to the latest version, you can run the following commands:

```bash
tern migrate --migrations ./db/migrations  --config ./db/tern.conf
```

If you ever need to revert a _single_ migration, you can do:

```bash
tern migrate -d -1 --migrations ./db/migrations  --config ./db/tern.conf
```

### Generating DB queries

The project uses [sqlc](https://sqlc.dev/) to generate type-safe Go code from SQL queries. The query definitions live under `db/queries`. You can update the generated files by running:

```bash
sqlc generate -f ./db/sqlc.yaml
```

### Running tests

The integration tests defined in the project require a running instance of Postgres. Once it is up and running, execute integration tests with:

```bash
go test -v ./tests/...
```
