CREATE TABLE IF NOT EXISTS direct_messages (
    id INT AUTO_INCREMENT PRIMARY KEY,
    sender_id INT NOT NULL,
    receiver_id INT NOT NULL,
    message TEXT NOT NULL,
    read_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (sender_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (receiver_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_direct_messages_sender_receiver_created (sender_id, receiver_id, created_at),
    INDEX idx_direct_messages_receiver_sender_created (receiver_id, sender_id, created_at),
    INDEX idx_direct_messages_receiver_read (receiver_id, read_at)
);
