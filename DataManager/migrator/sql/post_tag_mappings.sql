#posts
#tags

CREATE TABLE IF NOT EXISTS post_tag_mappings(
    tag_id INT NOT NULL REFERENCES tags(id),
    post_id INT NOT NULL REFERENCES posts(id),
    CONSTRAINT ptm_unique UNIQUE (tag_id, post_id)
);
