CREATE TABLE lessons (
    id TEXT PRIMARY KEY,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    subject TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT '',
    room TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    teacher TEXT NOT NULL DEFAULT ''
);
