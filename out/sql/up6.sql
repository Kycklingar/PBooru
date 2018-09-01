CREATE TABLE mime_type(
    id INTEGER PRIMARY KEY,
    mime VARCHAR(64) NOT NULL,
    type VARCHAR(64) NOT NULL,
    CONSTRAINT mime_type_unique UNIQUE (mime, type)
);

INSERT INTO mime_type(mime, type) VALUES("unknown", "unknown");


ALTER TABLE posts RENAME TO tmp_posts;

CREATE TABLE posts(
    id INTEGER PRIMARY KEY,
    multihash VARCHAR(49) UNIQUE NOT NULL,
    mime_id INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY(mime_id) REFERENCES mime_type(id)
);

INSERT INTO posts(id, multihash) SELECT id, multihash FROM tmp_posts;

DROP TABLE tmp_posts;