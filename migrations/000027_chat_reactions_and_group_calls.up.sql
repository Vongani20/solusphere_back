CREATE TABLE IF NOT EXISTS direct_message_reactions (
    message_id INT NOT NULL,
    user_id INT NOT NULL,
    emoji VARCHAR(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (message_id, user_id),
    FOREIGN KEY (message_id) REFERENCES direct_messages(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) DEFAULT CHARSET = utf8mb4;

CREATE TABLE IF NOT EXISTS group_calls (
    id CHAR(36) PRIMARY KEY,
    host_id INT NOT NULL,
    call_type VARCHAR(10) NOT NULL DEFAULT 'audio',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP NULL,
    FOREIGN KEY (host_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_group_calls_status (status, created_at)
);

CREATE TABLE IF NOT EXISTS group_call_participants (
    call_id CHAR(36) NOT NULL,
    user_id INT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'invited',
    invited_by INT NULL,
    invited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    joined_at TIMESTAMP NULL,
    left_at TIMESTAMP NULL,
    last_seen_at TIMESTAMP NULL,
    PRIMARY KEY (call_id, user_id),
    FOREIGN KEY (call_id) REFERENCES group_calls(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_group_call_participants_user (user_id, status)
);

CREATE TABLE IF NOT EXISTS group_call_signals (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    call_id CHAR(36) NOT NULL,
    from_user_id INT NOT NULL,
    to_user_id INT NOT NULL,
    kind VARCHAR(20) NOT NULL,
    payload MEDIUMTEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (call_id) REFERENCES group_calls(id) ON DELETE CASCADE,
    INDEX idx_group_call_signals_to (call_id, to_user_id, id)
);
