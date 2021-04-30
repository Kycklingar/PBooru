ALTER TABLE duplicate_report ADD COLUMN report_type INTEGER NOT NULL DEFAULT 0;

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

