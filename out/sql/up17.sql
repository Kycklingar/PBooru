CREATE TABLE duplicate_posts(
    id INTEGER PRIMARY KEY,
    dup_id INTEGER NOT NULL,
    post_id INTEGER NOT NULL UNIQUE REFERENCES posts(id),
    level INTEGER NOT NULL
);