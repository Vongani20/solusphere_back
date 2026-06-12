ALTER TABLE user_faces
    DROP INDEX idx_user_faces_face_id,
    DROP COLUMN face_id;
