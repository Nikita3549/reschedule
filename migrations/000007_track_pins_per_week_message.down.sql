ALTER TABLE weeks ADD COLUMN is_pinned BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE weeks
SET is_pinned = TRUE
WHERE EXISTS (
    SELECT 1 FROM week_messages
    WHERE week_id = weeks.id AND pin_state <> 0
);

ALTER TABLE week_messages DROP COLUMN pin_state;
