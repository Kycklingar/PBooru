CREATE TABLE IF NOT EXISTS tombstone (
	e621_id INTEGER PRIMARY KEY,
	md5 TEXT NOT NULL,
	reason TEXT NOT NULL,
	removed TIMESTAMPTZ NOT NULL,
	post_id INTEGER REFERENCES posts(id)
);

CREATE INDEX ON tombstone (md5);
CREATE INDEX ON tombstone (post_id);

CREATE PROCEDURE IF NOT EXISTS update_tombstone()
LANGUAGE SQL
AS $$
	UPDATE tombstone
	SET post_id = tmp.id
	FROM posts AS tmp
	JOIN hashes h
	ON h.post_id = tmp.id
	WHERE h.md5 = tombstone.md5
	AND tombstone.post_id IS NULL;
$$;
