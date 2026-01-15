# Migration Compaction Guide

This document describes how to generate a compacted baseline migration that consolidates all migrations prior to a certain point.

## Prerequisites

- PostgreSQL 17 client tools (for `pg_dump`)
- Docker (for running a fresh Postgres instance)
- Access to the migration files

## Step-by-Step Process

### 1. Start a Fresh PostgreSQL Database

Use docker-compose or start a standalone container:

```bash
docker run --name marble-compaction -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=marble -p 5433:5432 -d postgres:17
```

### 2. Temporarily Disable River Migrations

In `repositories/migrations.go`, comment out the River migration section to avoid creating River-specific tables in your dump:

```go
// Comment out this block in the Run() function:
/*
if migrateDownTo == nil {
    pgxPool, err := m.openDbPgx(ctx, cfg)
    if err != nil {
        return errors.Wrap(err, "unable to open db in Migrater")
    }
    migrator, err := rivermigrate.New(riverpgxv5.New(pgxPool), nil)
    if err != nil {
        return errors.Wrap(err, "unable to create migrator")
    }

    _, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
    if err != nil {
        return errors.Wrap(err, "unable to run migrations")
    }
}
*/
```

### 3. Run Migrations Up to the Compaction Point

Run the migrator against your fresh database. Make sure to run all migrations up to (and including) the last migration you want to compact.

```bash
# Set environment variables for your fresh database
export PG_HOSTNAME=localhost
export PG_PORT=5433
export PG_USER=postgres
export PG_PASSWORD=postgres
export PG_DATABASE=marble

# Run the migrate command
go run . --migrate
```

### 4. Export the Schema

Use `pg_dump` to export only the schema (no data). Include **both** the `marble` and `audit` schemas:

```bash
pg_dump --schema-only --no-owner --no-privileges -n marble -n audit \
    "postgresql://postgres:postgres@localhost:5433/marble" \
    > compacted_schema.sql
```

⚠️ **Important**: The application uses multiple schemas (`marble` for main data, `audit` for audit trails). Make sure to include all schemas used by the migrations.

### 5. Clean Up the Dump

The raw `pg_dump` output contains boilerplate that should be removed:

#### Remove these pg_dump artifacts:

- Header comments (`-- PostgreSQL database dump`, `-- Dumped from/by version`)
- Session SET statements:
  ```sql
  SET statement_timeout = 0;
  SET lock_timeout = 0;
  SET default_tablespace = '';
  SET default_table_access_method = heap;
  -- etc.
  ```

#### Replace explicit sequences with SERIAL:

pg_dump expands `SERIAL` columns into explicit sequences. Convert them back:

**Before (pg_dump output):**

```sql
CREATE TABLE marble.my_table (
    id integer NOT NULL,
    ...
);
CREATE SEQUENCE marble.my_table_id_seq AS integer ...;
ALTER SEQUENCE marble.my_table_id_seq OWNED BY marble.my_table.id;
ALTER TABLE ONLY marble.my_table ALTER COLUMN id SET DEFAULT nextval('marble.my_table_id_seq'::regclass);
```

**After (cleaned up):**

```sql
CREATE TABLE marble.my_table (
    id SERIAL NOT NULL,
    ...
);
```

### 6. Add Database/Role Configuration (pg_dump doesn't capture these!)

⚠️ **Important**: `pg_dump --schema-only` only exports schema objects. It does **not** capture:

- Database-level settings (`ALTER DATABASE ... SET search_path`)
- Role-level settings (`ALTER ROLE ... SET search_path`)
- Grants (`GRANT ALL PRIVILEGES ...`)
- Extensions (`CREATE EXTENSION`)
- Session settings (`SET SEARCH_PATH`)

You must **manually add** these at the beginning of your compacted migration, before the schema objects:

```sql
-- Create schemas
CREATE SCHEMA marble;
CREATE SCHEMA audit;

-- Grant privileges and set search path for marble schema
DO $$
BEGIN
   EXECUTE 'GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA marble TO ' || current_user;
END
$$;

DO $$
BEGIN
   EXECUTE 'GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA audit TO ' || current_user;
END
$$;

DO $$
BEGIN
   EXECUTE 'ALTER DATABASE ' || current_database() || ' SET search_path TO marble, public';
END
$$;

DO $$
BEGIN
   EXECUTE format('ALTER ROLE %I SET search_path = marble, public;', current_user);
END
$$;

SET SEARCH_PATH = marble, public;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Rest of schema from pg_dump...
```

Check the original migrations for any other extensions or database-level settings that may have been added.

### 7. Add Goose Markers

Wrap the schema in goose migration markers:

```sql
-- +goose Up
-- +goose StatementBegin

-- Your schema DDL here...

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP SCHEMA IF EXISTS marble CASCADE;
DROP SCHEMA IF EXISTS audit CASCADE;

-- +goose StatementEnd
```

### 8. Name the Migration File

Name the file with a timestamp that comes **before** all the migrations you're replacing but makes logical sense:

```
20241231000000_baseline_compacted.sql
```

### 9. Archive Old Migrations

Delete the old migration files (or archive them somewhere).

### 10. Update Minimum Version Check

In `repositories/migrations.go`, update the `MinimumMigrationVersion` constant to a version sufficiently **after** the compacted migrations. This ensures that databases on older versions must upgrade through an intermediate release first:

```go
const (
    // Must be greater than the highest version in the compacted baseline
    MinimumMigrationVersion int64 = 20250218103800
)
```

### 11. Test

1. **Fresh install**: Drop and recreate the database, run migrations — should apply baseline + subsequent migrations
2. **Valid upgrade**: Database at version ≥ MinimumMigrationVersion — should skip baseline, apply only new migrations
3. **Invalid upgrade**: Database below MinimumMigrationVersion — should fail with clear error message

### 12. Restore River Migrations

Uncomment the River migration block in `repositories/migrations.go`.

### 13. Update CI job

Update the CI job that verifies no misordered migrations, by removing the baseline migration (the version must be kept up to date) in the local copy of the repository before running the migrations.

## Cleanup

```bash
docker stop marble-compaction && docker rm marble-compaction
```

## Notes

- The compacted migration is only applied to **fresh databases**
- Existing databases skip it because their goose version is already higher
- The `MinimumMigrationVersion` check prevents running on databases that are too old
- River migrations are handled separately and should not be included in the compacted schema
- The application uses multiple schemas: `marble` (main data) and `audit` (audit trails) — both must be included in the compacted baseline
