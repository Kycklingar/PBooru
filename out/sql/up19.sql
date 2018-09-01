ALTER TABLE comics ADD COLUMN modified DATETIME NOT NULL DEFAULT "2018-01-01 00:00:00";

CREATE INDEX comic_mod_index ON comics(modified);

CREATE TRIGGER comic_modified_tr
AFTER INSERT ON comic_mappings
BEGIN
    UPDATE comics
    SET modified = CURRENT_TIMESTAMP
    WHERE id = NEW.comic_id;
END;