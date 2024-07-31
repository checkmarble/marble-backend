-- +goose Up
-- +goose StatementBegin
CREATE TABLE
    snooze_groups (
        id UUID PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4 (),
        created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
        organization_id UUID NOT NULL REFERENCES organizations (id)
    );

CREATE TABLE
    rule_snoozes (
        id UUID PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4 (),
        created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
        created_by_user UUID NOT NULL REFERENCES users (id),
        snooze_group_id UUID NOT NULL REFERENCES snooze_groups (id),
        pivot_value TEXT NOT NULL,
        starts_at TIMESTAMP WITH TIME ZONE NOT NULL,
        expires_at TIMESTAMP WITH TIME ZONE NOT NULL
    );

CREATE INDEX rule_snoozes_by_pivot ON rule_snoozes (pivot_value);

ALTER TABLE scenario_iteration_rules
ADD COLUMN snooze_group_id UUID REFERENCES snooze_groups (id);

-- +goose StatementEnd
-- +goose Down
ALTER TABLE scenario_iteration_rules
DROP COLUMN snooze_group_id;

DROP INDEX rule_snoozes_by_pivot;

DROP TABLE rule_snoozes;

DROP TABLE snooze_groups;

-- +goose StatementBegin
-- +goose StatementEnd