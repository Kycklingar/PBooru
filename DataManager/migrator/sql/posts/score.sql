#users
#posts

CREATE TABLE IF NOT EXISTS post_score_mapping(
	id SERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES posts(id),
	user_id INTEGER NOT NULL REFERENCES users(id),
	CONSTRAINT post_user UNIQUE (post_id, user_id)
);

CREATE INDEX IF NOT EXISTS post_score_post_id_index ON post_score_mapping(post_id);

CREATE OR REPLACE FUNCTION post_vote_update(postid INTEGER, userid INTEGER)
RETURNS INTEGER AS $$
DECLARE
row_exists INTEGER;
BEGIN
	SELECT 1
	INTO row_exists
	FROM post_score_mapping
	WHERE post_id = postid
	AND user_id = userid;

	IF (row_exists > 0) THEN
		DELETE FROM post_score_mapping
		WHERE post_id = postid
		AND user_id = userid;

		RETURN 0;
	ELSE
		INSERT INTO post_score_mapping (post_id, user_id)
		VALUES (postid, userid);

		RETURN 1;
END IF;
END;
$$
LANGUAGE plpgsql;
