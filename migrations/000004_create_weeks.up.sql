CREATE TABLE weeks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    start_at TIMESTAMPTZ NOT NULL UNIQUE
);
