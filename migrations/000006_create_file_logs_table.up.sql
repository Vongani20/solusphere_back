-- Add table to track secret versions (optional)
CREATE TABLE IF NOT EXISTS secret_versions (
    id SERIAL PRIMARY KEY,
    secret_name VARCHAR(255) NOT NULL,
    version VARCHAR(255) NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);