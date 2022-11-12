#users

CREATE TABLE IF NOT EXISTS message (
	id SERIAL PRIMARY KEY,
	sender INT NOT NULL REFERENCES users(id),
	recipient INT NOT NULL REFERENCES users(id),
	title TEXT NOT NULL,
	text TEXT NOT NULL,
	date TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
	message_id INT PRIMARY KEY REFERENCES message(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS messages_sent (
	message_id INT PRIMARY KEY REFERENCES message(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS messages_read (
	message_id INT PRIMARY KEY REFERENCES message(id) ON DELETE CASCADE
);


CREATE OR REPLACE FUNCTION new_message()
RETURNS trigger AS $$
BEGIN
	INSERT INTO messages(message_id)
	VALUES(NEW.id);

	INSERT INTO messages_sent(message_id)
	VALUES(NEW.id);

	RETURN NEW;
END;
$$
LANGUAGE 'plpgsql';

DROP TRIGGER IF EXISTS new_message_trigger ON message;

CREATE TRIGGER new_message_trigger
AFTER INSERT ON message
FOR EACH ROW
	EXECUTE PROCEDURE new_message();
