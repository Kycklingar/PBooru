CREATE TABLE duplicate_report(
	id SERIAL PRIMARY KEY,
	reporter INTEGER NOT NULL REFERENCES users(id),
	note TEXT,
	approved INTEGER NOT NULL DEFAULT 0,
	timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE duplicate_report_posts(
	report_id INTEGER NOT NULL REFERENCES duplicate_report(id),
	post_id INTEGER NOT NULL REFERENCES posts(id),
	score INTEGER NOT NULL DEFAULT 0
);
