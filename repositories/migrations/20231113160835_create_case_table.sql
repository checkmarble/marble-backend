-- +goose Up
-- +goose StatementBegin
CREATE TABLE cases (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id UUID REFERENCES organizations(id) ON DELETE CASCADE NOT NULL,
  name text NOT NULL,
  status VARCHAR NOT NULL DEFAULT 'open',
  description text,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX case_org_id_idx ON cases(org_id, created_at DESC);
CREATE INDEX case_status_idx ON cases(org_id, status, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX case_org_id_idx;
DROP INDEX case_status_idx;
DROP TABLE cases;
-- +goose StatementEnd
