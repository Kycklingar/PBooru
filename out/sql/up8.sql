CREATE TABLE IF NOT EXISTS post_score_mapping(
	post_id INTEGER NOT NULL REFERENCES posts(id),
	user_id INTEGER NOT NULL REFERENCES users(id),
	CONSTRAINT post_user UNIQUE (post_id, user_id)
);

ALTER TABLE posts ADD COLUMN IF NOT EXISTS score INTEGER NOT NULL DEFAULT 0;

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

CREATE OR REPLACE FUNCTION score_update()
RETURNS trigger AS $$
BEGIN
	UPDATE posts
	SET score = (
		SELECT count(*)
		FROM post_score_mapping
		WHERE post_id = NEW.post_id
		OR post_id = OLD.post_id
		)
	WHERE id = NEW.post_id
	OR id = OLD.post_id;

	RETURN NEW;
END;
$$
LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS post_score_insert_trigger ON post_score_mapping;
DROP TRIGGER IF EXISTS post_score_delete_trigger ON post_score_mapping;

CREATE TRIGGER post_score_insert_trigger
AFTER INSERT ON post_score_mapping
FOR EACH ROW
EXECUTE PROCEDURE score_update();

CREATE TRIGGER post_score_delete_trigger
AFTER DELETE ON post_score_mapping
FOR EACH ROW
EXECUTE PROCEDURE score_update();
