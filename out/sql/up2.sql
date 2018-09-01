CREATE TABLE post_tag_mappings(
    tag_id INTEGER NOT NULL,
    post_id INTEGER NOT NULL,
    FOREIGN KEY(tag_id) REFERENCES tags(id),
    FOREIGN KEY(post_id) REFERENCES posts(id),
    CONSTRAINT tag_post UNIQUE (tag_id, post_id)
);