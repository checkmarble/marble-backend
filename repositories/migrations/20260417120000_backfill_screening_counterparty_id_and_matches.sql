-- +goose Up

-- Backfill counterparty_id on screenings from screening_matches (all matches for a given screening have the same counterparty_id)
UPDATE screenings s
SET counterparty_id = sub.counterparty_id
FROM (
    SELECT DISTINCT ON (screening_id) screening_id, counterparty_id
    FROM screening_matches
    WHERE counterparty_id IS NOT NULL
) sub
WHERE s.id = sub.screening_id
  AND s.counterparty_id IS NULL;

-- Backfill number_of_matches on screenings from screening_matches count
UPDATE screenings s
SET number_of_matches = sub.cnt
FROM (
    SELECT screening_id, count(*) AS cnt
    FROM screening_matches
    GROUP BY screening_id
) sub
WHERE s.id = sub.screening_id
  AND (s.number_of_matches IS NULL OR s.number_of_matches = 0);

-- Drop counterparty_id from screening_matches (no longer needed)
ALTER TABLE screening_matches DROP COLUMN counterparty_id;

-- +goose Down
ALTER TABLE screening_matches ADD COLUMN counterparty_id text;
