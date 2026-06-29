CREATE TABLE IF NOT EXISTS user_presence (
    user_id INT PRIMARY KEY,
    last_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_user_presence_last_seen (last_seen_at)
);
