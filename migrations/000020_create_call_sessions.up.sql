CREATE TABLE IF NOT EXISTS call_sessions (
    id CHAR(36) PRIMARY KEY,
    caller_id INT NOT NULL,
    callee_id INT NOT NULL,
    call_type VARCHAR(10) NOT NULL DEFAULT 'audio',
    status VARCHAR(20) NOT NULL DEFAULT 'ringing',
    offer_sdp MEDIUMTEXT NULL,
    answer_sdp MEDIUMTEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    ended_at TIMESTAMP NULL,
    FOREIGN KEY (caller_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (callee_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_call_sessions_callee_status (callee_id, status),
    INDEX idx_call_sessions_caller_status (caller_id, status)
);

CREATE TABLE IF NOT EXISTS call_ice_candidates (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    call_id CHAR(36) NOT NULL,
    sender_id INT NOT NULL,
    candidate TEXT NOT NULL,
    sdp_mid VARCHAR(64) NULL,
    sdp_m_line_index INT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (call_id) REFERENCES call_sessions(id) ON DELETE CASCADE,
    INDEX idx_call_ice_call_id (call_id, id)
);
