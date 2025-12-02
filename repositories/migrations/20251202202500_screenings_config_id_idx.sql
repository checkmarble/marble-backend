-- +goose Up
-- +goose NO TRANSACTION
create index concurrently screenings_config_id_paginated on screenings (screening_config_id, created_at, id);

-- +goose Down
drop index screenings_config_id_paginated;