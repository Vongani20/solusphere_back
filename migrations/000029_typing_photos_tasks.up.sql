CREATE TABLE IF NOT EXISTS chat_typing (
    user_id INT NOT NULL,
    peer_id INT NOT NULL,
    typed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, peer_id),
    INDEX idx_chat_typing_peer (peer_id, user_id)
);

ALTER TABLE users ADD COLUMN profile_photo_url TEXT NULL;

CREATE TABLE IF NOT EXISTS tasks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    description TEXT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'todo',
    priority VARCHAR(10) NOT NULL DEFAULT 'medium',
    due_date DATE NULL,
    created_by INT NOT NULL,
    assigned_to INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (assigned_to) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_tasks_assigned_status (assigned_to, status),
    INDEX idx_tasks_created_by (created_by, status)
) DEFAULT CHARSET = utf8mb4;
