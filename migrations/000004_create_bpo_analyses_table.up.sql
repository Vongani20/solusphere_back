CREATE TABLE IF NOT EXISTS bpo_analyses (
    id CHAR(36) PRIMARY KEY,
    filename VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    extracted_text TEXT,
    analysis_result JSON,
    status VARCHAR(50) DEFAULT 'pending',
    page_count INT DEFAULT 0,
    analysis_type VARCHAR(100) DEFAULT 'general',
    confidence_score DECIMAL(5,4) DEFAULT 0.0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_bpo_analysis_status (status),
    INDEX idx_bpo_analysis_type (analysis_type),
    INDEX idx_bpo_analysis_created_at (created_at)
);