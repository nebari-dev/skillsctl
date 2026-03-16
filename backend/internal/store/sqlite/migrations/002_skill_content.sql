-- +goose Up
ALTER TABLE skill_versions ADD COLUMN content BLOB;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35.0.
-- For modernc.org/sqlite (which tracks recent SQLite versions), DROP COLUMN works.
ALTER TABLE skill_versions DROP COLUMN content;
