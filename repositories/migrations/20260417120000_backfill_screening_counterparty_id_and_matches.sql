-- +goose Up
-- Backfill counterparty_id and number_of_matches on screenings from screening_matches
UPDATE screenings s
SET
  counterparty_id = sub.counterparty_id,
  number_of_matches = sub.cnt
FROM
  (
    SELECT
      screening_id,
      max(counterparty_id) AS counterparty_id,
      count(*) AS cnt
    FROM
      screening_matches
    GROUP BY
      screening_id
  ) sub
WHERE
  s.id = sub.screening_id
  AND (
    s.counterparty_id IS NULL
    OR s.number_of_matches IS NULL
    OR s.number_of_matches = 0
  );

-- Drop counterparty_id from screening_matches (no longer needed)
ALTER TABLE screening_matches
DROP COLUMN counterparty_id;

ALTER TABLE screenings
ALTER COLUMN number_of_matches
SET DEFAULT 0;

-- +goose Down
ALTER TABLE screening_matches
ADD COLUMN counterparty_id text;

-- Copy counterparty_id back from screenings to screening_matches
UPDATE screening_matches sm
SET counterparty_id = s.counterparty_id
FROM screenings s
WHERE sm.screening_id = s.id
  AND s.counterparty_id IS NOT NULL;

ALTER TABLE screenings
ALTER COLUMN number_of_matches
DROP DEFAULT;