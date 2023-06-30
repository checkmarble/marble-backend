-- +goose Up
-- +goose StatementBegin


-- lists
CREATE TABLE custom_lists(
  id uuid DEFAULT uuid_generate_v4(),
  organization_id uuid NOT NULL,
  name VARCHAR NOT NULL,
  description VARCHAR NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id),
  CONSTRAINT fk_custom_lists_org FOREIGN KEY(organization_id) REFERENCES organizations(id) ON DELETE CASCADE
);

CREATE INDEX idx_organization_id ON custom_lists(organization_id);

-- list_value
CREATE TABLE custom_list_values(
  id uuid DEFAULT uuid_generate_v4(),
  custom_list_id uuid NOT NULL,
  value VARCHAR NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  deleted_at TIMESTAMP WITH TIME ZONE,
  PRIMARY KEY(id),
  CONSTRAINT fk_custom_lists_value_lists FOREIGN KEY(custom_list_id) REFERENCES custom_lists(id) ON DELETE CASCADE
);

CREATE INDEX idx_custom_list_id ON custom_list_values(custom_list_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE custom_lists CASCADE;
DROP TABLE custom_list_value CASCADE;

-- +goose StatementEnd
