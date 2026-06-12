ALTER TABLE users
    ADD COLUMN phone_number VARCHAR(32) NULL AFTER email,
    ADD COLUMN auth_provider VARCHAR(32) NOT NULL DEFAULT 'local' AFTER password,
    ADD INDEX idx_users_phone_number (phone_number);
