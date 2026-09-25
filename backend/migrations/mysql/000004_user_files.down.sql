DROP TABLE IF EXISTS file_references;
ALTER TABLE student_verifications DROP COLUMN ocr_completed_at, DROP COLUMN ocr_raw_json, DROP COLUMN ocr_confidence, DROP COLUMN ocr_platform;
ALTER TABLE users DROP INDEX idx_users_avatar_file, DROP COLUMN avatar_file_id;
