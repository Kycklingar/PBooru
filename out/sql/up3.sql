CREATE TABLE alias(
    alias_from INTEGER NOT NULL,
    alias_to INTEGER NOT NULL,
    FOREIGN KEY (alias_from) REFERENCES tags(id),
    FOREIGN KEY (alias_to) REFERENCES tags(id),
    CONSTRAINT alias_unique_constraint UNIQUE (alias_from, alias_to)
);