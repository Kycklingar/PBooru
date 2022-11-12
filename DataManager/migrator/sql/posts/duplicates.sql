#posts

CREATE TABLE IF NOT EXISTS duplicates (
	post_id INTEGER NOT NULL REFERENCES posts(id),
	dup_id INTEGER PRIMARY KEY REFERENCES posts(id),
	timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE FUNCTION get_dupe(pid INTEGER)
RETURNS integer AS $$
DECLARE rid INTEGER;
BEGIN
	SELECT COALESCE(post_id, pid) INTO rid
	FROM posts
	LEFT JOIN duplicates
	ON id = dup_id
	WHERE id = pid;
	RETURN rid;
END;
$$ LANGUAGE plpgsql;
