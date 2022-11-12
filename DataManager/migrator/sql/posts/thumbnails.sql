#posts

CREATE TABLE thumbnails(
	post_id INTEGER NOT NULL REFERENCES posts(id),
	dimension INTEGER NOT NULL,
	multihash CHAR(59) NOT NULL,
	PRIMARY KEY (post_id, dimension)
);
