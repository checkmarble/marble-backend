-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE rec record;
BEGIN
	FOR rec IN
	    SELECT table_schema, table_name, concat(table_name,'_pkey') AS constraint_name
	    FROM information_schema.tables
	    WHERE table_schema NOT IN ('marble','analytics','public', 'pg_catalog', 'information_schema', 'google_ml')
	    AND has_table_privilege(current_user, quote_ident(table_schema) || '.' || quote_ident(table_name), 'UPDATE')
	LOOP
		EXECUTE format('ALTER TABLE %I.%I drop constraint if exists %I;', rec.table_schema, rec.table_name, rec.constraint_name);
		RAISE NOTICE 'done for %', rec.table_name;
	END LOOP;
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
	rec record;
	has_pk boolean;
BEGIN
	FOR rec IN
	    SELECT table_schema, table_name
	    FROM information_schema.tables
	    WHERE table_schema NOT IN ('marble','analytics','public', 'pg_catalog', 'information_schema', 'google_ml')
	    AND has_table_privilege(current_user, quote_ident(table_schema) || '.' || quote_ident(table_name), 'UPDATE')
	LOOP
		-- Check if table already has a primary key
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.table_constraints
			WHERE table_schema = rec.table_schema
			AND table_name = rec.table_name
			AND constraint_type = 'PRIMARY KEY'
		) INTO has_pk;

		-- Only add primary key if table doesn't have one
		IF NOT has_pk THEN
			EXECUTE format('ALTER TABLE %I.%I ADD PRIMARY KEY (ID);', rec.table_schema, rec.table_name);
			RAISE NOTICE 'added primary key for %', rec.table_name;
		ELSE
			RAISE NOTICE 'skipped % - already has primary key', rec.table_name;
		END IF;
	END LOOP;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd