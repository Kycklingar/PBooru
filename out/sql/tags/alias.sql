#tags

CREATE TABLE IF NOT EXISTS alias(
    alias_from INT PRIMARY KEY NOT NULL REFERENCES tags(id),
    alias_to INT NOT NULL REFERENCES tags(id)
);

