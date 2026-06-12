ALTER TABLE users
    DROP INDEX idx_users_phone_number,
    DROP COLUMN auth_provider,
    DROP COLUMN phone_number;
