CREATE TABLE IF NOT EXISTS post_views(
	id BIGSERIAL PRIMARY KEY,
	post_id INTEGER REFERENCES posts(id) NOT NULL,
	views INTEGER NOT NULL,
	timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS post_views_post_id_index ON post_views(post_id);

DROP TRIGGER IF EXISTS post_score_delete_trigger ON post_score_mapping;
DROP TRIGGER IF EXISTS post_score_insert_trigger ON post_score_mapping;
DROP FUNCTION IF EXISTS score_update;

UPDATE posts
SET score = (
	SELECT count(*) * 1000
	FROM post_score_mapping
	WHERE post_id = posts.id
) + (
	SELECT COALESCE(SUM(views), 0)
	FROM post_views
	WHERE post_id = posts.id
);
