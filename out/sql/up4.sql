CREATE TABLE thumbnails(
	post_id INTEGER NOT NULL REFERENCES post(id),
	dimension INTEGER NOT NULL,
	multihash CHAR(49) NOT NULL,
	PRIMARY KEY (post_id, dimension)
);

INSERT INTO thumbnails(
	post_id, 
	dimension, 
	multihash
	)
SELECT id, 1024, thumbhash
FROM posts
WHERE thumbhash != 'NT';

ALTER TABLE posts
DROP COLUMN thumbhash;
