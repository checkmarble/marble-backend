-- +goose Up
-- +goose StatementBegin
CREATE TABLE scenarios(
    id uuid DEFAULT uuid_generate_v4(),
    org_id uuid NOT NULL,
    name VARCHAR NOT NULL,
    description VARCHAR NOT NULL,
    trigger_object_type VARCHAR NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    PRIMARY KEY(id),
    CONSTRAINT fk_scenarios_org FOREIGN KEY(org_id) REFERENCES organizations(id)
);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE scenarios;

-- +goose StatementEnd