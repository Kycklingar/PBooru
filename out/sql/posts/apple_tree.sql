#posts

CREATE TABLE IF NOT EXISTS apple_tree (
	apple INTEGER NOT NULL REFERENCES posts(id),
	pear INTEGER NOT NULL REFERENCES posts(id),
	processed TIMESTAMPTZ DEFAULT NULL,

	CONSTRAINT apple_pear UNIQUE (apple, pear),
	CHECK (apple < pear)
);
