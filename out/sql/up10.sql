CREATE TABLE comics(
    id INTEGER PRIMARY KEY,
    title VARCHAR(128) NOT NULL
);

CREATE TABLE comic_mappings(
    id INTEGER PRIMARY KEY,
    comic_id INTEGER NOT NULL,
    post_id INTEGER NOT NULL,
    post_order INTEGER NOT NULL,
    FOREIGN KEY(comic_id) REFERENCES comics(id),
    FOREIGN KEY(post_id) REFERENCES posts(id),
    CONSTRAINT comic_post_unique_constraint UNIQUE (comic_id, post_id)
);