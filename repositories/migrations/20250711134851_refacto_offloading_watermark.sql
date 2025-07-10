-- +goose Up
-- +goose StatementBegin

-- Lock the table to prevent concurrent access during the migration
LOCK TABLE offloading_watermarks IN EXCLUSIVE MODE;

-- Create the new watermarks table with the updated schema
CREATE TABLE watermarks (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    org_id UUID,
    type TEXT NOT NULL,
    watermark_time TIMESTAMP WITH TIME ZONE NOT NULL,
    watermark_id UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    params JSONB,

    PRIMARY KEY (id),
    -- Unique constraint treating NULL values as distinct (prevents multiple NULL org_id with same type)
    CONSTRAINT uq_watermarks_org_id_type UNIQUE NULLS NOT DISTINCT (org_id, type),
    CONSTRAINT fk_watermarks_org_id
        FOREIGN KEY (org_id) REFERENCES organizations (id)
        ON DELETE CASCADE
);

-- Migrate existing data from offloading_watermarks to watermarks
INSERT INTO watermarks (org_id, type, watermark_time, watermark_id, created_at, updated_at, params)
SELECT 
    org_id,
    'decision_rules' AS type,  -- Convert table_name to the new type system
    watermark_time,
    watermark_id,
    COALESCE(created_at, NOW()) AS created_at,
    COALESCE(updated_at, NOW()) AS updated_at,
    NULL AS params  -- No params in the old system
FROM offloading_watermarks;

-- Drop the old table
DROP TABLE offloading_watermarks;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Lock the table to prevent concurrent access during the migration
LOCK TABLE watermarks IN EXCLUSIVE MODE;

-- Recreate the old offloading_watermarks table
CREATE TABLE offloading_watermarks (
    org_id UUID,
    table_name TEXT,
    watermark_time TIMESTAMP WITH TIME ZONE NOT NULL,
    watermark_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,

    PRIMARY KEY (org_id, table_name),
    CONSTRAINT fk_org_id
        FOREIGN KEY (org_id) REFERENCES organizations (id)
        ON DELETE CASCADE
);

-- Migrate data back from watermarks to offloading_watermarks
-- Only migrate offloading type watermarks
INSERT INTO offloading_watermarks (org_id, table_name, watermark_time, watermark_id, created_at, updated_at)
SELECT 
    org_id,
    'decision_rules' AS table_name,  -- Convert type back to table_name
    watermark_time,
    watermark_id,
    created_at,
    updated_at
FROM watermarks
WHERE type = 'decision_rules' AND org_id IS NOT NULL;

-- Drop the new table
DROP TABLE watermarks;

-- +goose StatementEnd
