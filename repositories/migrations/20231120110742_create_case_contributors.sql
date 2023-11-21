-- +goose Up
-- +goose StatementBegin
CREATE TABLE case_contributors (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  case_id UUID REFERENCES cases(id) ON DELETE CASCADE NOT NULL,
  user_id UUID NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX case_contributors_case_id_user_id_idx on case_contributors(case_id, user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX case_contributors_case_id_user_id_idx;
DROP TABLE case_contributors;
-- +goose StatementEnd
