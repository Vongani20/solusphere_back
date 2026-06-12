ALTER TABLE user_faces
    ADD COLUMN face_id VARCHAR(255) NULL AFTER image_url,
    ADD INDEX idx_user_faces_face_id (face_id);
