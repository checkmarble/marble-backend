-- +goose Up
-- +goose StatementBegin

-- Update existing NULL values to empty strings
UPDATE continuous_screening_configs SET description = '' WHERE description IS NULL;

-- Make description field non-nullable with default empty string
ALTER TABLE continuous_screening_configs
    ALTER COLUMN description SET DEFAULT '',
    ALTER COLUMN description SET NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Make description field nullable again (for rollback)
ALTER TABLE continuous_screening_configs
    ALTER COLUMN description DROP NOT NULL,
    ALTER COLUMN description DROP DEFAULT;

-- +goose StatementEnd
