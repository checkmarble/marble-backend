-- +goose Up
-- +goose StatementBegin

ALTER TABLE cases ADD COLUMN type text NOT NULL DEFAULT 'decision';
ALTER TABLE cases ADD CONSTRAINT valid_case_type CHECK (type IN ('decision', 'continuous_screening'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE cases DROP CONSTRAINT valid_case_type;
ALTER TABLE cases DROP COLUMN type;

-- +goose StatementEnd
