CREATE TABLE IF NOT EXISTS login_audit_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NULL,
    email VARCHAR(255),
    username VARCHAR(100),
    login_method VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    failure_reason VARCHAR(255),
    ip_address VARCHAR(64),
    user_agent VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    INDEX idx_login_audit_user_id (user_id),
    INDEX idx_login_audit_email (email),
    INDEX idx_login_audit_status (status),
    INDEX idx_login_audit_method (login_method),
    INDEX idx_login_audit_created_at (created_at)
);
