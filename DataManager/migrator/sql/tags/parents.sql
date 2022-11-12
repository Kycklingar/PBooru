CREATE TABLE IF NOT EXISTS parent_tags(
    parent_id INT NOT NULL REFERENCES tags(id),
    child_id INT NOT NULL REFERENCES tags(id),
    CONSTRAINT parent_child_unique UNIQUE (child_id, parent_id)
);

