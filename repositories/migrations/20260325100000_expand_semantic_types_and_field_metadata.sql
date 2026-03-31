-- +goose Up
ALTER TABLE data_model_fields ADD COLUMN metadata jsonb DEFAULT NULL;
ALTER TABLE data_model_fields ADD COLUMN alias text NOT NULL DEFAULT '';

ALTER TABLE data_model_tables ADD COLUMN metadata jsonb DEFAULT NULL;

ALTER TABLE data_model_links ADD COLUMN link_type text NOT NULL DEFAULT 'related';
ALTER TABLE data_model_links ADD CONSTRAINT data_model_links_link_type_check CHECK (link_type IN ('related', 'belongs_to'));

ALTER TABLE data_model_tables ADD COLUMN primary_ordering_field text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE data_model_tables DROP COLUMN primary_ordering_field;

ALTER TABLE data_model_links DROP CONSTRAINT data_model_links_link_type_check;
ALTER TABLE data_model_links DROP COLUMN link_type;

ALTER TABLE data_model_tables DROP COLUMN metadata;

ALTER TABLE data_model_fields DROP COLUMN metadata;
ALTER TABLE data_model_fields DROP COLUMN alias;
