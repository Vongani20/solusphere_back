ALTER TABLE direct_messages
    MODIFY message TEXT NULL,
    ADD COLUMN message_type VARCHAR(20) NOT NULL DEFAULT 'text' AFTER message,
    ADD COLUMN attachment_url TEXT NULL AFTER message_type,
    ADD COLUMN attachment_mime VARCHAR(100) NULL AFTER attachment_url;
