CREATE TABLE IF NOT EXISTS post_views(
	id BIGSERIAL PRIMARY KEY,
	post_id INTEGER REFERENCES posts(id) NOT NULL,
	views INTEGER NOT NULL,
	timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX post_views_post_id_index ON post_views(post_id);

CREATE OR REPLACE FUNCTION score_update()
RETURNS trigger AS $$
BEGIN
	UPDATE posts
	SET score = (
		SELECT count(*)
		FROM post_score_mapping
		WHERE post_id = NEW.post_id
		OR post_id = OLD.post_id
	) + (
		SELECT COALESCE(SUM(views), 0) / 1000
		FROM post_views
		WHERE post_id = NEW.post_id
		OR post_id = OLD.post_id
	)
	WHERE id = NEW.post_id
	OR id = OLD.post_id;

	RETURN NEW;
END;
$$
LANGUAGE plpgsql;
