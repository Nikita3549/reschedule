CREATE TABLE week_messages (
    week_id UUID REFERENCES weeks (id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users (id) ON DELETE CASCADE,
    message_id INT NOT NULL,
    chat_id BIGINT NOT NULL,

    PRIMARY KEY (week_id, user_id),
    UNIQUE (chat_id, message_id)
);
