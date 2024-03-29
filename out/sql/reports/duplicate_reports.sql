#users
#posts

CREATE TABLE IF NOT EXISTS duplicate_report(
	id SERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES posts(id),
	reporter INTEGER NOT NULL REFERENCES users(id),
	note TEXT,
	report_type INTEGER NOT NULL DEFAULT 0,
	approved TIMESTAMPTZ DEFAULT NULL,
	timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS duplicate_report_posts(
	id SERIAL PRIMARY KEY,
	report_id INTEGER NOT NULL REFERENCES duplicate_report(id),
	post_id INTEGER NOT NULL REFERENCES posts(id)
);
