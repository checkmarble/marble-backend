-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE rec record;
BEGIN
	FOR rec IN 
	    SELECT table_schema, table_name, concat(table_name,'_pkey') AS constraint_name
	    FROM information_schema.tables 
	    WHERE table_schema NOT IN ('marble','analytics','public', 'pg_catalog', 'information_schema')
	LOOP 
		EXECUTE format('ALTER TABLE %I.%I drop constraint if exists %I;', rec.table_schema, rec.table_name, rec.constraint_name);
		RAISE NOTICE 'done for %', rec.table_name;
	END LOOP;
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE rec record;
BEGIN
	FOR rec IN 
	    SELECT table_schema, table_name
	    FROM information_schema.tables 
	    WHERE table_schema NOT IN ('marble','analytics','public', 'pg_catalog', 'information_schema')
	LOOP 
		EXECUTE format('ALTER TABLE %I.%I ADD PRIMARY KEY (ID);', rec.table_schema, rec.table_name);
		RAISE NOTICE 'done for %', rec.table_name;
	END LOOP;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd