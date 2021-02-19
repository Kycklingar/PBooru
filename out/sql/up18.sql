ALTER TABLE posts ADD COLUMN alt_group INT REFERENCES posts(id);

UPDATE posts SET alt_group = id;
