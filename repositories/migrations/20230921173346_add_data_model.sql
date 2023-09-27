-- +goose Up
-- +goose StatementBegin
CREATE TABLE data_model_tables (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID REFERENCES organizations ON DELETE CASCADE NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    UNIQUE (organization_id, name)
);

CREATE TYPE data_model_types AS ENUM (
    'Bool',
    'Int',
    'Float',
    'String',
    'Timestamp'
);

CREATE TABLE data_model_fields (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    table_id        UUID REFERENCES data_model_tables ON DELETE CASCADE NOT NULL,
    name            TEXT NOT NULL,
    type            data_model_types NOT NULL,
    nullable        BOOLEAN NOT NULL,
    description     TEXT,
    UNIQUE (table_id, name)
);

CREATE TABLE data_model_links (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID REFERENCES organizations NOT NULL,
    name            TEXT NOT NULL,
    parent_table_id UUID REFERENCES data_model_tables ON DELETE CASCADE NOT NULL,
    parent_field_id UUID REFERENCES data_model_fields ON DELETE CASCADE NOT NULL,
    child_table_id  UUID REFERENCES data_model_tables ON DELETE CASCADE NOT NULL,
    child_field_id  UUID REFERENCES data_model_fields ON DELETE CASCADE NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE data_model_links;
DROP TABLE data_model_fields;
DROP TABLE data_model_tables;
DROP TYPE data_model_types;

-- +goose StatementEnd
