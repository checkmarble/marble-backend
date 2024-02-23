-- +goose Up
-- +goose StatementBegin
DROP TABLE data_models;

DROP TYPE data_models_status;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
CREATE TYPE data_models_status AS ENUM('validated', 'live', 'deprecated');

CREATE TABLE
      data_models (
            id uuid DEFAULT uuid_generate_v4 () PRIMARY KEY,
            org_id uuid REFERENCES organizations ON DELETE CASCADE NOT NULL,
            version VARCHAR NOT NULL,
            status data_models_status NOT NULL,
            tables json NOT NULL,
            deleted_at TIMESTAMP WITH TIME ZONE
      );

-- +goose StatementEnd