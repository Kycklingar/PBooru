CREATE TABLE parent_tags(
    parent_id INTEGER REFERENCES tags(id) NOT NULL,
    child_id INTEGER REFERENCES tags(id) NOT NULL,
    CONSTRAINT parent_tags_unique_constraint UNIQUE (parent_id, child_id)
);