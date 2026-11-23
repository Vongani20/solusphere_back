CREATE TABLE IF NOT EXISTS rfp_documents (
    id CHAR(36) PRIMARY KEY,
    user_id INT NOT NULL,
    original_filename VARCHAR(1024) NOT NULL DEFAULT '',
    sources_json LONGTEXT NULL,
    extracted_text LONGTEXT NULL,
    analysis_source VARCHAR(20) NOT NULL DEFAULT 'builtin',
    model VARCHAR(100) NULL,
    truncated TINYINT(1) NOT NULL DEFAULT 0,
    warning TEXT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'processing',
    template_json LONGTEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_rfp_documents_user_created (user_id, created_at)
);
