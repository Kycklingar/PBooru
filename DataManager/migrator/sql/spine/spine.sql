#users

CREATE TYPE log_action AS ENUM ('create', 'modify', 'delete');

CREATE TABLE logs (
	log_id BIGSERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id),
	timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE logs_tables (
	id SERIAL PRIMARY KEY,
	table_name TEXT NOT NULL UNIQUE
);

CREATE TABLE logs_tables_altered (
	log_id BIGINT REFERENCES logs(log_id) ON DELETE CASCADE,
	table_id INTEGER NOT NULL REFERENCES logs_tables(id) ON DELETE CASCADE,

	UNIQUE(log_id, table_id)
);

