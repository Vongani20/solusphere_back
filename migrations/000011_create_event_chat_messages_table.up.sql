CREATE TABLE IF NOT EXISTS event_chat_messages (
    id INT AUTO_INCREMENT PRIMARY KEY,
    event_id INT NOT NULL,
    sender_id INT NOT NULL,
    sender_role ENUM('user', 'admin') NOT NULL DEFAULT 'user',
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE,
    FOREIGN KEY (sender_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_event_chat_messages_event_created (event_id, created_at),
    INDEX idx_event_chat_messages_sender_id (sender_id)
);