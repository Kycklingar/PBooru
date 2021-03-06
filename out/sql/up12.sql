CREATE TABLE IF NOT EXISTS duplicate_report(
	id SERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES posts(id),
	reporter INTEGER NOT NULL REFERENCES users(id),
	note TEXT,
	approved TIMESTAMP DEFAULT NULL,
	timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS duplicate_report_posts(
	report_id INTEGER NOT NULL REFERENCES duplicate_report(id),
	post_id INTEGER NOT NULL REFERENCES posts(id)
);

CREATE TABLE IF NOT EXISTS duplicates (
	post_id INTEGER NOT NULL REFERENCES posts(id),
	dup_id INTEGER PRIMARY KEY REFERENCES posts(id),
	timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS apple_tree (
	apple INTEGER NOT NULL REFERENCES posts(id),
	pear INTEGER NOT NULL REFERENCES posts(id),
	processed TIMESTAMP DEFAULT NULL,

	CONSTRAINT apple_pear UNIQUE (apple, pear),
	CHECK (apple < pear)
);

INSERT INTO duplicates (post_id, dup_id)
	SELECT d1.post_id, d2.post_id
	FROM duplicate_posts d1
	LEFT JOIN duplicate_posts d2
	ON d1.dup_id = d2.dup_id
	WHERE d1.level = (
		SELECT MIN(level)
		FROM duplicate_posts
		WHERE dup_id = d1.dup_id
	)
	AND d1.post_id != d2.post_id;

DROP TABLE duplicate_posts;

CREATE TRIGGER post_score_update_trigger
AFTER UPDATE ON post_score_mapping
FOR EACH ROW EXECUTE PROCEDURE score_update();

CREATE INDEX phash_h1_index ON phash (h1);
CREATE INDEX phash_h2_index ON phash (h2);
CREATE INDEX phash_h3_index ON phash (h3);
CREATE INDEX phash_h4_index ON phash (h4);
