CREATE TABLE IF NOT EXISTS user_consents (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    consent_type VARCHAR(64) NOT NULL,
    consent_version VARCHAR(16) NOT NULL DEFAULT '1.0',
    signed_name VARCHAR(200) NOT NULL,
    accepted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ip_address VARCHAR(64),
    user_agent VARCHAR(500),
    UNIQUE KEY uniq_user_consent (user_id, consent_type, consent_version),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_user_consents_user_id (user_id)
);
