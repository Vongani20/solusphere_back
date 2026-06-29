ALTER TABLE direct_messages
    DROP COLUMN attachment_mime,
    DROP COLUMN attachment_url,
    DROP COLUMN message_type,
    MODIFY message TEXT NOT NULL;
