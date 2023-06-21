-- +goose Up
-- +goose StatementBegin


-- lists
CREATE TABLE lists(
  id uuid DEFAULT uuid_generate_v4(),
  org_id uuid NOT NULL,
  name VARCHAR NOT NULL,
  description VARCHAR NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE,
  updated_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id),
  CONSTRAINT fk_lists_org FOREIGN KEY(org_id) REFERENCES organizations(id) ON DELETE CASCADE
);

-- list_value
CREATE TABLE list_value(
  id uuid DEFAULT uuid_generate_v4(),
  list_id uuid NOT NULL,
  value VARCHAR NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE,
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id),
  CONSTRAINT fk_lists_value_lists FOREIGN KEY(list_id) REFERENCES lists(id) ON DELETE CASCADE
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE lists CASCADE;
DROP TABLE list_value CASCADE;

-- +goose StatementEnd
