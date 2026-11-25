ALTER TABLE week_messages ADD COLUMN pin_state SMALLINT NOT NULL DEFAULT 0 CHECK (pin_state IN (0, 1, 2));

UPDATE week_messages
SET pin_state = 1
FROM weeks
WHERE week_messages.week_id = weeks.id AND weeks.is_pinned;

ALTER TABLE weeks DROP COLUMN is_pinned;
