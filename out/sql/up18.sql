ALTER TABLE posts ADD COLUMN alt_group INT REFERENCES posts(id);

UPDATE posts SET alt_group = id;

CREATE INDEX IF NOT EXISTS post_alt_index ON posts (alt_group);
