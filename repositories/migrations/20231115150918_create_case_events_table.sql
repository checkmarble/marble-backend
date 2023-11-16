-- +goose Up
-- +goose StatementBegin
CREATE TABLE case_events (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  case_id UUID REFERENCES cases(id) ON DELETE CASCADE NOT NULL,
  user_id UUID NOT NULL,
  event_type VARCHAR NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  additional_note text,
  resource_id UUID,
  resource_type text,
  new_value VARCHAR,
  previous_value VARCHAR
);

CREATE INDEX case_event_case_id_idx ON case_events(case_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX case_event_case_id_idx;
DROP TABLE case_events;
-- +goose StatementEnd
