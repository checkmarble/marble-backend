-- +goose Up
-- +goose StatementBegin
ALTER TABLE partners
ADD COLUMN bic VARCHAR NOT NULL DEFAULT '';

CREATE INDEX partners_bic_idx ON partners (LOWER(bic));

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE partners
DROP COLUMN bic;

DROP INDEX partners_bic_idx;

-- +goose StatementEnd