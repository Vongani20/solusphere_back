CREATE TABLE help_desk_attachments (
    id INT AUTO_INCREMENT PRIMARY KEY,
    ticket_id INT NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes INT NOT NULL,
    storage_url TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (ticket_id) REFERENCES help_desk_tickets(id) ON DELETE CASCADE,
    INDEX idx_help_desk_attachments_ticket (ticket_id)
);
